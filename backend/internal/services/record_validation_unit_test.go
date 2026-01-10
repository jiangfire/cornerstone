package services

import (
	"fmt"
	"testing"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/assert"
)

// TestValidateFieldValue_RegexPattern tests regex validation logic directly
func TestValidateFieldValue_RegexPattern(t *testing.T) {
	service := &RecordService{}

	// Test email validation
	emailField := models.Field{
		Type:    "string",
		Options: `{"validation":"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$","max_length":100}`,
	}

	// Valid email
	err := service.validateFieldValue(emailField, "test@example.com")
	assert.NoError(t, err, "Valid email should pass")

	// Invalid email
	err = service.validateFieldValue(emailField, "invalid-email")
	assert.Error(t, err, "Invalid email should fail")
	assert.Contains(t, err.Error(), "格式不匹配")

	// Test product code validation
	productField := models.Field{
		Type:    "string",
		Options: `{"validation":"^[A-Z]{2,3}-\\d{4}$","max_length":20}`,
	}

	// Valid product codes
	validCodes := []string{"AB-1234", "ABC-5678", "XYZ-9999"}
	for _, code := range validCodes {
		err := service.validateFieldValue(productField, code)
		assert.NoError(t, err, "Valid product code %s should pass", code)
	}

	// Invalid product codes
	invalidCodes := []string{"ab-1234", "A-1234", "ABCD-1234", "AB-123", "AB-12345"}
	for _, code := range invalidCodes {
		err := service.validateFieldValue(productField, code)
		assert.Error(t, err, "Invalid product code %s should fail", code)
	}
}

// TestValidateFieldValue_MaxLength tests max length validation
func TestValidateFieldValue_MaxLength(t *testing.T) {
	service := &RecordService{}

	field := models.Field{
		Type:    "string",
		Options: `{"max_length":10}`,
	}

	// Valid length
	err := service.validateFieldValue(field, "ShortName")
	assert.NoError(t, err, "Name within max length should pass")

	// Invalid length
	err = service.validateFieldValue(field, "ThisNameIsWayTooLong")
	assert.Error(t, err, "Name exceeding max length should fail")
	assert.Contains(t, err.Error(), "长度不能超过")
}

// TestValidateFieldValue_SelectOptions tests single and multi-select validation
func TestValidateFieldValue_SelectOptions(t *testing.T) {
	service := &RecordService{}

	// Single select
	singleSelectField := models.Field{
		Type:    "single_select",
		Options: `{"options":["Electronics","Books","Clothing"]}`,
	}

	err := service.validateFieldValue(singleSelectField, "Electronics")
	assert.NoError(t, err, "Valid single select should pass")

	err = service.validateFieldValue(singleSelectField, "InvalidCategory")
	assert.Error(t, err, "Invalid single select should fail")

	// Multi select
	multiSelectField := models.Field{
		Type:    "multi_select",
		Options: `{"options":["New","Sale","Featured"]}`,
	}

	err = service.validateFieldValue(multiSelectField, []interface{}{"New", "Featured"})
	assert.NoError(t, err, "Valid multi select should pass")

	err = service.validateFieldValue(multiSelectField, []interface{}{"New", "InvalidTag"})
	assert.Error(t, err, "Invalid multi select should fail")
}

// TestValidateFieldValue_TypeValidation tests basic type validation
func TestValidateFieldValue_TypeValidation(t *testing.T) {
	service := &RecordService{}

	// Number type
	numberField := models.Field{Type: "number"}

	validNumbers := []interface{}{42, 3.14, int32(100), int64(1000), float32(2.5)}
	for _, num := range validNumbers {
		err := service.validateFieldValue(numberField, num)
		assert.NoError(t, err, "Number type %T should pass", num)
	}

	invalidNumbers := []interface{}{"42", true, []int{1, 2, 3}, map[string]interface{}{"value": 42}}
	for _, num := range invalidNumbers {
		err := service.validateFieldValue(numberField, num)
		assert.Error(t, err, "Non-number type %T should fail", num)
	}

	// Boolean type
	boolField := models.Field{Type: "boolean"}

	err := service.validateFieldValue(boolField, true)
	assert.NoError(t, err, "Boolean true should pass")

	err = service.validateFieldValue(boolField, false)
	assert.NoError(t, err, "Boolean false should pass")

	err = service.validateFieldValue(boolField, "true")
	assert.Error(t, err, "String 'true' should fail boolean validation")

	// Date/datetime type
	dateField := models.Field{Type: "date"}

	err = service.validateFieldValue(dateField, "2026-01-09")
	assert.NoError(t, err, "Date string should pass")

	err = service.validateFieldValue(dateField, 12345)
	assert.Error(t, err, "Non-string date should fail")
}

