package services

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates a temporary SQLite database for testing
func setupTestDB() *gorm.DB {
	// Use a unique temporary file for each test to avoid conflicts
	dbFile := fmt.Sprintf("test_validation_%d.db", time.Now().UnixNano())

	// Clean up any existing test database
	os.Remove(dbFile)

	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		panic(fmt.Sprintf("failed to connect to test database: %v", err))
	}

	// Auto migrate all models
	err = db.AutoMigrate(
		&models.User{},
		&models.Database{},
		&models.DatabaseAccess{},
		&models.Table{},
		&models.Field{},
		&models.Record{},
	)
	if err != nil {
		panic(fmt.Sprintf("failed to migrate models: %v", err))
	}

	return db
}

// cleanupTestDB removes test database files
func cleanupTestDB() {
	// Remove any test database files
	files, _ := filepath.Glob("test_validation_*.db")
	for _, file := range files {
		os.Remove(file)
	}
}

// TestMain sets up and tears down test environment
func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()

	// Cleanup
	cleanupTestDB()

	os.Exit(code)
}

// TestRecordValidation_RegexPattern tests regex validation for string fields
func TestRecordValidation_RegexPattern(t *testing.T) {
	db := setupTestDB()
	service := NewRecordService(db)

	// Create test user, database, and table
	user := models.User{ID: "usr_test", Username: "testuser", Email: "test@example.com", Password: "hash"}
	db.Create(&user)

	database := models.Database{ID: "db_test", Name: "TestDB", OwnerID: "usr_test"}
	db.Create(&database)

	access := models.DatabaseAccess{UserID: "usr_test", DatabaseID: "db_test", Role: "owner"}
	db.Create(&access)

	table := models.Table{ID: "tbl_test", DatabaseID: "db_test", Name: "TestTable"}
	db.Create(&table)

	// Create field with email validation regex
	emailField := models.Field{
		ID:       "fld_email",
		TableID:  "tbl_test",
		Name:     "Email",
		Type:     "string",
		Required: true,
		Options:  `{"validation":"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$","max_length":100}`,
	}
	db.Create(&emailField)

	// Test valid email
	validReq := CreateRecordRequest{
		TableID: "tbl_test",
		Data: map[string]interface{}{
			"Email": "test@example.com",
		},
	}
	_, err := service.CreateRecord(validReq, "usr_test")
	assert.NoError(t, err, "Valid email should pass validation")

	// Test invalid email
	invalidReq := CreateRecordRequest{
		TableID: "tbl_test",
		Data: map[string]interface{}{
			"Email": "invalid-email",
		},
	}
	_, err = service.CreateRecord(invalidReq, "usr_test")
	assert.Error(t, err, "Invalid email should fail validation")
	assert.Contains(t, err.Error(), "格式不匹配", "Error should mention format mismatch")
}

// TestRecordValidation_ProductCode tests product code format validation
func TestRecordValidation_ProductCode(t *testing.T) {
	db := setupTestDB()
	service := NewRecordService(db)

	// Setup test data
	user := models.User{ID: "usr_test", Username: "testuser", Email: "test@example.com", Password: "hash"}
	db.Create(&user)
	database := models.Database{ID: "db_test", Name: "TestDB", OwnerID: "usr_test"}
	db.Create(&database)
	access := models.DatabaseAccess{UserID: "usr_test", DatabaseID: "db_test", Role: "owner"}
	db.Create(&access)
	table := models.Table{ID: "tbl_test", DatabaseID: "db_test", Name: "TestTable"}
	db.Create(&table)

	// Create product code field with format: ABC-1234
	productField := models.Field{
		ID:       "fld_product",
		TableID:  "tbl_test",
		Name:     "ProductCode",
		Type:     "string",
		Required: true,
		Options:  `{"validation":"^[A-Z]{2,3}-\\d{4}$","max_length":20}`,
	}
	db.Create(&productField)

	// Test valid product codes
	validCodes := []string{"AB-1234", "ABC-5678", "XYZ-9999"}
	for _, code := range validCodes {
		req := CreateRecordRequest{
			TableID: "tbl_test",
			Data:    map[string]interface{}{"ProductCode": code},
		}
		_, err := service.CreateRecord(req, "usr_test")
		assert.NoError(t, err, "Valid product code %s should pass", code)
	}

	// Test invalid product codes
	invalidCodes := []string{"ab-1234", "A-1234", "ABCD-1234", "AB-123", "AB-12345"}
	for _, code := range invalidCodes {
		req := CreateRecordRequest{
			TableID: "tbl_test",
			Data:    map[string]interface{}{"ProductCode": code},
		}
		_, err := service.CreateRecord(req, "usr_test")
		assert.Error(t, err, "Invalid product code %s should fail", code)
	}
}

