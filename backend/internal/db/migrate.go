package db

import (
	"fmt"
	"time"

	"github.com/jiangfire/cornerstone/backend/internal/config"
	"github.com/jiangfire/cornerstone/backend/internal/models"
	pkgdb "github.com/jiangfire/cornerstone/backend/pkg/db"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// InitDB 初始化数据库连接（包装pkg/db中的函数）
func InitDB(cfg config.DatabaseConfig) error {
	return pkgdb.InitDB(cfg)
}

// Migrate 执行所有数据库迁移
func Migrate() error {
	database := pkgdb.DB()
	logger := zap.L()

	logger.Info("开始数据库迁移...")

	// 注册插件（如果需要）
	if err := database.AutoMigrate(
		// 核心用户和组织模型
		&models.User{},
		&models.Organization{},
		&models.OrganizationMember{},

		// 数据库和权限模型
		&models.Database{},
		&models.DatabaseAccess{},

		// 数据结构模型
		&models.Table{},
		&models.Field{},
		&models.Record{},

		// 字段级权限模型
		&models.FieldPermission{},

		// 文件和插件模型
		&models.File{},
		&models.Plugin{},
		&models.PluginBinding{},

		// 安全相关
		&models.TokenBlacklist{},
	); err != nil {
		return fmt.Errorf("自动迁移失败: %w", err)
	}

	logger.Info("基础表结构迁移完成")

	// 创建复合索引
	if err := createIndexes(database); err != nil {
		return fmt.Errorf("创建索引失败: %w", err)
	}

	logger.Info("复合索引创建完成")

	// 创建物化视图（权限缓存）
	if err := createMaterializedViews(database); err != nil {
		return fmt.Errorf("创建物化视图失败: %w", err)
	}

	logger.Info("物化视图创建完成")

	// 创建token_blacklist表的特殊索引
	if err := createTokenBlacklistIndexes(database); err != nil {
		return fmt.Errorf("创建token blacklist索引失败: %w", err)
	}

	logger.Info("Token blacklist索引创建完成")

	logger.Info("数据库迁移完成 ✅")
	return nil
}

// createIndexes 创建复合索引以提升查询性能
func createIndexes(db *gorm.DB) error {
	// records表的JSONB GIN索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_records_data ON records USING GIN(data)").Error; err != nil {
		return err
	}

	// database_access复合索引（权限查询优化）
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_database_access_user_db ON database_access(user_id, database_id)").Error; err != nil {
		return err
	}

	// organization_members复合索引（组织成员查询优化）
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_org_members_org_user ON organization_members(organization_id, user_id)").Error; err != nil {
		return err
	}

	// records表的外键索引（数据查询优化）
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_records_table_id ON records(table_id)").Error; err != nil {
		return err
	}

	// tables表的外键索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_tables_database_id ON tables(database_id)").Error; err != nil {
		return err
	}

	// fields表的外键索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_fields_table_id ON fields(table_id)").Error; err != nil {
		return err
	}

	// files表的外键索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_files_record_id ON files(record_id)").Error; err != nil {
		return err
	}

	// plugin_bindings复合索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_plugin_bindings_plugin_table ON plugin_bindings(plugin_id, table_id)").Error; err != nil {
		return err
	}

	// field_permissions复合索引（权限查询优化）
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_field_permissions_table_field_role ON field_permissions(table_id, field_id, role)").Error; err != nil {
		return err
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_field_permissions_role ON field_permissions(role)").Error; err != nil {
		return err
	}

	return nil
}

// createMaterializedViews 创建权限缓存物化视图
func createMaterializedViews(db *gorm.DB) error {
	// 删除旧视图（如果存在）
	db.Exec("DROP MATERIALIZED VIEW IF EXISTS user_database_permissions")

	// 创建物化视图
	viewSQL := `
CREATE MATERIALIZED VIEW user_database_permissions AS
SELECT
    da.id as access_id,
    da.user_id,
    da.database_id,
    da.role,
    d.name as db_name,
    d.owner_id,
    d.created_at,
    d.updated_at
FROM database_access da
JOIN databases d ON da.database_id = d.id
WHERE d.deleted_at IS NULL
`

	if err := db.Exec(viewSQL).Error; err != nil {
		return err
	}

	// 在物化视图上创建索引
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_user_db_permissions_unique ON user_database_permissions(user_id, database_id)").Error; err != nil {
		return err
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_user_db_permissions_user ON user_database_permissions(user_id)").Error; err != nil {
		return err
	}

	return nil
}

// createTokenBlacklistIndexes 创建token黑名单的特殊索引
func createTokenBlacklistIndexes(db *gorm.DB) error {
	// 创建普通索引用于查询性能
	// 条件索引在PostgreSQL中需要IMMUTABLE函数，这里我们使用应用层过滤
	indexSQL := `
CREATE INDEX IF NOT EXISTS idx_blacklist_expired
ON token_blacklist(expired_at)
`

	if err := db.Exec(indexSQL).Error; err != nil {
		return err
	}

	return nil
}

// RefreshMaterializedViews 刷新物化视图（用于定时任务）
func RefreshMaterializedViews() error {
	database := pkgdb.DB()
	logger := zap.L()

	logger.Info("开始刷新物化视图...")

	if err := database.Exec("REFRESH MATERIALIZED VIEW CONCURRENTLY user_database_permissions").Error; err != nil {
		return fmt.Errorf("刷新物化视图失败: %w", err)
	}

	logger.Info("物化视图刷新完成")
	return nil
}

// CleanupExpiredTokens 清理过期的token黑名单记录
func CleanupExpiredTokens() error {
	database := pkgdb.DB()
	logger := zap.L()

	result := database.Where("expired_at <= ?", time.Now()).Delete(&models.TokenBlacklist{})
	if result.Error != nil {
		return fmt.Errorf("清理过期token失败: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		logger.Info("清理过期token记录", zap.Int64("count", result.RowsAffected))
	}

	return nil
}

// SetupPeriodicTasks 设置定时任务（建议在main.go中调用）
func SetupPeriodicTasks() {
	// 每5分钟刷新一次物化视图
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			if err := RefreshMaterializedViews(); err != nil {
				zap.L().Error("定时刷新物化视图失败", zap.Error(err))
			}
		}
	}()

	// 每小时清理一次过期token
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			if err := CleanupExpiredTokens(); err != nil {
				zap.L().Error("定时清理过期token失败", zap.Error(err))
			}
		}
	}()
}