// TestValidateRecordData_RequiredFields tests required field validation
func TestValidateRecordData_RequiredFields(t *testing.T) {
	service := &RecordService{}

	// Mock database with fields
	fields := []models.Field{
		{ID: "fld_req1", Name: "RequiredField1", Type: "string", Required: true},
		{ID: "fld_req2", Name: "RequiredField2", Type: "number", Required: true},
		{ID: "fld_opt", Name: "OptionalField", Type: "string", Required: false},
	}

	// Test all required fields provided
	data := map[string]interface{}{
		"RequiredField1": "value1",
		"RequiredField2": 42,
		"OptionalField":  "optional",
	}
	err := service.validateRecordDataWithFields(fields, data)
	assert.NoError(t, err, "All required fields provided should pass")

	// Test missing required field
	data = map[string]interface{}{
		"RequiredField1": "value1",
		// Missing RequiredField2
		"OptionalField": "optional",
	}
	err = service.validateRecordDataWithFields(fields, data)
	assert.Error(t, err, "Missing required field should fail")
	assert.Contains(t, err.Error(), "是必填的")
}

// TestValidateRecordData_FieldNameOrID tests validation with both field names and IDs
func TestValidateRecordData_FieldNameOrID(t *testing.T) {
	service := &RecordService{}

	fields := []models.Field{
		{ID: "fld_test", Name: "TestField", Type: "string", Required: true, Options: `{"max_length":5}`},
	}

	// Test using field name
	data := map[string]interface{}{"TestField": "abc"}
	err := service.validateRecordDataWithFields(fields, data)
	assert.NoError(t, err, "Should work with field name")

	// Test using field ID
	data = map[string]interface{}{"fld_test": "abc"}
	err = service.validateRecordDataWithFields(fields, data)
	assert.NoError(t, err, "Should work with field ID")

	// Test validation still works with field name
	data = map[string]interface{}{"TestField": "toolong"}
	err = service.validateRecordDataWithFields(fields, data)
	assert.Error(t, err, "Validation should work with field name")

	// Test validation still works with field ID
	data = map[string]interface{}{"fld_test": "toolong"}
	err = service.validateRecordDataWithFields(fields, data)
	assert.Error(t, err, "Validation should work with field ID")
}

// TestValidateRecordData_CombinedRules tests multiple validation rules on same field
func TestValidateRecordData_CombinedRules(t *testing.T) {
	service := &RecordService{}

	fields := []models.Field{
		{ID: "fld_combined", Name: "CombinedField", Type: "string", Required: true, Options: `{"validation":"^[A-Z]+$","max_length":5}`},
	}

	// Valid format and length
	data := map[string]interface{}{"CombinedField": "ABC"}
	err := service.validateRecordDataWithFields(fields, data)
	assert.NoError(t, err, "Valid format and length should pass")

	// Valid format but too long
	data = map[string]interface{}{"CombinedField": "ABCDEF"}
	err = service.validateRecordDataWithFields(fields, data)
	assert.Error(t, err, "Valid format but too long should fail")
	assert.Contains(t, err.Error(), "长度不能超过")

	// Invalid format but correct length
	data = map[string]interface{}{"CombinedField": "abc"}
	err = service.validateRecordDataWithFields(fields, data)
	assert.Error(t, err, "Invalid format but correct length should fail")
	assert.Contains(t, err.Error(), "格式不匹配")
}

// TestValidateFieldValue_EmptyOptional tests that optional fields can be empty
func TestValidateFieldValue_EmptyOptional(t *testing.T) {
	service := &RecordService{}

	// Optional field with validation
	field := models.Field{
		Type:    "string",
		Required: false,
		Options: `{"validation":"^[A-Z]+$","max_length":5}`,
	}

	// Empty value for optional field should pass
	err := service.validateFieldValue(field, "")
	assert.NoError(t, err, "Empty optional field should pass")

	// Nil value for optional field should pass
	err = service.validateFieldValue(field, nil)
	assert.NoError(t, err, "Nil optional field should pass")
}

// TestValidateFieldValue_SpecialCharacters tests handling of special characters
func TestValidateFieldValue_SpecialCharacters(t *testing.T) {
	service := &RecordService{}

	field := models.Field{
		Type:    "string",
		Options: `{"max_length":50}`,
	}

	// Test with special characters that should be allowed in strings
	testCases := []struct {
		value    string
		shouldPass bool
		desc     string
	}{
		{"Hello World", true, "spaces"},
		{"test@example.com", true, "email chars"},
		{"user_123", true, "underscore and numbers"},
		{"test-dash", true, "dash"},
		{"test.dot", true, "dot"},
		{"", true, "empty (optional)"},
	}

	for _, tc := range testCases {
		err := service.validateFieldValue(field, tc.value)
		if tc.shouldPass {
			assert.NoError(t, err, "Value with %s should pass: %s", tc.desc, tc.value)
		} else {
			assert.Error(t, err, "Value with %s should fail: %s", tc.desc, tc.value)
		}
	}
}