// TestRecordValidation_MaxLength tests max length validation
func TestRecordValidation_MaxLength(t *testing.T) {
	db := setupTestDB()
	service := NewRecordService(db)

	// Setup test data
	user := models.User{ID: "usr_test", Username: "testuser", Email: "test@example.com", Password: "hash"}
	db.Create(&user)
	database := models.Database{ID: "db_test", Name: "TestDB", OwnerID: "usr_test"}
	db.Create(&database)
	access := models.DatabaseAccess{UserID: "usr_test", DatabaseID: "db_test", Role: "owner"}
	db.Create(&access)
	table := models.Table{ID: "tbl_test", DatabaseID: "db_test", Name: "TestTable"}
	db.Create(&table)

	// Create field with max length 10
	nameField := models.Field{
		ID:       "fld_name",
		TableID:  "tbl_test",
		Name:     "Name",
		Type:     "string",
		Required: true,
		Options:  `{"max_length":10}`,
	}
	db.Create(&nameField)

	// Test valid length
	validReq := CreateRecordRequest{
		TableID: "tbl_test",
		Data:    map[string]interface{}{"Name": "ShortName"},
	}
	_, err := service.CreateRecord(validReq, "usr_test")
	assert.NoError(t, err, "Name within max length should pass")

	// Test invalid length
	invalidReq := CreateRecordRequest{
		TableID: "tbl_test",
		Data:    map[string]interface{}{"Name": "ThisNameIsWayTooLong"},
	}
	_, err = service.CreateRecord(invalidReq, "usr_test")
	assert.Error(t, err, "Name exceeding max length should fail")
	assert.Contains(t, err.Error(), "长度不能超过", "Error should mention max length")
}

// TestRecordValidation_SelectOptions tests single and multi-select validation
func TestRecordValidation_SelectOptions(t *testing.T) {
	db := setupTestDB()
	service := NewRecordService(db)

	// Setup test data
	user := models.User{ID: "usr_test", Username: "testuser", Email: "test@example.com", Password: "hash"}
	db.Create(&user)
	database := models.Database{ID: "db_test", Name: "TestDB", OwnerID: "usr_test"}
	db.Create(&database)
	access := models.DatabaseAccess{UserID: "usr_test", DatabaseID: "db_test", Role: "owner"}
	db.Create(&access)
	table := models.Table{ID: "tbl_test", DatabaseID: "db_test", Name: "TestTable"}
	db.Create(&table)

	// Create single select field
	categoryField := models.Field{
		ID:       "fld_category",
		TableID:  "tbl_test",
		Name:     "Category",
		Type:     "single_select",
		Required: true,
		Options:  `{"options":["Electronics","Books","Clothing"]}`,
	}
	db.Create(&categoryField)

	// Create multi select field
	tagsField := models.Field{
		ID:       "fld_tags",
		TableID:  "tbl_test",
		Name:     "Tags",
		Type:     "multi_select",
		Required: false,
		Options:  `{"options":["New","Sale","Featured"]}`,
	}
	db.Create(&tagsField)

	// Test valid selections
	validReq := CreateRecordRequest{
		TableID: "tbl_test",
		Data: map[string]interface{}{
			"Category": "Electronics",
			"Tags":     []interface{}{"New", "Featured"},
		},
	}
	_, err := service.CreateRecord(validReq, "usr_test")
	assert.NoError(t, err, "Valid selections should pass")

	// Test invalid single select
	invalidSingleReq := CreateRecordRequest{
		TableID: "tbl_test",
		Data: map[string]interface{}{
			"Category": "InvalidCategory",
		},
	}
	_, err = service.CreateRecord(invalidSingleReq, "usr_test")
	assert.Error(t, err, "Invalid single select value should fail")

	// Test invalid multi select
	invalidMultiReq := CreateRecordRequest{
		TableID: "tbl_test",
		Data: map[string]interface{}{
			"Category": "Electronics",
			"Tags":     []interface{}{"New", "InvalidTag"},
		},
	}
	_, err = service.CreateRecord(invalidMultiReq, "usr_test")
	assert.Error(t, err, "Invalid multi select value should fail")
}

