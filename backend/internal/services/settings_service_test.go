package services

import (
	"testing"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSettingsServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), newServiceTestGormConfig())
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.AppSettings{}))
	return db
}

func TestSettingsService_GetSettingsInitializesDefaultsAndUpdatePersists(t *testing.T) {
	db := setupSettingsServiceTestDB(t)
	service := NewSettingsService(db)

	settings, err := service.GetSettings()
	require.NoError(t, err)
	require.Equal(t, "Cornerstone", settings.SystemName)
	require.True(t, settings.AllowRegistration)
	require.Equal(t, 50, settings.MaxFileSize)
	require.Equal(t, "./plugins", settings.PluginWorkDir)

	updated, err := service.UpdateSettings(UpdateSettingsRequest{
		SystemName:        "Cornerstone Secure",
		SystemDescription: "secured",
		AllowRegistration: false,
		MaxFileSize:       128,
		DBType:            "sqlite",
		DBPoolSize:        20,
		DBTimeout:         45,
		PluginTimeout:     120,
		PluginWorkDir:     "./runtime/plugins",
		PluginAutoUpdate:  true,
	}, "usr_admin")
	require.NoError(t, err)
	require.Equal(t, "usr_admin", updated.UpdatedBy)

	runtimeTimeout, runtimeDir, err := service.GetPluginRuntimeConfig()
	require.NoError(t, err)
	require.Equal(t, 120, runtimeTimeout)
	require.Equal(t, "./runtime/plugins", runtimeDir)

	var stored models.AppSettings
	require.NoError(t, db.Where("id = ?", 1).First(&stored).Error)
	require.Equal(t, "Cornerstone Secure", stored.SystemName)
	require.False(t, stored.AllowRegistration)
	require.Equal(t, 128, stored.MaxFileSize)
	require.Equal(t, "usr_admin", stored.UpdatedBy)
}

func TestSettingsService_GetPluginRuntimeConfigFallsBackForInvalidStoredValues(t *testing.T) {
	db := setupSettingsServiceTestDB(t)
	service := NewSettingsService(db)

	require.NoError(t, db.Create(&models.AppSettings{
		ID:                1,
		SystemName:        "Cornerstone",
		SystemDescription: "invalid runtime",
		AllowRegistration: true,
		MaxFileSize:       50,
		DBType:            "sqlite",
		DBPoolSize:        10,
		DBTimeout:         30,
		PluginTimeout:     0,
		PluginWorkDir:     "",
	}).Error)

	timeout, workDir, err := service.GetPluginRuntimeConfig()
	require.NoError(t, err)
	require.Equal(t, 300, timeout)
	require.Equal(t, "./plugins", workDir)
}