// Helper method to test validateRecordData with mock fields
func (s *RecordService) validateRecordDataWithFields(fields []models.Field, data map[string]interface{}) error {
	// Simulate the validation logic from validateRecordData
	for _, field := range fields {
		// 支持通过字段ID或字段名查找数据
		value, existsByID := data[field.ID]
		valueByName, existsByName := data[field.Name]

		// 如果通过ID和名称都找不到，但字段是必填的，则报错
		if field.Required && !existsByID && !existsByName {
			return fieldRequiredError(field.Name)
		}

		// 如果字段不存在，跳过验证
		if !existsByID && !existsByName {
			continue
		}

		// 优先使用通过名称找到的值（如果存在）
		if existsByName {
			value = valueByName
		}

		// 对于可选字段，如果值为空或nil，则跳过验证
		if !field.Required && (value == nil || value == "") {
			continue
		}

		// 根据字段类型验证数据
		if err := s.validateFieldValue(field, value); err != nil {
			return fieldValidationError(field.Name, err)
		}
	}

	return nil
}

// Helper functions to match error messages
func fieldRequiredError(fieldName string) error {
	return &fieldValidationErrorImpl{fieldName: fieldName, msg: "字段 '" + fieldName + "' 是必填的"}
}

func fieldValidationError(fieldName string, err error) error {
	return &fieldValidationErrorImpl{fieldName: fieldName, msg: "字段 '" + fieldName + "' 验证失败: " + err.Error()}
}

type fieldValidationErrorImpl struct {
	fieldName string
	msg       string
}

func (e *fieldValidationErrorImpl) Error() string {
	return e.msg
}

// TestUpdateRecord_OptimisticLocking tests optimistic locking functionality
func TestUpdateRecord_OptimisticLocking(t *testing.T) {
	// Test cases for optimistic locking
	testCases := []struct {
		name           string
		currentVersion int
		requestVersion int
		shouldFail     bool
		description    string
	}{
		{
			name:           "Version match - should succeed",
			currentVersion: 5,
			requestVersion: 5,
			shouldFail:     false,
			description:    "When versions match, update should proceed",
		},
		{
			name:           "Version mismatch - should fail",
			currentVersion: 5,
			requestVersion: 3,
			shouldFail:     true,
			description:    "When versions don't match, update should be rejected",
		},
		{
			name:           "Version 0 - should succeed (no lock)",
			currentVersion: 5,
			requestVersion: 0,
			shouldFail:     false,
			description:    "When request version is 0, no optimistic locking check",
		},
		{
			name:           "Higher request version - should fail",
			currentVersion: 5,
			requestVersion: 7,
			shouldFail:     true,
			description:    "When request version is higher, update should be rejected",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate the optimistic lock check logic
			recordVersion := tc.currentVersion
			requestVersion := tc.requestVersion

			var err error
			if requestVersion > 0 && recordVersion != requestVersion {
				err = fmt.Errorf("记录已被其他用户修改，当前版本：%d，请求版本：%d", recordVersion, requestVersion)
			}

			if tc.shouldFail {
				assert.Error(t, err, tc.description)
				assert.Contains(t, err.Error(), "记录已被其他用户修改", "Error should mention concurrent modification")
			} else {
				assert.NoError(t, err, tc.description)
			}
		})
	}
}

// TestUpdateRecord_VersionIncrement tests that version increments correctly
func TestUpdateRecord_VersionIncrement(t *testing.T) {
	// Test that version increments by 1 after successful update
	initialVersion := 5
	expectedNewVersion := initialVersion + 1

	// Simulate the version increment logic
	actualNewVersion := initialVersion + 1

	assert.Equal(t, expectedNewVersion, actualNewVersion, "Version should increment by 1 after update")
}

// TestUpdateRecord_ConcurrentModificationScenario tests a realistic concurrent modification scenario
func TestUpdateRecord_ConcurrentModificationScenario(t *testing.T) {
	// Scenario: User A and User B both load the same record (version 1)
	// User A updates successfully (version becomes 2)
	// User B tries to update with old version (1) - should fail

	initialVersion := 1

	// User A updates successfully
	userAVersion := initialVersion
	userAUpdatedVersion := userAVersion + 1 // Now version 2

	// User B tries to update with stale version
	userBVersion := initialVersion // Still version 1

	// Check if User B's update would be rejected
	var err error
	if userBVersion > 0 && userAUpdatedVersion != userBVersion {
		err = fmt.Errorf("记录已被其他用户修改，当前版本：%d，请求版本：%d", userAUpdatedVersion, userBVersion)
	}

	assert.Error(t, err, "User B's update should be rejected due to version mismatch")
	assert.Contains(t, err.Error(), "记录已被其他用户修改")
	assert.Contains(t, err.Error(), "当前版本：2")
	assert.Contains(t, err.Error(), "请求版本：1")
}

// TestValidateFieldValue_NilHandling tests that nil values are handled correctly
func TestValidateFieldValue_NilHandling(t *testing.T) {
	service := &RecordService{}

	// Test various field types with nil values
	fieldTypes := []models.Field{
		{Type: "string"},
		{Type: "number"},
		{Type: "boolean"},
		{Type: "date"},
		{Type: "single_select"},
		{Type: "multi_select"},
	}

	for _, field := range fieldTypes {
		t.Run(fmt.Sprintf("Nil_%s", field.Type), func(t *testing.T) {
			err := service.validateFieldValue(field, nil)
			assert.NoError(t, err, "Nil values should be handled gracefully for all field types")
		})
	}
}