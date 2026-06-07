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

// DB returns the database connection object
func DB() *gorm.DB {
	if db == nil {
		panic("database not initialized")
	}
	return db
}

// SetDB sets the database connection object (for testing)
func SetDB(d *gorm.DB) {
	db = d
}

// IsInitialized checks whether the database is initialized
func IsInitialized() bool {
	return db != nil
}

// IsSQLite checks whether the current database is SQLite
func IsSQLite() bool {
	if db == nil {
		return false
	}
	return db.Name() == "sqlite"
}

// IsPostgres checks whether the current database is PostgreSQL
func IsPostgres() bool {
	if db == nil {
		return false
	}
	return db.Name() == "postgres"
}

// IsMySQL checks whether the current database is MySQL
func IsMySQL() bool {
	if db == nil {
		return false
	}
	return db.Name() == "mysql"
}

// InitDB initializes the database connection
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

	// configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	// SQLite :memory: each connection is an independent database, must be limited to a single connection
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

// initMySQL initializes MySQL connection
func initMySQL(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn, err := mysql.ParseDSN(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse MySQL DSN: %w", err)
	}
	// ensure time.Time fields are scanned correctly
	if !dsn.ParseTime {
		dsn.ParseTime = true
	}
	db, err := gorm.Open(gormmysql.Open(dsn.FormatDSN()), &gorm.Config{
		Logger: NewZapLogger(zap.L()),
	})
	if err != nil {
		return nil, err
	}

	// MySQL 8.0 enables NO_ZERO_DATE by default, causing zero-value time.Time inserts to fail.
	// SET SESSION ensures the current connection takes effect immediately (SET GLOBAL only affects new connections).
	if err := db.Exec("SET SESSION sql_mode='ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION'").Error; err != nil {
		return nil, fmt.Errorf("failed to set MySQL sql_mode: %w", err)
	}

	return db, nil
}

// initPostgres initializes PostgreSQL connection
func initPostgres(cfg config.DatabaseConfig) (*gorm.DB, error) {
	return gorm.Open(postgres.New(postgres.Config{
		DSN:                  cfg.URL,
		PreferSimpleProtocol: true, // disable prepared statement usage
	}), &gorm.Config{
		Logger: NewZapLogger(zap.L()),
	})
}

// initSQLite initializes SQLite connection
func initSQLite(cfg config.DatabaseConfig) (*gorm.DB, error) {
	return gorm.Open(sqlite.Open(cfg.URL), &gorm.Config{
		Logger: NewZapLogger(zap.L()),
	})
}

// CloseDB closes the underlying database connection pool
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
