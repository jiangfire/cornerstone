package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/config"
	"github.com/jiangfire/cornerstone/backend/internal/db"
	"github.com/jiangfire/cornerstone/backend/internal/frontend"
	"github.com/jiangfire/cornerstone/backend/internal/handlers"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/pkg/asyncworker"
	applog "github.com/jiangfire/cornerstone/backend/pkg/log"
	"github.com/jiangfire/cornerstone/backend/pkg/utils"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Version is set at build time via -ldflags="-X main.Version=..."
var Version = "dev"

func main() {
	// 1. 加载配置（从环境变量）
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. 初始化日志
	if err := applog.InitLogger(cfg.Logger); err != nil {
		log.Fatalf("Failed to init logger: %v", err)
	}

	logger := applog.GetLogger()
	logger.Info("Starting Cornerstone server...")

	// 2.5 显式注入 JWT 配置，确保签发/验证使用同一密钥（避免 dev 模式下 lazy load 二次随机）。
	if err := utils.InitJWT(cfg.JWT.Secret, cfg.JWT.Expiration); err != nil {
		applog.Fatalf("Failed to init JWT: %v", err)
	}

	// 3. 初始化数据库
	if err := retryOperation(func() error {
		return db.InitDB(cfg.Database)
	}, 3, time.Second); err != nil {
		applog.Fatalf("Failed to init database: %v", err)
	}

	// 4. 执行数据库迁移
	if err := retryOperation(db.Migrate, 3, time.Second); err != nil {
		applog.Fatalf("Failed to migrate database: %v", err)
	}

	// 5. 设置定时任务（物化视图刷新和token清理）
	taskCtx, cancelTasks := context.WithCancel(context.Background())
	periodicTaskWG := db.SetupPeriodicTasks(taskCtx, cfg.Integrations)

	// 5.5 注入插件异步任务池（提供 panic 兜底 + 关停时的等待边界）
	pluginPool := asyncworker.New(context.Background())
	services.SetDefaultPluginPool(pluginPool)

	// 6. 创建Gin引擎
	gin.SetMode(cfg.Server.Mode)
	r := gin.New()
	handlers.SetVersion(Version)
	handlers.ConfigureMCP(handlers.MCPOptions{
		SSEKeepaliveInterval: time.Duration(cfg.MCP.SSEKeepaliveSec) * time.Second,
		SSERetryInterval:     time.Duration(cfg.MCP.SSERetryMS) * time.Millisecond,
		SSEReplayBuffer:      cfg.MCP.SSEReplayBuffer,
	})

	// 7. 注册中间件
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())
	r.Use(middleware.RequestID())
	r.Use(middleware.RequestLogger())

	// 8. 健康检查 + 指标路由
	// /health  : liveness, 不查依赖, 进程在跑就 200
	// /ready   : readiness, 失败时返回 503, 用作 compose / k8s 探针
	// /metrics : Prometheus 抓取端点, 暴露 Go runtime + process collector 默认指标
	r.GET("/health", handlers.Health)
	r.GET("/ready", handlers.Ready)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// 头像公开访问（无需认证，用于 img src 直接引用）
	r.GET("/avatars/:filename", handlers.ServeAvatar)

	// HTTP MCP 路由
	r.OPTIONS("/mcp", handlers.HandleMCPOptions)
	mcpRoute := r.Group("/mcp")
	mcpRoute.Use(middleware.MCPOriginGuard(), middleware.Auth())
	mcpRoute.POST("", handlers.HandleMCP)
	mcpRoute.GET("", handlers.HandleMCPGet)

	// 9. API路由组（全局限流）
	api := r.Group("/api")
	api.Use(middleware.RateLimit())
	{
		// 认证路由（无需认证，附加更严格限流）
		auth := api.Group("/auth")
		auth.Use(middleware.AuthRateLimit())
		auth.POST("/register", handlers.Register)
		auth.POST("/login", handlers.Login)

		// 系统集成事件（使用集成 token，已有 IntegrationTokenAuth，不叠加通用限流）
		integrations := api.Group("/integrations")
		integrations.Use(middleware.IntegrationTokenAuth(middleware.IntegrationAuthConfig{
			InboundTokens: cfg.Integrations.InboundTokens,
			SharedToken:   cfg.Integrations.SharedToken,
		}))
		integrations.POST("/events", handlers.ReceiveIntegrationEvent)

		// 注入 LLM Governor 客户端（如果配置了）
		if cfg.Integrations.LLMGovernorURL != "" && cfg.Integrations.LLMGovernorToken != "" {
			client := services.NewLLMGovernorClient(cfg.Integrations.LLMGovernorURL, cfg.Integrations.LLMGovernorToken)
			services.SetDefaultLLMGovernorClient(client)
			logger.Info("LLM Governor client configured")
		}

		// 需要认证的路由
		protected := api.Group("")
		protected.Use(middleware.Auth())
		{
			// 用户相关
			protected.GET("/users/me", handlers.GetUserInfo)
			protected.PUT("/users/me", handlers.UpdateUserInfo)
			protected.POST("/users/me/avatar", handlers.UploadAvatar)
			protected.PUT("/users/me/password", handlers.ChangeUserPassword)
			protected.DELETE("/users/me", handlers.DeleteUserAccount)
			protected.GET("/users", handlers.ListUsers)
			protected.GET("/users/search", handlers.SearchUsers)
			protected.POST("/auth/logout", handlers.Logout)

			// 组织相关
			protected.POST("/organizations", handlers.CreateOrganization)
			protected.GET("/organizations", handlers.ListOrganizations)
			protected.GET("/organizations/:id", handlers.GetOrganization)
			protected.PUT("/organizations/:id", handlers.UpdateOrganization)
			protected.DELETE("/organizations/:id", handlers.DeleteOrganization)

			// 组织成员相关
			protected.GET("/organizations/:id/members", handlers.ListOrganizationMembers)
			protected.POST("/organizations/:id/members", handlers.AddOrganizationMember)
			protected.DELETE("/organizations/:id/members/:member_id", handlers.RemoveOrganizationMember)
			protected.PUT("/organizations/:id/members/:member_id/role", handlers.UpdateOrganizationMemberRole)

			// 数据库相关
			protected.POST("/databases", handlers.CreateDatabase)
			protected.GET("/databases", handlers.ListDatabases)
			protected.GET("/databases/:id", handlers.GetDatabase)
			protected.PUT("/databases/:id", handlers.UpdateDatabase)
			protected.DELETE("/databases/:id", handlers.DeleteDatabase)

			// 数据库权限相关
			protected.POST("/databases/:id/share", handlers.ShareDatabase)
			protected.GET("/databases/:id/users", handlers.ListDatabaseUsers)
			protected.DELETE("/databases/:id/users/:user_id", handlers.RemoveDatabaseUser)
			protected.PUT("/databases/:id/users/:user_id/role", handlers.UpdateDatabaseUserRole)

			// 表相关
			protected.POST("/tables", handlers.CreateTable)
			protected.GET("/databases/:id/tables", handlers.ListTables)
			protected.GET("/tables/:id", handlers.GetTable)
			protected.PUT("/tables/:id", handlers.UpdateTable)
			protected.DELETE("/tables/:id", handlers.DeleteTable)

			// 字段相关
			protected.POST("/fields", handlers.CreateField)
			protected.GET("/tables/:id/fields", handlers.ListFields)
			protected.GET("/fields/:id", handlers.GetField)
			protected.PUT("/fields/:id", handlers.UpdateField)
			protected.DELETE("/fields/:id", handlers.DeleteField)

			// 字段权限相关 - 必须在字段相关之后，避免路由冲突
			protected.GET("/tables/:id/field-permissions", handlers.GetFieldPermissions)
			protected.PUT("/tables/:id/field-permissions", handlers.SetFieldPermission)
			protected.PUT("/tables/:id/field-permissions/batch", handlers.BatchSetFieldPermissions)

			// 记录相关
			protected.POST("/records", handlers.CreateRecord)
			protected.GET("/records", handlers.ListRecords)
			protected.GET("/records/export", handlers.ExportRecords)
			protected.GET("/records/:id", handlers.GetRecord)
			protected.PUT("/records/:id", handlers.UpdateRecord)
			protected.DELETE("/records/:id", handlers.DeleteRecord)
			protected.POST("/records/batch", handlers.BatchCreateRecords)

			// 文件相关
			protected.POST("/files/upload", handlers.UploadFile)
			protected.GET("/files/:id", handlers.GetFile)
			protected.GET("/files/:id/download", handlers.DownloadFile)
			protected.DELETE("/files/:id", handlers.DeleteFile)
			protected.GET("/records/:id/files", handlers.ListRecordFiles)

			// 插件相关
			protected.POST("/plugins", handlers.CreatePlugin)
			protected.GET("/plugins", handlers.ListPlugins)
			protected.GET("/plugins/:id", handlers.GetPlugin)
			protected.PUT("/plugins/:id", handlers.UpdatePlugin)
			protected.DELETE("/plugins/:id", handlers.DeletePlugin)
			protected.POST("/plugins/:id/bind", handlers.BindPlugin)
			protected.DELETE("/plugins/:id/unbind", handlers.UnbindPlugin)
			protected.GET("/plugins/:id/bindings", handlers.ListPluginBindings)
			protected.POST("/plugins/:id/execute", handlers.ExecutePlugin)
			protected.GET("/plugins/:id/executions", handlers.ListPluginExecutions)

			// 统计相关
			protected.GET("/stats/summary", handlers.GetStatsSummary)
			protected.GET("/stats/activities", handlers.GetRecentActivities)

			// 查询 DSL 相关
			queryHandler := handlers.NewQueryHandler()
			protected.GET("/query", queryHandler.Query)
			protected.POST("/query", queryHandler.Query)
			protected.GET("/query/simple", queryHandler.SimplifiedQuery)
			protected.POST("/query/batch", queryHandler.BatchQuery)
			protected.POST("/query/explain", queryHandler.QueryExplain)
			protected.POST("/query/validate", queryHandler.QueryValidate)
			protected.GET("/query/tables", queryHandler.ListTables)
			protected.GET("/query/schema/:table", queryHandler.GetTableSchema)

			// 系统设置
			protected.GET("/settings", handlers.GetSettings)
			protected.PUT("/settings", handlers.UpdateSettings)

			// 治理任务与审核
			protected.POST("/governance/tasks", handlers.CreateGovernanceTask)
			protected.GET("/governance/tasks", handlers.ListGovernanceTasks)
			protected.GET("/governance/tasks/:id", handlers.GetGovernanceTask)
			protected.PUT("/governance/tasks/:id", handlers.UpdateGovernanceTask)
			protected.DELETE("/governance/tasks/:id", handlers.DeleteGovernanceTask)
			protected.POST("/governance/tasks/:id/evidences", handlers.CreateGovernanceEvidence)
			protected.POST("/governance/tasks/:id/comments", handlers.CreateGovernanceComment)
			protected.POST("/governance/reviews", handlers.CreateGovernanceReview)
			protected.GET("/governance/reviews/:id", handlers.GetGovernanceReview)
			protected.POST("/governance/reviews/:id/approve", handlers.ApproveGovernanceReview)
			protected.POST("/governance/reviews/:id/reject", handlers.RejectGovernanceReview)
			protected.POST("/governance/reviews/:id/apply", handlers.ApplyGovernanceReview)
			protected.POST("/governance/ai/recommendations", handlers.GenerateAIRecommendation)
		}
	}

	// API 版本兼容路由：将 /api/v1/* 复用到 /api/*
	r.Any("/api/v1/*path", func(c *gin.Context) {
		c.Request.URL.Path = "/api" + c.Param("path")
		r.HandleContext(c)
	})

	// 10. 注册前端静态文件服务
	frontend.RegisterRoutes(r)

	// 11. 启动服务器
	srv := &http.Server{
		Addr:              cfg.GetServerAddr(),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// 优雅关闭
	go func() {
		applog.Infof("Server starting on %s", cfg.GetServerAddr())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			applog.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	applog.Info("Shutting down server...")

	// 优雅关闭（5秒超时）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 先停止后台任务，避免关库后继续访问DB
	cancelTasks()
	waitPeriodicTasks(periodicTaskWG, 3*time.Second)

	if err := srv.Shutdown(ctx); err != nil {
		applog.Fatalf("Server forced to shutdown: %v", err)
	}

	// 关停插件池：取消 ctx 让 in-flight 插件感知，并最多再等 5s 给当前脚本收尾。
	// 必须发生在 srv.Shutdown 之后（已无新 HTTP 触发）、CloseDB 之前（任务可能还在写执行记录）。
	services.SetDefaultPluginPool(nil)
	if err := pluginPool.Stop(5 * time.Second); err != nil {
		applog.Errorf("Plugin worker pool shutdown timed out: %v", err)
	}

	if err := db.CloseDB(); err != nil {
		applog.Errorf("Failed to close database: %v", err)
	}

	applog.Sync()
	applog.Info("Server exited")
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
