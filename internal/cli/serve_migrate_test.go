package cli

import (
	"bytes"
	"os"
	"sync"
	"testing"
	"time"

	pkgdb "github.com/jiangfire/cornerstone/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunMigrate_Success(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "migrate-test-*.db")
	require.NoError(t, err)
	tmpFile.Close()
	dbPath := tmpFile.Name()
	t.Cleanup(func() {
		os.Remove(dbPath)
	})

	t.Setenv("DB_TYPE", "sqlite")
	t.Setenv("DATABASE_URL", dbPath)
	t.Setenv("LOG_LEVEL", "error")

	t.Cleanup(func() {
		_ = pkgdb.CloseDB()
		pkgdb.SetDB(nil)
	})

	out := captureOutput(t, func() {
		err := runMigrate(migrateCmd, []string{})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "迁移完成")
}

func TestRunMigrate_ConfigLoadError(t *testing.T) {
	t.Setenv("DB_TYPE", "invalid_db_type")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("LOG_LEVEL", "error")

	t.Cleanup(func() {
		_ = pkgdb.CloseDB()
		pkgdb.SetDB(nil)
	})

	err := runMigrate(migrateCmd, []string{})
	assert.Error(t, err)
}

func TestExecute_Version(t *testing.T) {
	old := rootCmd.OutOrStdout()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	defer rootCmd.SetOut(old)

	rootCmd.SetArgs([]string{"--version"})
	err := rootCmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Cornerstone")
	rootCmd.SetArgs([]string{})
}

func TestExecute_InvalidCommand(t *testing.T) {
	rootCmd.SetArgs([]string{"nonexistent"})
	err := rootCmd.Execute()
	assert.Error(t, err)
	rootCmd.SetArgs([]string{})
}

func TestWaitPeriodicTasks_NilWG(t *testing.T) {
	assert.NotPanics(t, func() {
		waitPeriodicTasks(nil, time.Second)
	})
}

func TestWaitPeriodicTasks_CompletesQuickly(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wg.Done()
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		waitPeriodicTasks(&wg, time.Second)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("waitPeriodicTasks did not return in time")
	}
}

func TestWaitPeriodicTasks_Timeout(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		time.Sleep(5 * time.Second)
		wg.Done()
	}()

	start := time.Now()
	waitPeriodicTasks(&wg, 50*time.Millisecond)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 1*time.Second)
}