// TestRecordValidation_RequiredFields tests required field validation
func TestRecordValidation_RequiredFields(t *testing.T) {
	db := setupTestDB()
	service := NewRecordService(db)

	// Setup test data
	user := models.User{ID: "usr_test", Username: "testuser", Email: "test@example.com", Password: "hash"}
	db.Create(&user)
	database := models.Database{ID: "db_test", Name: "TestDB", OwnerID: "usr_test"}
	db.Create(&database)
	access := models.DatabaseAccess{UserID: "usr_test", DatabaseID: "db_test", Role: "owner"}
	db.Create(&access)
	table := models.Table{ID: "tbl_test", DatabaseID: "db_test", Name: "TestTable"}
	db.Create(&table)

	// Create required fields
	requiredField1 := models.Field{
		ID:       "fld_req1",
		TableID:  "tbl_test",
		Name:     "RequiredField1",
		Type:     "string",
		Required: true,
	}
	db.Create(&requiredField1)

	requiredField2 := models.Field{
		ID:       "fld_req2",
		TableID:  "tbl_test",
		Name:     "RequiredField2",
		Type:     "number",
		Required: true,
	}
	db.Create(&requiredField2)

	optionalField := models.Field{
		ID:       "fld_opt",
		TableID:  "tbl_test",
		Name:     "OptionalField",
		Type:     "string",
		Required: false,
	}
	db.Create(&optionalField)

	// Test all required fields provided
	validReq := CreateRecordRequest{
		TableID: "tbl_test",
		Data: map[string]interface{}{
			"RequiredField1": "value1",
			"RequiredField2": 42,
			"OptionalField":  "optional",
		},
	}
	_, err := service.CreateRecord(validReq, "usr_test")
	assert.NoError(t, err, "All required fields provided should pass")

	// Test missing required field
	invalidReq := CreateRecordRequest{
		TableID: "tbl_test",
		Data: map[string]interface{}{
			"RequiredField1": "value1",
			// Missing RequiredField2
			"OptionalField": "optional",
		},
	}
	_, err = service.CreateRecord(invalidReq, "usr_test")
	assert.Error(t, err, "Missing required field should fail")
	assert.Contains(t, err.Error(), "是必填的", "Error should mention required field")
}

// TestRecordValidation_FieldNameOrID tests that validation works with both field names and IDs
func TestRecordValidation_FieldNameOrID(t *testing.T) {
	db := setupTestDB()
	service := NewRecordService(db)

	// Setup test data
	user := models.User{ID: "usr_test", Username: "testuser", Email: "test@example.com", Password: "hash"}
	db.Create(&user)
	database := models.Database{ID: "db_test", Name: "TestDB", OwnerID: "usr_test"}
	db.Create(&database)
	access := models.DatabaseAccess{UserID: "usr_test", DatabaseID: "db_test", Role: "owner"}
	db.Create(&access)
	table := models.Table{ID: "tbl_test", DatabaseID: "db_test", Name: "TestTable"}
	db.Create(&table)

	field := models.Field{
		ID:       "fld_test",
		TableID:  "tbl_test",
		Name:     "TestField",
		Type:     "string",
		Required: true,
		Options:  `{"max_length":5}`,
	}
	db.Create(&field)

	// Test using field name
	reqWithName := CreateRecordRequest{
		TableID: "tbl_test",
		Data:    map[string]interface{}{"TestField": "abc"},
	}
	_, err := service.CreateRecord(reqWithName, "usr_test")
	assert.NoError(t, err, "Should work with field name")

	// Test using field ID
	reqWithID := CreateRecordRequest{
		TableID: "tbl_test",
		Data:    map[string]interface{}{"fld_test": "abc"},
	}
	_, err = service.CreateRecord(reqWithID, "usr_test")
	assert.NoError(t, err, "Should work with field ID")

	// Test validation still works with field name
	invalidReq := CreateRecordRequest{
		TableID: "tbl_test",
		Data:    map[string]interface{}{"TestField": "toolong"},
	}
	_, err = service.CreateRecord(invalidReq, "usr_test")
	assert.Error(t, err, "Validation should work with field name")

	// Test validation still works with field ID
	invalidReqWithID := CreateRecordRequest{
		TableID: "tbl_test",
		Data:    map[string]interface{}{"fld_test": "toolong"},
	}
	_, err = service.CreateRecord(invalidReqWithID, "usr_test")
	assert.Error(t, err, "Validation should work with field ID")
}

