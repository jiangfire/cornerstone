package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/testutil"
)

func setupTokenTestDB(t *testing.T) *gorm.DB {
	return testutil.SetupTestDB(t)
}

func TestTokenService_CreateToken(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	token, err := svc.CreateToken(CreateTokenRequest{
		Name:   "test-token",
		Scopes: `{"databases":{},"tables":{}}`,
	})
	require.NoError(t, err)
	assert.NotNil(t, token)
	assert.NotEmpty(t, token.ID)
	assert.NotEmpty(t, token.Token)
	assert.Equal(t, "test-token", token.Name)
	assert.False(t, token.IsMaster)
	assert.Equal(t, `{"databases":{},"tables":{}}`, token.Scopes)
}

func TestTokenService_CreateTokenWithExpiry(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	expiresAt := time.Now().Add(24 * time.Hour)
	token, err := svc.CreateToken(CreateTokenRequest{
		Name:      "expiring",
		Scopes:    "{}",
		ExpiresAt: &expiresAt,
	})
	require.NoError(t, err)
	assert.NotNil(t, token.ExpiresAt)
}

func TestTokenService_ListTokens_Master(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	master := &models.Token{Name: "master", IsMaster: true, Scopes: "{}"}
	require.NoError(t, d.Create(master).Error)

	worker1 := &models.Token{Name: "worker1", IsMaster: false, Scopes: "{}"}
	require.NoError(t, d.Create(worker1).Error)

	worker2 := &models.Token{Name: "worker2", IsMaster: false, Scopes: "{}"}
	require.NoError(t, d.Create(worker2).Error)

	tokens, err := svc.ListTokens(master.ID, true)
	require.NoError(t, err)
	assert.Len(t, tokens, 2)

	for _, tok := range tokens {
		assert.False(t, tok.IsMaster)
	}
}

func TestTokenService_ListTokens_NonMaster(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	master := &models.Token{Name: "master", IsMaster: true, Scopes: "{}"}
	require.NoError(t, d.Create(master).Error)

	worker1 := &models.Token{Name: "worker1", IsMaster: false, Scopes: "{}"}
	require.NoError(t, d.Create(worker1).Error)

	worker2 := &models.Token{Name: "worker2", IsMaster: false, Scopes: "{}"}
	require.NoError(t, d.Create(worker2).Error)

	tokens, err := svc.ListTokens(worker1.ID, false)
	require.NoError(t, err)
	assert.Len(t, tokens, 1)
	assert.Equal(t, worker1.ID, tokens[0].ID)
}

