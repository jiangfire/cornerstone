package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/config"
	internaldb "github.com/jiangfire/cornerstone/backend/internal/db"
	"github.com/jiangfire/cornerstone/backend/internal/models"
	pkgdb "github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/stretchr/testify/require"
)

func setupSettingsHandlerTestDB(t *testing.T) {
	t.Helper()

	_ = os.Setenv("JWT_SECRET", "settings-handler-test-secret")

	dbFile := t.TempDir() + "\\settings-handler.db"
	require.NoError(t, pkgdb.InitDB(config.DatabaseConfig{
		Type:        "sqlite",
		URL:         dbFile,
		MaxOpen:     1,
		MaxIdle:     1,
		MaxLifetime: 60,
	}))
	t.Cleanup(func() {
		_ = pkgdb.CloseDB()
	})
	require.NoError(t, internaldb.Migrate())
}

func TestUpdateSettingsRequiresSystemAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupSettingsHandlerTestDB(t)

	database := pkgdb.DB()
	admin := models.User{
		Username:      "settings_admin",
		Email:         "settings_admin@example.com",
		Password:      "hashed",
		IsSystemAdmin: true,
	}
	user := models.User{
		Username: "settings_user",
		Email:    "settings_user@example.com",
		Password: "hashed",
	}
	require.NoError(t, database.Create(&admin).Error)
	require.NoError(t, database.Create(&user).Error)

	body := `{"system_name":"Cornerstone","system_description":"secure","allow_registration":false,"max_file_size":64,"db_type":"sqlite","db_pool_size":10,"db_timeout":30,"plugin_timeout":300,"plugin_work_dir":"./plugins","plugin_auto_update":false}`

	userReq := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	userReq.Header.Set("Content-Type", "application/json")
	userRecorder := httptest.NewRecorder()
	userContext, _ := gin.CreateTestContext(userRecorder)
	userContext.Request = userReq
	userContext.Set("user_id", user.ID)
	UpdateSettings(userContext)
	require.Equal(t, http.StatusForbidden, userRecorder.Code)

	adminReq := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	adminReq.Header.Set("Content-Type", "application/json")
	adminRecorder := httptest.NewRecorder()
	adminContext, _ := gin.CreateTestContext(adminRecorder)
	adminContext.Request = adminReq
	adminContext.Set("user_id", admin.ID)
	UpdateSettings(adminContext)
	require.Equal(t, http.StatusOK, adminRecorder.Code)

	getReq := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	getRecorder := httptest.NewRecorder()
	getContext, _ := gin.CreateTestContext(getRecorder)
	getContext.Request = getReq
	getContext.Set("user_id", user.ID)
	GetSettings(getContext)
	require.Equal(t, http.StatusForbidden, getRecorder.Code)
}
