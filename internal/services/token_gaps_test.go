package services

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/models"
)

func TestTokenService_CreateToken_DBError(t *testing.T) {
	d := setupTokenTestDB(t)
	sqlDB, err := d.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	svc := NewTokenService(d)
	_, err = svc.CreateToken(CreateTokenRequest{
		Name:   "test",
		Scopes: "{}",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "创建 Token 失败")
}

func TestTokenService_ListTokens_DBError(t *testing.T) {
	d := setupTokenTestDB(t)
	sqlDB, err := d.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	svc := NewTokenService(d)
	_, err = svc.ListTokens("some_id", true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "查询 Token 列表失败")
}

func TestTokenService_DeleteToken_DBErrorOnQuery(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	master := &models.Token{Name: "master", IsMaster: true, Scopes: "{}"}
	require.NoError(t, d.Create(master).Error)

	target := &models.Token{Name: "target", IsMaster: false, Scopes: "{}"}
	require.NoError(t, d.Create(target).Error)

	sqlDB, err := d.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	err = svc.DeleteToken(master.ID, target.ID, true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "查询 Token 失败")
}

func TestTokenService_DeleteToken_DBErrorOnDelete(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	master := &models.Token{Name: "master", IsMaster: true, Scopes: "{}"}
	require.NoError(t, d.Create(master).Error)

	target := &models.Token{Name: "target", IsMaster: false, Scopes: "{}"}
	require.NoError(t, d.Create(target).Error)

	d.Callback().Delete().Before("gorm:delete").Register("test:force_delete_err", func(gdb *gorm.DB) {
		gdb.Error = fmt.Errorf("forced delete error")
	})
	t.Cleanup(func() { _ = d.Callback().Delete().Remove("test:force_delete_err") })

	err := svc.DeleteToken(master.ID, target.ID, true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "删除 Token 失败")
}

func TestTokenService_UpdateToken_DBErrorOnFirst(t *testing.T) {
	d := setupTokenTestDB(t)
	sqlDB, err := d.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	svc := NewTokenService(d)
	_, err = svc.UpdateToken("any_id", "{}", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "查询 Token 失败")
}

func TestTokenService_UpdateToken_DBErrorOnUpdate(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	worker := &models.Token{Name: "worker", IsMaster: false, Scopes: "{}"}
	require.NoError(t, d.Create(worker).Error)

	d.Callback().Update().Before("gorm:update").Register("test:force_update_err", func(gdb *gorm.DB) {
		gdb.Error = fmt.Errorf("forced update error")
	})
	t.Cleanup(func() { _ = d.Callback().Update().Remove("test:force_update_err") })

	_, err := svc.UpdateToken(worker.ID, `{"databases":{}}`, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "更新 Token 失败")
}

func TestTokenService_UpdateToken_DBErrorOnRequery(t *testing.T) {
	d := setupTokenTestDB(t)
	svc := NewTokenService(d)

	worker := &models.Token{Name: "worker", IsMaster: false, Scopes: "{}"}
	require.NoError(t, d.Create(worker).Error)

	queryCount := 0
	d.Callback().Query().Before("gorm:query").Register("test:force_requery_err", func(gdb *gorm.DB) {
		queryCount++
		if queryCount >= 2 {
			gdb.Error = fmt.Errorf("forced requery error")
		}
	})
	t.Cleanup(func() { _ = d.Callback().Query().Remove("test:force_requery_err") })

	_, err := svc.UpdateToken(worker.ID, `{"databases":{}}`, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "查询最新 Token 失败")
}
