package cli

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/internal/config"
	"github.com/jiangfire/cornerstone/internal/db"
	"github.com/jiangfire/cornerstone/internal/handlers"
	"github.com/jiangfire/cornerstone/internal/middleware"
	"github.com/jiangfire/cornerstone/internal/services"
	// Blank import to register swagger docs for gin-swagger UI.
	_ "github.com/jiangfire/cornerstone/internal/swagger"
	applog "github.com/jiangfire/cornerstone/pkg/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title           Cornerstone API
// @version         1.0
// @description     Cornerstone is a headless data platform providing REST API, Query DSL, AI assistant and MCP endpoints.
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "start HTTP API + MCP server",
	Long: `Start Cornerstone HTTP API and MCP protocol server.
Provides REST API, Query DSL, AI assistant and MCP SSE endpoints for external clients and AI agents.`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if err := applog.InitLogger(cfg.Logger); err != nil {
		return err
	}

	logger := applog.GetLogger()
	logger.Info("Starting Cornerstone server...")

	if err := retryOperation(func() error {
		return db.InitDB(cfg.Database)
	}, 3, time.Second); err != nil {
		applog.Fatalf("Failed to init database: %v", err)
	}

	if err := retryOperation(db.Migrate, 3, time.Second); err != nil {
		applog.Fatalf("Failed to migrate database: %v", err)
	}

	taskCtx, cancelTasks := context.WithCancel(context.Background())
	periodicTaskWG := db.SetupPeriodicTasks(taskCtx)

	gin.SetMode(cfg.Server.Mode)
	r := gin.New()
	handlers.SetVersion(Version)
	handlers.ConfigureMCP(handlers.MCPOptions{
		SSEKeepaliveInterval: time.Duration(cfg.MCP.SSEKeepaliveSec) * time.Second,
		SSERetryInterval:     time.Duration(cfg.MCP.SSERetryMS) * time.Millisecond,
		SSEReplayBuffer:      cfg.MCP.SSEReplayBuffer,
	})

	if cfg.LLM.APIKey != "" {
		agent := services.NewAIAgent(cfg.LLM.APIKey, cfg.LLM.Model, cfg.LLM.BaseURL)
		handlers.InitAIAgent(agent)
		logger.Info("AI Agent initialized", zap.String("model", cfg.LLM.Model))
	} else {
		logger.Warn("LLM_API_KEY not set, AI assistant will be unavailable")
	}

	r.Use(gin.Recovery())
	r.Use(middleware.CORS())
	r.Use(middleware.RequestID())
	r.Use(middleware.RequestLogger())

	r.GET("/health", handlers.Health)
	r.GET("/ready", handlers.Ready)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.OPTIONS("/mcp", handlers.HandleMCPOptions)
	mcpRoute := r.Group("/mcp")
	mcpRoute.Use(middleware.MCPOriginGuard(), middleware.Auth())
	mcpRoute.POST("", handlers.HandleMCP)
	mcpRoute.GET("", handlers.HandleMCPGet)

	api := r.Group("/api/v1")
	{
		tokenRoute := api.Group("/tokens")
		tokenRoute.Use(middleware.Auth())
		tokenRoute.GET("", handlers.ListTokens)
		tokenRoute.POST("", middleware.RequireMaster(), handlers.CreateToken)
		tokenRoute.PUT("/:id", middleware.RequireMaster(), handlers.UpdateToken)
		tokenRoute.DELETE("/:id", handlers.DeleteToken)

		protected := api.Group("")
		protected.Use(middleware.Auth())
		{
			protected.POST("/databases", handlers.CreateDatabase)
			protected.POST("/databases/with-tables", handlers.CreateDatabaseWithTables)
			protected.GET("/databases", handlers.ListDatabases)
			protected.GET("/databases/:id", handlers.GetDatabase)
			protected.PUT("/databases/:id", handlers.UpdateDatabase)
			protected.DELETE("/databases/:id", handlers.DeleteDatabase)

			protected.POST("/tables", handlers.CreateTable)
			protected.GET("/databases/:id/tables", handlers.ListTables)
			protected.GET("/tables/:id", handlers.GetTable)
			protected.PUT("/tables/:id", handlers.UpdateTable)
			protected.DELETE("/tables/:id", handlers.DeleteTable)

			protected.POST("/fields", handlers.CreateField)
			protected.GET("/tables/:id/fields", handlers.ListFields)
			protected.GET("/fields/:id", handlers.GetField)
			protected.PUT("/fields/:id", handlers.UpdateField)
			protected.DELETE("/fields/:id", handlers.DeleteField)

			protected.POST("/records", handlers.CreateRecord)
			protected.GET("/records", handlers.ListRecords)
			protected.GET("/records/export", handlers.ExportRecords)
			protected.GET("/records/:id", handlers.GetRecord)
			protected.PUT("/records/:id", handlers.UpdateRecord)
			protected.DELETE("/records/:id", handlers.DeleteRecord)
			protected.POST("/records/batch", handlers.BatchCreateRecords)

			protected.POST("/files/upload", handlers.UploadFile)
			protected.GET("/files/:id", handlers.GetFile)
			protected.GET("/files/:id/download", handlers.DownloadFile)
			protected.DELETE("/files/:id", handlers.DeleteFile)
			protected.GET("/records/:id/files", handlers.ListRecordFiles)

			queryHandler := handlers.NewQueryHandler()
			protected.GET("/query", queryHandler.Query)
			protected.POST("/query", queryHandler.Query)
			protected.GET("/query/simple", queryHandler.SimplifiedQuery)
			protected.POST("/query/batch", queryHandler.BatchQuery)
			protected.POST("/query/explain", queryHandler.QueryExplain)
			protected.POST("/query/validate", queryHandler.QueryValidate)
			protected.GET("/query/tables", queryHandler.ListTables)
			protected.GET("/query/schema/:table", queryHandler.GetTableSchema)

			protected.POST("/ai/chat", handlers.ChatWithAI)
		}
	}

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		// Redirect /api/* to /api/v1/* for backward compatibility
		if len(path) >= 5 && path[:5] == "/api/" && !strings.HasPrefix(path, "/api/v1/") {
			c.Request.URL.Path = "/api/v1" + strings.TrimPrefix(path, "/api")
			r.HandleContext(c)
			return
		}
		if len(path) >= 5 && path[:5] == "/api/" {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not found",
				"message": "The requested API endpoint does not exist",
			})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not found",
			"message": "Cornerstone API Server. Use /api endpoints or /mcp for MCP protocol.",
		})
	})

	srv := &http.Server{
		Addr:              cfg.GetServerAddr(),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		applog.Infof("Server starting on %s", cfg.GetServerAddr())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			applog.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	applog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cancelTasks()
	waitPeriodicTasks(periodicTaskWG, 3*time.Second)

	if err := srv.Shutdown(ctx); err != nil {
		applog.Fatalf("Server forced to shutdown: %v", err)
	}

	if err := db.CloseDB(); err != nil {
		applog.Errorf("Failed to close database: %v", err)
	}

	applog.Sync()
	applog.Info("Server exited")
	return nil
}

func waitPeriodicTasks(wg *sync.WaitGroup, timeout time.Duration) {
	if wg == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		applog.Warn("Periodic tasks shutdown timeout")
	}
}

func retryOperation(op func() error, maxAttempts int, baseDelay time.Duration) error {
	var lastErr error
	for i := range maxAttempts {
		if err := op(); err != nil {
			lastErr = err
			if i < maxAttempts-1 {
				time.Sleep(baseDelay * time.Duration(i+1))
			}
			continue
		}
		return nil
	}
	return lastErr
}
