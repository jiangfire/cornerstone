package db

import (
	"github.com/jiangfire/cornerstone/backend/internal/config"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

// DB 返回数据库连接对象
func DB() *gorm.DB {
	if db == nil {
		panic("postgres database not initialized")
	}
	return db
}

// InitDB 初始化数据库连接
func InitDB(cfg config.DatabaseConfig) error {
	var err error
	db, err = gorm.Open(postgres.New(postgres.Config{
		DSN:                  cfg.URL,
		PreferSimpleProtocol: true, // disable prepared statement usage
	}), &gorm.Config{
		Logger: NewZapLogger(zap.L()),
	})
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
	sqlDB.SetConnMaxLifetime(0) // 0表示无限制

	return nil
}