func TestTokenService_DeleteToken_MasterCanDeleteAny(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	master := &models.Token{Name: "master", IsMaster: true, Scopes: "{}"}
	require.NoError(t, d.Create(master).Error)

	worker := &models.Token{Name: "worker", IsMaster: false, Scopes: "{}"}
	require.NoError(t, d.Create(worker).Error)

	err := svc.DeleteToken(master.ID, worker.ID, true)
	require.NoError(t, err)

	var count int64
	d.Model(&models.Token{}).Where("id = ?", worker.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestTokenService_DeleteToken_NonMasterCanDeleteOwn(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	worker := &models.Token{Name: "worker", IsMaster: false, Scopes: "{}"}
	require.NoError(t, d.Create(worker).Error)

	err := svc.DeleteToken(worker.ID, worker.ID, false)
	require.NoError(t, err)

	var count int64
	d.Model(&models.Token{}).Where("id = ?", worker.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestTokenService_DeleteToken_NonMasterCantDeleteOthers(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	worker1 := &models.Token{Name: "worker1", IsMaster: false, Scopes: "{}"}
	require.NoError(t, d.Create(worker1).Error)

	worker2 := &models.Token{Name: "worker2", IsMaster: false, Scopes: "{}"}
	require.NoError(t, d.Create(worker2).Error)

	err := svc.DeleteToken(worker1.ID, worker2.ID, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied: cannot delete other tokens")
}

func TestTokenService_DeleteToken_Nonexistent(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	master := &models.Token{Name: "master", IsMaster: true, Scopes: "{}"}
	require.NoError(t, d.Create(master).Error)

	err := svc.DeleteToken(master.ID, "nonexistent_id", true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token not found")
}

func TestTokenService_DeleteToken_NonMasterCantDeleteMaster(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	master := &models.Token{Name: "master", IsMaster: true, Scopes: "{}"}
	require.NoError(t, d.Create(master).Error)

	worker := &models.Token{Name: "worker", IsMaster: false, Scopes: "{}"}
	require.NoError(t, d.Create(worker).Error)

	err := svc.DeleteToken(worker.ID, master.ID, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied: cannot delete other tokens")
}

func TestTokenService_UpdateToken(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	worker := &models.Token{Name: "worker", IsMaster: false, Scopes: "{}"}
	require.NoError(t, d.Create(worker).Error)

	newScopes := `{"databases":{"db_1":"admin"},"tables":{}}`
	updated, err := svc.UpdateToken(worker.ID, newScopes, nil)
	require.NoError(t, err)
	assert.Equal(t, newScopes, updated.Scopes)
	assert.Nil(t, updated.ExpiresAt)
}

func TestTokenService_UpdateToken_WithExpiry(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	worker := &models.Token{Name: "worker", IsMaster: false, Scopes: "{}"}
	require.NoError(t, d.Create(worker).Error)

	expiresAt := time.Now().Add(48 * time.Hour)
	newScopes := `{"databases":{},"tables":{}}`
	updated, err := svc.UpdateToken(worker.ID, newScopes, &expiresAt)
	require.NoError(t, err)
	assert.NotNil(t, updated.ExpiresAt)
}

func TestTokenService_UpdateToken_CantUpdateMaster(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	master := &models.Token{Name: "master", IsMaster: true, Scopes: "{}"}
	require.NoError(t, d.Create(master).Error)

	_, err := svc.UpdateToken(master.ID, `{"databases":{}}`, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot modify master token permissions")
}

func TestTokenService_UpdateToken_Nonexistent(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	_, err := svc.UpdateToken("nonexistent_id", "{}", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token not found")
}

func TestTokenService_DeleteToken_InvalidatesCache(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	master := &models.Token{Name: "master", IsMaster: true, Scopes: "{}"}
	require.NoError(t, d.Create(master).Error)

	worker := &models.Token{Name: "worker", IsMaster: false, Scopes: "{}"}
	require.NoError(t, d.Create(worker).Error)

	authz.ClearTokenCache()
	_, err := authz.NewAuthorizer(d, worker.ID)
	require.NoError(t, err)

	err = svc.DeleteToken(master.ID, worker.ID, true)
	require.NoError(t, err)

	_, err = authz.NewAuthorizer(d, worker.ID)
	assert.Error(t, err)
}

func TestTokenService_UpdateToken_InvalidatesCache(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	worker := &models.Token{Name: "worker", IsMaster: false, Scopes: `{"databases":{}}`}
	require.NoError(t, d.Create(worker).Error)

	authz.ClearTokenCache()
	a1, err := authz.NewAuthorizer(d, worker.ID)
	require.NoError(t, err)
	assert.False(t, a1.CanCreateDatabase())

	newScopes := `{"databases":{"db_1":"admin"}}`
	_, err = svc.UpdateToken(worker.ID, newScopes, nil)
	require.NoError(t, err)

	a2, err := authz.NewAuthorizer(d, worker.ID)
	require.NoError(t, err)
	assert.True(t, a2.CanAccessDatabase("db_1", "read"))
}

func TestTokenService_CreateToken_GeneratesID(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	token, err := svc.CreateToken(CreateTokenRequest{
		Name:   "auto-id",
		Scopes: "{}",
	})
	require.NoError(t, err)
	assert.Contains(t, token.ID, "tok_")
}

func TestTokenService_CreateToken_GeneratesTokenValue(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	token, err := svc.CreateToken(CreateTokenRequest{
		Name:   "auto-value",
		Scopes: "{}",
	})
	require.NoError(t, err)
	assert.Contains(t, token.Token, "cs_")
}

func TestTokenService_DeleteToken_MasterCanDeleteOwnMasterToken(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	master := &models.Token{Name: "master", IsMaster: true, Scopes: "{}"}
	require.NoError(t, d.Create(master).Error)

	err := svc.DeleteToken(master.ID, master.ID, true)
	require.NoError(t, err)
}