// TestRecordValidation_NumberType tests number type validation
func TestRecordValidation_NumberType(t *testing.T) {
	db := setupTestDB()
	service := NewRecordService(db)

	// Setup test data
	user := models.User{ID: "usr_test", Username: "testuser", Email: "test@example.com", Password: "hash"}
	db.Create(&user)
	database := models.Database{ID: "db_test", Name: "TestDB", OwnerID: "usr_test"}
	db.Create(&database)
	access := models.DatabaseAccess{UserID: "usr_test", DatabaseID: "db_test", Role: "owner"}
	db.Create(&access)
	table := models.Table{ID: "tbl_test", DatabaseID: "db_test", Name: "TestTable"}
	db.Create(&table)

	field := models.Field{
		ID:       "fld_num",
		TableID:  "tbl_test",
		Name:     "Quantity",
		Type:     "number",
		Required: true,
	}
	db.Create(&field)

	// Test valid number types
	validNumbers := []interface{}{
		42,
		3.14,
		int32(100),
		int64(1000),
		float32(2.5),
	}

	for i, num := range validNumbers {
		req := CreateRecordRequest{
			TableID: "tbl_test",
			Data:    map[string]interface{}{"Quantity": num},
		}
		_, err := service.CreateRecord(req, "usr_test")
		assert.NoError(t, err, "Number type %T should pass", num)

		// Small delay to ensure unique timestamps for ID generation
		if i < len(validNumbers)-1 {
			time.Sleep(1 * time.Millisecond)
		}
	}

	// Test invalid number types
	invalidNumbers := []interface{}{
		"42",
		true,
		[]int{1, 2, 3},
		map[string]interface{}{"value": 42},
	}

	for _, num := range invalidNumbers {
		req := CreateRecordRequest{
			TableID: "tbl_test",
			Data:    map[string]interface{}{"Quantity": num},
		}
		_, err := service.CreateRecord(req, "usr_test")
		assert.Error(t, err, "Non-number type %T should fail", num)
	}
}

// TestRecordValidation_BooleanType tests boolean type validation
func TestRecordValidation_BooleanType(t *testing.T) {
	db := setupTestDB()
	service := NewRecordService(db)

	// Setup test data
	user := models.User{ID: "usr_test", Username: "testuser", Email: "test@example.com", Password: "hash"}
	db.Create(&user)
	database := models.Database{ID: "db_test", Name: "TestDB", OwnerID: "usr_test"}
	db.Create(&database)
	access := models.DatabaseAccess{UserID: "usr_test", DatabaseID: "db_test", Role: "owner"}
	db.Create(&access)
	table := models.Table{ID: "tbl_test", DatabaseID: "db_test", Name: "TestTable"}
	db.Create(&table)

	field := models.Field{
		ID:       "fld_bool",
		TableID:  "tbl_test",
		Name:     "IsActive",
		Type:     "boolean",
		Required: true,
	}
	db.Create(&field)

	// Test valid boolean
	validReq := CreateRecordRequest{
		TableID: "tbl_test",
		Data:    map[string]interface{}{"IsActive": true},
	}
	_, err := service.CreateRecord(validReq, "usr_test")
	assert.NoError(t, err, "Boolean true should pass")

	validReq2 := CreateRecordRequest{
		TableID: "tbl_test",
		Data:    map[string]interface{}{"IsActive": false},
	}
	_, err = service.CreateRecord(validReq2, "usr_test")
	assert.NoError(t, err, "Boolean false should pass")

	// Test invalid boolean
	invalidReq := CreateRecordRequest{
		TableID: "tbl_test",
		Data:    map[string]interface{}{"IsActive": "true"},
	}
	_, err = service.CreateRecord(invalidReq, "usr_test")
	assert.Error(t, err, "String 'true' should fail boolean validation")
}

