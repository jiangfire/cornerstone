package db

import (
	"time"

	"github.com/glebarez/sqlite"
	"github.com/jiangfire/cornerstone/backend/internal/config"
	"go.uber.org/zap"
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

// IsSQLite 检查当前是否为 SQLite 数据库
func IsSQLite() bool {
	if db == nil {
		return false
	}
	return db.Dialector.Name() == "sqlite"
}

// IsPostgres 检查当前是否为 PostgreSQL 数据库
func IsPostgres() bool {
	if db == nil {
		return false
	}
	return db.Dialector.Name() == "postgres"
}

// InitDB 初始化数据库连接
func InitDB(cfg config.DatabaseConfig) error {
	var err error

	switch cfg.Type {
	case "sqlite":
		db, err = initSQLite(cfg)
	case "postgres":
		fallthrough
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

	sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	if cfg.MaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.MaxLifetime) * time.Second)
	}

	return nil
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
