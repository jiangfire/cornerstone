package services

import (
	"os"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSQLiteConnection(t *testing.T) {
	// Use a temporary file for SQLite database
	dbFile := "test_simple.db"

	// Clean up any existing test database
	os.Remove(dbFile)
	defer os.Remove(dbFile)

	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	// Test basic query
	var result string
	err = db.Raw("SELECT 1").Scan(&result).Error
	if err != nil {
		t.Fatalf("failed to execute basic query: %v", err)
	}

	if result != "1" {
		t.Fatalf("expected '1', got '%s'", result)
	}

	t.Log("SQLite connection test passed")
}