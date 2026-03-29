package mcp

import (
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newMCPTestGormConfig() *gorm.Config {
	return &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	}
}
