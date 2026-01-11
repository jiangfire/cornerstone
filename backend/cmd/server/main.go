package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/config"
	"github.com/jiangfire/cornerstone/backend/internal/db"
	"github.com/jiangfire/cornerstone/backend/internal/handlers"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	applog "github.com/jiangfire/cornerstone/backend/pkg/log"
)

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

	// 3. 初始化数据库
	if err := db.InitDB(cfg.Database); err != nil {
		applog.Fatalf("Failed to init database: %v", err)
	}

	// 4. 执行数据库迁移
	if err := db.Migrate(); err != nil {
		applog.Fatalf("Failed to migrate database: %v", err)
	}

	// 5. 设置定时任务（物化视图刷新和token清理）
	db.SetupPeriodicTasks()

	// 6. 创建Gin引擎
	gin.SetMode(cfg.Server.Mode)
	r := gin.New()

	// 7. 注册中间件
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())
	r.Use(middleware.RequestLogger())

	// 8. 健康检查路由
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "cornerstone-backend",
			"time":    time.Now().Format(time.RFC3339),
		})
	})

	// 9. API路由组
	api := r.Group("/api")
	{
		// 认证路由（无需认证）
		auth := api.Group("/auth")
		{
			auth.POST("/register", handlers.Register)
			auth.POST("/login", handlers.Login)
		}

		// 需要认证的路由
		protected := api.Group("")
		protected.Use(middleware.Auth())
		{
			// 用户相关
			protected.GET("/users/me", handlers.GetUserInfo)
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
		}
	}

	// 10. 启动服务器
	srv := &http.Server{
		Addr:    cfg.GetServerAddr(),
		Handler: r,
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

	if err := srv.Shutdown(ctx); err != nil {
		applog.Fatalf("Server forced to shutdown: %v", err)
	}

	applog.Info("Server exited")
}