// TestRecordValidation_DateType tests date/datetime type validation
func TestRecordValidation_DateType(t *testing.T) {
	db := setupTestDB()
	service := NewRecordService(db)

	// Setup test data
	user := models.User{ID: "usr_test", Username: "testuser", Email: "test@example.com", Password: "hash"}
	db.Create(&user)
	database := models.Database{ID: "db_test", Name: "TestDB", OwnerID: "usr_test"}
	db.Create(&database)
	access := models.DatabaseAccess{UserID: "usr_test", DatabaseID: "db_test", Role: "owner"}
	db.Create(&access)
	table := models.Table{ID: "tbl_test", DatabaseID: "db_test", Name: "TestTable"}
	db.Create(&table)

	dateField := models.Field{
		ID:       "fld_date",
		TableID:  "tbl_test",
		Name:     "DateField",
		Type:     "date",
		Required: true,
	}
	db.Create(&dateField)

	datetimeField := models.Field{
		ID:       "fld_datetime",
		TableID:  "tbl_test",
		Name:     "DateTimeField",
		Type:     "datetime",
		Required: false,
	}
	db.Create(&datetimeField)

	// Test valid date strings
	validReq := CreateRecordRequest{
		TableID: "tbl_test",
		Data: map[string]interface{}{
			"DateField":     "2026-01-09",
			"DateTimeField": "2026-01-09T21:30:00Z",
		},
	}
	_, err := service.CreateRecord(validReq, "usr_test")
	assert.NoError(t, err, "Date strings should pass")

	// Test invalid date type
	invalidReq := CreateRecordRequest{
		TableID: "tbl_test",
		Data: map[string]interface{}{
			"DateField": 12345, // number instead of string
		},
	}
	_, err = service.CreateRecord(invalidReq, "usr_test")
	assert.Error(t, err, "Non-string date should fail")
}

// TestRecordUpdate_OptimisticLocking tests optimistic locking during updates
func TestRecordUpdate_OptimisticLocking(t *testing.T) {
	db := setupTestDB()
	service := NewRecordService(db)

	// Setup test data
	user := models.User{ID: "usr_test", Username: "testuser", Email: "test@example.com", Password: "hash"}
	db.Create(&user)
	database := models.Database{ID: "db_test", Name: "TestDB", OwnerID: "usr_test"}
	db.Create(&database)
	access := models.DatabaseAccess{UserID: "usr_test", DatabaseID: "db_test", Role: "owner"}
	db.Create(&access)
	table := models.Table{ID: "tbl_test", DatabaseID: "db_test", Name: "TestTable"}
	db.Create(&table)

	field := models.Field{
		ID:       "fld_data",
		TableID:  "tbl_test",
		Name:     "Data",
		Type:     "string",
		Required: true,
	}
	db.Create(&field)

	// Create a record
	createReq := CreateRecordRequest{
		TableID: "tbl_test",
		Data:    map[string]interface{}{"Data": "original"},
	}
	record, err := service.CreateRecord(createReq, "usr_test")
	assert.NoError(t, err)
	assert.Equal(t, 1, record.Version, "Initial version should be 1")

	// Test valid update with correct version
	updateReq := UpdateRecordRequest{
		Data:    map[string]interface{}{"Data": "updated"},
		Version: 1,
	}
	updatedRecord, err := service.UpdateRecord(record.ID, updateReq, "usr_test")
	assert.NoError(t, err)
	assert.Equal(t, 2, updatedRecord.Version, "Version should increment to 2")

	// Test invalid update with wrong version
	invalidUpdateReq := UpdateRecordRequest{
		Data:    map[string]interface{}{"Data": "conflict"},
		Version: 1, // Wrong version, should be 2
	}
	_, err = service.UpdateRecord(record.ID, invalidUpdateReq, "usr_test")
	assert.Error(t, err, "Update with wrong version should fail")
	assert.Contains(t, err.Error(), "记录已被其他用户修改", "Error should mention concurrent modification")
}

