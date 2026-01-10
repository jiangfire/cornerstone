package main

import (
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		panic(err)
	}
	fmt.Println("SQLite connection successful")

	// Test basic operation
	type TestTable struct {
		ID   string `gorm:"primaryKey"`
		Name string
	}

	err = db.AutoMigrate(&TestTable{})
	if err != nil {
		fmt.Printf("Migration error: %v\n", err)
		panic(err)
	}

	fmt.Println("Migration successful")
}