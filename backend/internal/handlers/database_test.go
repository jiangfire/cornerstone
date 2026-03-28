package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/config"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/models"
	pkgdb "github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/jiangfire/cornerstone/backend/pkg/utils"
	"github.com/stretchr/testify/require"
)

func setupDatabaseHandlerTest(t *testing.T) (*gin.Engine, models.User) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	_ = os.Setenv("JWT_SECRET", "test-secret-key-for-database-handler")

	dbFile := t.TempDir() + "\\database-handler-test.db"
	cfg := config.DatabaseConfig{
		Type: "sqlite",
		URL:  dbFile,
	}
	require.NoError(t, pkgdb.InitDB(cfg))
	t.Cleanup(func() {
		_ = pkgdb.CloseDB()
	})

	require.NoError(t, pkgdb.DB().AutoMigrate(
		&models.User{},
		&models.Database{},
		&models.DatabaseAccess{},
		&models.TokenBlacklist{},
	))

	owner := models.User{
		Username: "database_handler_owner",
		Email:    "database_handler_owner@example.com",
		Password: "hashed",
	}
	require.NoError(t, pkgdb.DB().Create(&owner).Error)

	router := gin.New()
	protected := router.Group("/api")
	protected.Use(middleware.Auth())
	protected.POST("/databases/:id/share", ShareDatabase)
	protected.PUT("/databases/:id/users/:user_id/role", UpdateDatabaseUserRole)

	return router, owner
}

func authHeaderForDatabaseUser(t *testing.T, user models.User) string {
	t.Helper()

	token, err := utils.GenerateToken(user.ID, user.Username, "user")
	require.NoError(t, err)
	return "Bearer " + token
}

func createDatabaseHandlerUser(t *testing.T, username string) models.User {
	t.Helper()

	user := models.User{
		Username: username,
		Email:    username + "@example.com",
		Password: "hashed",
	}
	require.NoError(t, pkgdb.DB().Create(&user).Error)
	return user
}

func createOwnedDatabaseHandlerFixture(t *testing.T, ownerID string, name string) models.Database {
	t.Helper()

	database := models.Database{
		Name:       name,
		OwnerID:    ownerID,
		IsPersonal: true,
	}
	require.NoError(t, pkgdb.DB().Create(&database).Error)
	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     ownerID,
		DatabaseID: database.ID,
		Role:       "owner",
	}).Error)
	return database
}

func TestDatabaseHandlerUpdateUserRoleAcceptsBodyWithoutUserID(t *testing.T) {
	router, owner := setupDatabaseHandlerTest(t)
	member := createDatabaseHandlerUser(t, "database_handler_member")
	database := createOwnedDatabaseHandlerFixture(t, owner.ID, "Handler Role DB")

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     member.ID,
		DatabaseID: database.ID,
		Role:       "viewer",
	}).Error)

	req := httptest.NewRequest(http.MethodPut, "/api/databases/"+database.ID+"/users/"+member.ID+"/role", bytes.NewBufferString(`{"role":"editor"}`))
	req.Header.Set("Authorization", authHeaderForDatabaseUser(t, owner))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var access models.DatabaseAccess
	require.NoError(t, pkgdb.DB().Where("database_id = ? AND user_id = ?", database.ID, member.ID).First(&access).Error)
	require.Equal(t, "editor", access.Role)
}

func TestDatabaseHandlerShareRejectsOwnerRoleAtRequestBoundary(t *testing.T) {
	router, owner := setupDatabaseHandlerTest(t)
	member := createDatabaseHandlerUser(t, "database_handler_share_member")
	database := createOwnedDatabaseHandlerFixture(t, owner.ID, "Handler Share DB")

	reqBody, err := json.Marshal(map[string]string{
		"user_id": member.ID,
		"role":    "owner",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/databases/"+database.ID+"/share", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", authHeaderForDatabaseUser(t, owner))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var count int64
	require.NoError(t, pkgdb.DB().Model(&models.DatabaseAccess{}).Where("database_id = ? AND user_id = ?", database.ID, member.ID).Count(&count).Error)
	require.Zero(t, count)
}