// TestRecordUpdate_ValidationOnUpdate tests that validation works on updates too
func TestRecordUpdate_ValidationOnUpdate(t *testing.T) {
	db := setupTestDB()
	service := NewRecordService(db)

	// Setup test data
	user := models.User{ID: "usr_test", Username: "testuser", Email: "test@example.com", Password: "hash"}
	db.Create(&user)
	database := models.Database{ID: "db_test", Name: "TestDB", OwnerID: "usr_test"}
	db.Create(&database)
	access := models.DatabaseAccess{UserID: "usr_test", DatabaseID: "db_test", Role: "owner"}
	db.Create(&access)
	table := models.Table{ID: "tbl_test", DatabaseID: "db_test", Name: "TestTable"}
	db.Create(&table)

	field := models.Field{
		ID:       "fld_code",
		TableID:  "tbl_test",
		Name:     "Code",
		Type:     "string",
		Required: true,
		Options:  `{"validation":"^[A-Z]{3}$","max_length":3}`,
	}
	db.Create(&field)

	// Create valid record
	createReq := CreateRecordRequest{
		TableID: "tbl_test",
		Data:    map[string]interface{}{"Code": "ABC"},
	}
	record, err := service.CreateRecord(createReq, "usr_test")
	assert.NoError(t, err)

	// Test valid update
	validUpdateReq := UpdateRecordRequest{
		Data:    map[string]interface{}{"Code": "XYZ"},
		Version: 1,
	}
	_, err = service.UpdateRecord(record.ID, validUpdateReq, "usr_test")
	assert.NoError(t, err, "Valid update should pass")

	// Test invalid update (validation should still apply)
	invalidUpdateReq := UpdateRecordRequest{
		Data:    map[string]interface{}{"Code": "invalid"},
		Version: 2,
	}
	_, err = service.UpdateRecord(record.ID, invalidUpdateReq, "usr_test")
	assert.Error(t, err, "Invalid update should fail validation")
	assert.Contains(t, err.Error(), "长度不能超过", "Error should mention max length")
}

// TestBatchCreateValidation tests validation in batch creation
func TestBatchCreateValidation(t *testing.T) {
	db := setupTestDB()
	service := NewRecordService(db)

	// Setup test data
	user := models.User{ID: "usr_test", Username: "testuser", Email: "test@example.com", Password: "hash"}
	db.Create(&user)
	database := models.Database{ID: "db_test", Name: "TestDB", OwnerID: "usr_test"}
	db.Create(&database)
	access := models.DatabaseAccess{UserID: "usr_test", DatabaseID: "db_test", Role: "owner"}
	db.Create(&access)
	table := models.Table{ID: "tbl_test", DatabaseID: "db_test", Name: "TestTable"}
	db.Create(&table)

	field := models.Field{
		ID:       "fld_val",
		TableID:  "tbl_test",
		Name:     "Value",
		Type:     "string",
		Required: true,
		Options:  `{"max_length":5}`,
	}
	db.Create(&field)

	// Test batch creation with valid data
	validReq := CreateRecordRequest{
		TableID: "tbl_test",
		Data:    map[string]interface{}{"Value": "valid"},
	}

	// Add a small delay before batch creation to ensure unique timestamps
	time.Sleep(5 * time.Millisecond)

	records, err := service.BatchCreateRecords(validReq, "usr_test", 3)
	assert.NoError(t, err)
	assert.Len(t, records, 3, "Should create 3 records")

	// Test batch creation with invalid data
	invalidReq := CreateRecordRequest{
		TableID: "tbl_test",
		Data:    map[string]interface{}{"Value": "toolong"},
	}
	_, err = service.BatchCreateRecords(invalidReq, "usr_test", 2)
	assert.Error(t, err, "Batch creation should fail with invalid data")
}

