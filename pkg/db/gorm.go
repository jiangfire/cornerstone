package db

import (
	"fmt"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/go-sql-driver/mysql"
	"github.com/jiangfire/cornerstone/internal/config"
	"go.uber.org/zap"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

// DB 返回数据库连接对象
func DB() *gorm.DB {
	if db == nil {
		panic("database not initialized")
	}
	return db
}

// SetDB 设置数据库连接对象（用于测试）
func SetDB(d *gorm.DB) {
	db = d
}

// IsInitialized 检查数据库是否已初始化
func IsInitialized() bool {
	return db != nil
}

// IsSQLite 检查当前是否为 SQLite 数据库
func IsSQLite() bool {
	if db == nil {
		return false
	}
	return db.Name() == "sqlite"
}

// IsPostgres 检查当前是否为 PostgreSQL 数据库
func IsPostgres() bool {
	if db == nil {
		return false
	}
	return db.Name() == "postgres"
}

// IsMySQL 检查当前是否为 MySQL 数据库
func IsMySQL() bool {
	if db == nil {
		return false
	}
	return db.Name() == "mysql"
}

// InitDB 初始化数据库连接
func InitDB(cfg config.DatabaseConfig) error {
	var err error

	switch cfg.Type {
	case "sqlite":
		db, err = initSQLite(cfg)
	case "mysql":
		db, err = initMySQL(cfg)
	default:
		db, err = initPostgres(cfg)
	}

	if err != nil {
		return err
	}

	// 设置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	// SQLite :memory: 每个连接都是独立的数据库，必须限制为单连接
	if cfg.Type == "sqlite" && cfg.URL == ":memory:" {
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
	} else {
		sqlDB.SetMaxOpenConns(cfg.MaxOpen)
		sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	}
	if cfg.MaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.MaxLifetime) * time.Second)
	}

	return nil
}

// initMySQL 初始化 MySQL 连接
func initMySQL(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn, err := mysql.ParseDSN(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("解析 MySQL DSN 失败: %w", err)
	}
	// 确保 time.Time 字段能正确扫描
	if !dsn.ParseTime {
		dsn.ParseTime = true
	}
	db, err := gorm.Open(gormmysql.Open(dsn.FormatDSN()), &gorm.Config{
		Logger: NewZapLogger(zap.L()),
	})
	if err != nil {
		return nil, err
	}

	// MySQL 8.0 默认启用 NO_ZERO_DATE，会导致 time.Time 零值插入失败。
	// SET SESSION 确保当前连接立即生效（SET GLOBAL 仅影响新连接）。
	if err := db.Exec("SET SESSION sql_mode='ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION'").Error; err != nil {
		return nil, fmt.Errorf("设置 MySQL sql_mode 失败: %w", err)
	}

	return db, nil
}

// initPostgres 初始化 PostgreSQL 连接
func initPostgres(cfg config.DatabaseConfig) (*gorm.DB, error) {
	return gorm.Open(postgres.New(postgres.Config{
		DSN:                  cfg.URL,
		PreferSimpleProtocol: true, // disable prepared statement usage
	}), &gorm.Config{
		Logger: NewZapLogger(zap.L()),
	})
}

// initSQLite 初始化 SQLite 连接
func initSQLite(cfg config.DatabaseConfig) (*gorm.DB, error) {
	return gorm.Open(sqlite.Open(cfg.URL), &gorm.Config{
		Logger: NewZapLogger(zap.L()),
	})
}

// CloseDB 关闭底层数据库连接池
func CloseDB() error {
	if db == nil {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}
