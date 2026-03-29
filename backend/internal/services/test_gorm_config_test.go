package services

import (
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newServiceTestGormConfig() *gorm.Config {
	return &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	}
}