// TestRecordQuery_WithFilters tests that query filters work correctly
func TestRecordQuery_WithFilters(t *testing.T) {
	db := setupTestDB()
	service := NewRecordService(db)

	// Setup test data
	user := models.User{ID: "usr_test", Username: "testuser", Email: "test@example.com", Password: "hash"}
	db.Create(&user)
	database := models.Database{ID: "db_test", Name: "TestDB", OwnerID: "usr_test"}
	db.Create(&database)
	access := models.DatabaseAccess{UserID: "usr_test", DatabaseID: "db_test", Role: "owner"}
	db.Create(&access)
	table := models.Table{ID: "tbl_test", DatabaseID: "db_test", Name: "TestTable"}
	db.Create(&table)

	field := models.Field{
		ID:       "fld_status",
		TableID:  "tbl_test",
		Name:     "Status",
		Type:     "string",
		Required: true,
	}
	db.Create(&field)

	// Create multiple records with different statuses
	statuses := []string{"active", "inactive", "active", "pending"}
	for i, status := range statuses {
		req := CreateRecordRequest{
			TableID: "tbl_test",
			Data:    map[string]interface{}{"Status": status},
		}
		_, err := service.CreateRecord(req, "usr_test")
		assert.NoError(t, err)

		// Small delay to ensure unique timestamps
		if i < len(statuses)-1 {
			time.Sleep(1 * time.Millisecond)
		}
	}

	// Query with filter for "active" status
	queryReq := QueryRequest{
		TableID: "tbl_test",
		Limit:   10,
		Filter:  `{"Status":"active"}`,
	}
	result, err := service.ListRecords(queryReq, "usr_test")
	assert.NoError(t, err)
	assert.Len(t, result.Records, 2, "Should find 2 active records")
	assert.Equal(t, int64(2), result.Total, "Total should be 2")
}

// TestRecordValidation_CombinedRules tests multiple validation rules on same field
func TestRecordValidation_CombinedRules(t *testing.T) {
	db := setupTestDB()
	service := NewRecordService(db)

	// Setup test data
	user := models.User{ID: "usr_test", Username: "testuser", Email: "test@example.com", Password: "hash"}
	db.Create(&user)
	database := models.Database{ID: "db_test", Name: "TestDB", OwnerID: "usr_test"}
	db.Create(&database)
	access := models.DatabaseAccess{UserID: "usr_test", DatabaseID: "db_test", Role: "owner"}
	db.Create(&access)
	table := models.Table{ID: "tbl_test", DatabaseID: "db_test", Name: "TestTable"}
	db.Create(&table)

	// Field with both regex and max length validation
	field := models.Field{
		ID:       "fld_combined",
		TableID:  "tbl_test",
		Name:     "CombinedField",
		Type:     "string",
		Required: true,
		Options:  `{"validation":"^[A-Z]+$","max_length":5}`,
	}
	db.Create(&field)

	// Test: valid format and length
	req1 := CreateRecordRequest{
		TableID: "tbl_test",
		Data:    map[string]interface{}{"CombinedField": "ABC"},
	}
	_, err := service.CreateRecord(req1, "usr_test")
	assert.NoError(t, err, "Valid format and length should pass")

	// Test: valid format but too long
	req2 := CreateRecordRequest{
		TableID: "tbl_test",
		Data:    map[string]interface{}{"CombinedField": "ABCDEF"},
	}
	_, err = service.CreateRecord(req2, "usr_test")
	assert.Error(t, err, "Valid format but too long should fail")
	assert.Contains(t, err.Error(), "长度不能超过", "Should fail on length")

	// Test: invalid format but correct length
	req3 := CreateRecordRequest{
		TableID: "tbl_test",
		Data:    map[string]interface{}{"CombinedField": "abc"},
	}
	_, err = service.CreateRecord(req3, "usr_test")
	assert.Error(t, err, "Invalid format but correct length should fail")
	assert.Contains(t, err.Error(), "格式不匹配", "Should fail on format")

	// Test: invalid format and too long
	req4 := CreateRecordRequest{
		TableID: "tbl_test",
		Data:    map[string]interface{}{"CombinedField": "abc123"},
	}
	_, err = service.CreateRecord(req4, "usr_test")
	assert.Error(t, err, "Invalid format and too long should fail")
	// Should fail on first validation encountered
}
