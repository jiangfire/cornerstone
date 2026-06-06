package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/models"
)

// ============================================================
// Helpers
// ============================================================

func setupFieldFullTestEnv(t *testing.T) (*FieldService, *gorm.DB, *models.Database, *models.Table, *models.Token) {
	db := setupTestDB(t)
	svc := NewFieldService(db)

	database := &models.Database{Name: "FieldTestDB"}
	require.NoError(t, db.Create(database).Error)

	table := &models.Table{DatabaseID: database.ID, Name: "users"}
	require.NoError(t, db.Create(table).Error)

	master := &models.Token{Name: "master", Token: "cs_master_field", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)

	return svc, db, database, table, master
}

// ============================================================
// CreateField - success with all types
// ============================================================

func TestCreateField_AllValidTypes(t *testing.T) {
	svc, _, _, table, master := setupFieldFullTestEnv(t)

	types := []string{"string", "number", "boolean", "date", "datetime", "json", "list"}
	for i, ft := range types {
		field, err := svc.CreateField(CreateFieldRequest{
			TableID: table.ID,
			Name:    fmt.Sprintf("field_%s", ft),
			Type:    ft,
		}, master.ID)
		require.NoErrorf(t, err, "type %s should be accepted", ft)
		assert.Equal(t, ft, field.Type)
		assert.NotEmpty(t, field.ID)
		_ = i
	}
}

// ============================================================
// CreateField - duplicate name
// ============================================================

func TestCreateField_DuplicateName(t *testing.T) {
	svc, _, _, table, master := setupFieldFullTestEnv(t)

	_, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "email",
		Type:    "string",
	}, master.ID)
	require.NoError(t, err)

	_, err = svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "email",
		Type:    "text",
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已存在同名字段")
}

// ============================================================
// CreateField - nonexistent table
// ============================================================

func TestCreateField_NonexistentTable(t *testing.T) {
	svc, _, _, _, master := setupFieldFullTestEnv(t)

	_, err := svc.CreateField(CreateFieldRequest{
		TableID: "tbl_nonexistent",
		Name:    "email",
		Type:    "string",
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "表不存在")
}

// ============================================================
// CreateField - invalid field type
// ============================================================

func TestCreateField_InvalidType(t *testing.T) {
	svc, _, _, table, master := setupFieldFullTestEnv(t)

	invalidTypes := []string{"integer", "float", "array", "object", "blob", "timestamp", ""}
	for _, ft := range invalidTypes {
		_, err := svc.CreateField(CreateFieldRequest{
			TableID: table.ID,
			Name:    "test_field",
			Type:    ft,
		}, master.ID)
		assert.Errorf(t, err, "type %q should be rejected", ft)
		assert.Contains(t, err.Error(), "字段类型验证失败")
	}
}

// ============================================================
// CreateField - name too long
// ============================================================

func TestCreateField_NameTooLong(t *testing.T) {
	svc, _, _, table, master := setupFieldFullTestEnv(t)

	_, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    strings.Repeat("a", 256),
		Type:    "string",
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "字段名称验证失败")
}

// ============================================================
// CreateField - with config (MaxLength, Validation regex)
// ============================================================

func TestCreateField_WithConfig(t *testing.T) {
	svc, _, _, table, master := setupFieldFullTestEnv(t)

	maxLen := 100
	field, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "username",
		Type:    "string",
		Config: FieldConfig{
			MaxLength:  &maxLen,
			Validation: `^[a-zA-Z0-9_]+$`,
		},
	}, master.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, field.ID)

	var config FieldConfig
	require.NoError(t, json.Unmarshal([]byte(field.Options), &config))
	assert.Equal(t, 100, *config.MaxLength)
	assert.Equal(t, `^[a-zA-Z0-9_]+$`, config.Validation)
}

// ============================================================
// ListFields - returns fields for a table
// ============================================================

func TestListFields_ReturnsFields(t *testing.T) {
	svc, _, _, table, master := setupFieldFullTestEnv(t)

	_, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "name",
		Type:    "string",
	}, master.ID)
	require.NoError(t, err)

	_, err = svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "age",
		Type:    "number",
	}, master.ID)
	require.NoError(t, err)

	fields, err := svc.ListFields(table.ID, master.ID)
	require.NoError(t, err)
	assert.Len(t, fields, 2)
	names := []string{fields[0].Name, fields[1].Name}
	assert.Contains(t, names, "name")
	assert.Contains(t, names, "age")
}

// ============================================================
// ListFields - nonexistent table
// ============================================================

func TestListFields_NonexistentTable(t *testing.T) {
	svc, _, _, _, master := setupFieldFullTestEnv(t)

	fields, err := svc.ListFields("tbl_nonexistent", master.ID)
	assert.Error(t, err)
	assert.Nil(t, fields)
	assert.Contains(t, err.Error(), "表不存在")
}

// ============================================================
// GetField - success
// ============================================================

func TestGetField_Success(t *testing.T) {
	svc, _, _, table, master := setupFieldFullTestEnv(t)

	created, err := svc.CreateField(CreateFieldRequest{
		TableID:  table.ID,
		Name:     "email",
		Type:     "string",
		Required: true,
	}, master.ID)
	require.NoError(t, err)

	field, err := svc.GetField(created.ID, master.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, field.ID)
	assert.Equal(t, "email", field.Name)
	assert.Equal(t, "string", field.Type)
	assert.True(t, field.Required)
}

// ============================================================
// GetField - nonexistent field
// ============================================================

func TestGetField_Nonexistent(t *testing.T) {
	svc, _, _, _, master := setupFieldFullTestEnv(t)

	_, err := svc.GetField("fld_nonexistent", master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "字段不存在")
}

// ============================================================
// UpdateField - success (name, type, required)
// ============================================================

func TestUpdateField_Success(t *testing.T) {
	svc, _, _, table, master := setupFieldFullTestEnv(t)

	created, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "email",
		Type:    "string",
	}, master.ID)
	require.NoError(t, err)

	updated, err := svc.UpdateField(created.ID, UpdateFieldRequest{
		Name:        "email_address",
		Type:        "text",
		Required:    true,
		Description: "updated desc",
	}, master.ID)
	require.NoError(t, err)
	assert.Equal(t, "email_address", updated.Name)
	assert.Equal(t, "text", updated.Type)
	assert.True(t, updated.Required)
	assert.Equal(t, "updated desc", updated.Description)
}

// ============================================================
// UpdateField - duplicate name
// ============================================================

func TestUpdateField_DuplicateName(t *testing.T) {
	svc, _, _, table, master := setupFieldFullTestEnv(t)

	_, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "name",
		Type:    "string",
	}, master.ID)
	require.NoError(t, err)

	f2, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "email",
		Type:    "string",
	}, master.ID)
	require.NoError(t, err)

	_, err = svc.UpdateField(f2.ID, UpdateFieldRequest{
		Name: "name",
		Type: "string",
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已存在同名字段")
}

// ============================================================
// UpdateField - nonexistent field
// ============================================================

func TestUpdateField_Nonexistent(t *testing.T) {
	svc, _, _, _, master := setupFieldFullTestEnv(t)

	_, err := svc.UpdateField("fld_nonexistent", UpdateFieldRequest{
		Name: "new_name",
		Type: "string",
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "字段不存在")
}

// ============================================================
// DeleteField - success (soft delete with __deleted__ suffix)
// ============================================================

func TestDeleteField_Success(t *testing.T) {
	svc, _, _, table, master := setupFieldFullTestEnv(t)

	created, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "email",
		Type:    "string",
	}, master.ID)
	require.NoError(t, err)

	err = svc.DeleteField(created.ID, master.ID)
	require.NoError(t, err)

	_, err = svc.GetField(created.ID, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "字段不存在")
}

func TestDeleteField_DeletedNameSuffix(t *testing.T) {
	svc, db, _, table, master := setupFieldFullTestEnv(t)

	created, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "email",
		Type:    "string",
	}, master.ID)
	require.NoError(t, err)

	expectedName := fmt.Sprintf("email__deleted__%s", created.ID)

	err = svc.DeleteField(created.ID, master.ID)
	require.NoError(t, err)

	var deleted models.Field
	require.NoError(t, db.Unscoped().Where("id = ?", created.ID).First(&deleted).Error)
	assert.Equal(t, expectedName, deleted.Name)
	assert.True(t, deleted.DeletedAt.Valid)
}

// ============================================================
// DeleteField - nonexistent errors
// ============================================================

func TestDeleteField_Nonexistent(t *testing.T) {
	svc, _, _, _, master := setupFieldFullTestEnv(t)

	err := svc.DeleteField("fld_nonexistent", master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "字段不存在")
}

// ============================================================
// CheckFieldPermission - master can access
// ============================================================

func TestCheckFieldPermission_MasterCanAccess(t *testing.T) {
	svc, _, _, table, master := setupFieldFullTestEnv(t)

	created, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "email",
		Type:    "string",
	}, master.ID)
	require.NoError(t, err)

	err = svc.CheckFieldPermission(master.ID, created.ID, "read")
	assert.NoError(t, err)
}

// ============================================================
// CheckFieldPermission - nonexistent field errors
// ============================================================

func TestCheckFieldPermission_NonexistentField(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFieldService(db)

	viewer := &models.Token{
		Name:     "viewer_noperm",
		Token:    "cs_viewer_noperm",
		IsMaster: false,
		Scopes:   `{"databases":{},"tables":{}}`,
	}
	require.NoError(t, db.Create(viewer).Error)

	err := svc.CheckFieldPermission(viewer.ID, "fld_nonexistent", "read")
	assert.Error(t, err)
}

// ============================================================
// validateFieldConfig - too many options (>100)
// ============================================================

func TestValidateFieldConfig_TooManyOptions(t *testing.T) {
	options := make([]string, 101)
	for i := range options {
		options[i] = fmt.Sprintf("option_%d", i)
	}

	err := validateFieldConfig(FieldConfig{Options: options})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "选项数量不能超过100个")
}

// ============================================================
// validateFieldConfig - option too long (>255)
// ============================================================

func TestValidateFieldConfig_OptionTooLong(t *testing.T) {
	err := validateFieldConfig(FieldConfig{
		Options: []string{strings.Repeat("a", 256)},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "选项值长度不能超过255个字符")
}

// ============================================================
// validateFieldConfig - min > max error
// ============================================================

func TestValidateFieldConfig_MinGreaterThanMax(t *testing.T) {
	cfgMin := 10.0
	cfgMax := 5.0

	err := validateFieldConfig(FieldConfig{
		Min: &cfgMin,
		Max: &cfgMax,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "最小值不能大于最大值")
}

// ============================================================
// validateFieldConfig - max_length < 1 error
// ============================================================

func TestValidateFieldConfig_MaxLengthLessThanOne(t *testing.T) {
	maxLen := 0

	err := validateFieldConfig(FieldConfig{
		MaxLength: &maxLen,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "最大长度必须大于0")
}

func TestValidateFieldConfig_NegativeMaxLength(t *testing.T) {
	maxLen := -1

	err := validateFieldConfig(FieldConfig{
		MaxLength: &maxLen,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "最大长度必须大于0")
}

// ============================================================
// validateFieldConfig - invalid regex error
// ============================================================

func TestValidateFieldConfig_InvalidRegex(t *testing.T) {
	err := validateFieldConfig(FieldConfig{
		Validation: "[invalid",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的正则表达式")
}

// ============================================================
// validateFieldConfig - too many allowed types (>50)
// ============================================================

func TestValidateFieldConfig_TooManyAllowedTypes(t *testing.T) {
	types := make([]string, 51)
	for i := range types {
		types[i] = fmt.Sprintf(".%s", string(rune('a'+i%26)))
	}

	err := validateFieldConfig(FieldConfig{
		AllowedTypes: types,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "允许的文件类型数量不能超过50个")
}

// ============================================================
// validateFieldConfig - negative max file size
// ============================================================

func TestValidateFieldConfig_NegativeMaxFileSize(t *testing.T) {
	err := validateFieldConfig(FieldConfig{
		MaxFileSizeMB: -1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "附件大小限制不能小于0")
}

// ============================================================
// validateFieldConfig - valid config passes
// ============================================================

func TestValidateFieldConfig_Valid(t *testing.T) {
	cfgMin := 1.0
	cfgMax := 100.0
	maxLen := 50

	err := validateFieldConfig(FieldConfig{
		Options:       []string{"a", "b"},
		Min:           &cfgMin,
		Max:           &cfgMax,
		MaxLength:     &maxLen,
		Validation:    `^[a-z]+$`,
		AllowedTypes:  []string{".pdf", "image/*"},
		MaxFileSizeMB: 10,
	})
	assert.NoError(t, err)
}

// ============================================================
// sanitizeFieldName - strips dangerous chars
// ============================================================

func TestSanitizeFieldName(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"strips angle brackets", `a<b>c`, "abc"},
		{"strips double quotes", `a"b"c`, "abc"},
		{"strips single quotes", "a'b'c", "abc"},
		{"trims whitespace", "  name  ", "name"},
		{"clean input unchanged", "my_field", "my_field"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeFieldName(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

// ============================================================
// sanitizeFieldConfig - strips dangerous chars from options
// ============================================================

func TestSanitizeFieldConfig(t *testing.T) {
	config := FieldConfig{
		Options:      []string{`<option>`, `"quoted"`, `'single'`, "  spaced  ", ""},
		Validation:   "  ^[a-z]+$  ",
		AllowedTypes: []string{"  .pdf  ", "", "  .doc  "},
	}

	result := sanitizeFieldConfig(config)

	assert.Equal(t, []string{"option", "quoted", "single", "spaced"}, result.Options)
	assert.Equal(t, "^[a-z]+$", result.Validation)
	assert.Equal(t, []string{".pdf", ".doc"}, result.AllowedTypes)
}

// ============================================================
// isAttachmentFieldType / supportsFieldOptions / isDeprecatedFieldType
// ============================================================

func TestIsAttachmentFieldType(t *testing.T) {
	assert.True(t, isAttachmentFieldType("file"))
	assert.False(t, isAttachmentFieldType("string"))
	assert.False(t, isAttachmentFieldType("list"))
}

func TestSupportsFieldOptions(t *testing.T) {
	assert.True(t, supportsFieldOptions("list"))
	assert.False(t, supportsFieldOptions("string"))
	assert.False(t, supportsFieldOptions("number"))
}

func TestIsDeprecatedFieldType(t *testing.T) {
	assert.False(t, isDeprecatedFieldType("string"))
	assert.False(t, isDeprecatedFieldType("text"))
	assert.False(t, isDeprecatedFieldType("anything"))
}

// ============================================================
// buildDeletedFieldName - truncates long names
// ============================================================

func TestBuildDeletedFieldName(t *testing.T) {
	t.Run("normal name", func(t *testing.T) {
		result := buildDeletedFieldName("email", "fld_123")
		assert.Equal(t, "email__deleted__fld_123", result)
	})

	t.Run("truncates long name", func(t *testing.T) {
		longName := strings.Repeat("x", 300)
		fieldID := "fld_1234567890"
		result := buildDeletedFieldName(longName, fieldID)
		assert.LessOrEqual(t, len(result), 255)
		assert.Contains(t, result, "__deleted__"+fieldID)
	})

	t.Run("exact boundary", func(t *testing.T) {
		suffix := "__deleted__fld_123"
		name := strings.Repeat("x", 255-len(suffix))
		result := buildDeletedFieldName(name, "fld_123")
		assert.Len(t, result, 255)
	})
}

// ============================================================
// ListFields - respects field-level permissions (skips fields user can't read)
// ============================================================

func TestListFields_RespectsFieldPermissions(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFieldService(db)

	database := &models.Database{Name: "PermTestDB"}
	require.NoError(t, db.Create(database).Error)

	table := &models.Table{DatabaseID: database.ID, Name: "perm_table"}
	require.NoError(t, db.Create(table).Error)

	f1, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "visible_field",
		Type:    "string",
	}, "user1")
	require.NoError(t, err)

	f2, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "hidden_field",
		Type:    "number",
	}, "user1")
	require.NoError(t, err)

	t.Run("CheckFieldPermission with explicit field grant and low table role", func(t *testing.T) {
		scopes := fmt.Sprintf(
			`{"databases":{},"tables":{"%s":{"role":"","fields":{"%s":["read"]}}}}`,
			table.ID, f1.ID,
		)
		restricted := &models.Token{
			Name:     "restricted",
			Token:    "cs_restricted",
			IsMaster: false,
			Scopes:   scopes,
		}
		require.NoError(t, db.Create(restricted).Error)

		err := svc.CheckFieldPermission(restricted.ID, f1.ID, "read")
		assert.NoError(t, err, "field with explicit grant should be accessible")

		err = svc.CheckFieldPermission(restricted.ID, f2.ID, "read")
		assert.Error(t, err, "field without grant should not be accessible")
	})

	t.Run("master sees all fields in ListFields", func(t *testing.T) {
		fields, err := svc.ListFields(table.ID, "user1")
		require.NoError(t, err)
		assert.Len(t, fields, 2)
	})
}

// ============================================================
// validateFieldName additional coverage
// ============================================================

func TestValidateFieldName_Valid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"single char", "a"},
		{"underscore", "my_field"},
		{"unicode", "字段"},
		{"with digits", "field1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, validateFieldName(tt.input))
		})
	}
}

func TestValidateFieldName_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		substr string
	}{
		{"empty", "", "1-255"},
		{"too long", strings.Repeat("x", 256), "1-255"},
		{"starts with digit", "1field", "不能以数字开头"},
		{"spaces", "my field", "只能包含字母"},
		{"special chars", "field!", "只能包含字母"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFieldName(tt.input)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.substr)
		})
	}
}

// ============================================================
// validateFieldDescription
// ============================================================

func TestValidateFieldDescription(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, validateFieldDescription("normal description"))
	})

	t.Run("too long", func(t *testing.T) {
		err := validateFieldDescription(strings.Repeat("x", 1001))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "1000")
	})
}

// ============================================================
// normalizeFieldType
// ============================================================

func TestNormalizeFieldType(t *testing.T) {
	assert.Equal(t, "string", normalizeFieldType("string"))
	assert.Equal(t, "list", normalizeFieldType("list"))
}

// ============================================================
// validateMutableFieldType
// ============================================================

func TestValidateMutableFieldType(t *testing.T) {
	assert.NoError(t, validateMutableFieldType("string"))
	assert.NoError(t, validateMutableFieldType("list"))
}

// ============================================================
// containsRole
// ============================================================

func TestContainsRole(t *testing.T) {
	assert.True(t, containsRole([]string{"owner", "admin"}, "owner"))
	assert.True(t, containsRole([]string{"owner", "admin"}, "Owner"))
	assert.False(t, containsRole([]string{"owner", "admin"}, "editor"))
	assert.False(t, containsRole([]string{}, "owner"))
}

func TestRequiredActionForRoles(t *testing.T) {
	assert.Equal(t, authz.ActionRead, requiredActionForRoles([]string{"owner", "admin", "editor", "viewer"}))
	assert.Equal(t, authz.ActionWrite, requiredActionForRoles([]string{"owner", "admin", "editor"}))
	assert.Equal(t, authz.ActionManage, requiredActionForRoles([]string{"owner", "admin"}))
	assert.Equal(t, authz.ActionRead, requiredActionForRoles(nil))
}

// ============================================================
// CreateField with Options string for list type
// ============================================================

func TestCreateField_ListOptionsFromString(t *testing.T) {
	svc, _, _, table, master := setupFieldFullTestEnv(t)

	field, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "status",
		Type:    "list",
		Options: "active, inactive, pending",
	}, master.ID)
	require.NoError(t, err)

	var config FieldConfig
	require.NoError(t, json.Unmarshal([]byte(field.Options), &config))
	assert.Contains(t, config.Options, "active")
	assert.Contains(t, config.Options, "inactive")
	assert.Contains(t, config.Options, "pending")
}

// ============================================================
// UpdateField with config
// ============================================================

func TestUpdateField_WithConfig(t *testing.T) {
	svc, _, _, table, master := setupFieldFullTestEnv(t)

	created, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "score",
		Type:    "number",
	}, master.ID)
	require.NoError(t, err)

	cfgMin := 0.0
	cfgMax := 100.0
	updated, err := svc.UpdateField(created.ID, UpdateFieldRequest{
		Name:     "score",
		Type:     "number",
		Required: true,
		Config: FieldConfig{
			Min: &cfgMin,
			Max: &cfgMax,
		},
	}, master.ID)
	require.NoError(t, err)
	assert.True(t, updated.Required)

	var config FieldConfig
	require.NoError(t, json.Unmarshal([]byte(updated.Options), &config))
	assert.Equal(t, 0.0, *config.Min)
	assert.Equal(t, 100.0, *config.Max)
}

// ============================================================
// CreateField - with required flag
// ============================================================

func TestCreateField_RequiredFlag(t *testing.T) {
	svc, _, _, table, master := setupFieldFullTestEnv(t)

	field, err := svc.CreateField(CreateFieldRequest{
		TableID:  table.ID,
		Name:     "mandatory",
		Type:     "string",
		Required: true,
	}, master.ID)
	require.NoError(t, err)
	assert.True(t, field.Required)

	field2, err := svc.CreateField(CreateFieldRequest{
		TableID:  table.ID,
		Name:     "optional",
		Type:     "string",
		Required: false,
	}, master.ID)
	require.NoError(t, err)
	assert.False(t, field2.Required)
}

// ============================================================
// CreateField same name in different tables
// ============================================================

func TestCreateField_SameNameDifferentTables(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFieldService(db)

	database := &models.Database{Name: "MultiTableDB"}
	require.NoError(t, db.Create(database).Error)

	table1 := &models.Table{DatabaseID: database.ID, Name: "users"}
	require.NoError(t, db.Create(table1).Error)

	table2 := &models.Table{DatabaseID: database.ID, Name: "products"}
	require.NoError(t, db.Create(table2).Error)

	master := &models.Token{Name: "master", Token: "cs_master_multitable", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)

	f1, err := svc.CreateField(CreateFieldRequest{
		TableID: table1.ID,
		Name:    "name",
		Type:    "string",
	}, master.ID)
	require.NoError(t, err)

	f2, err := svc.CreateField(CreateFieldRequest{
		TableID: table2.ID,
		Name:    "name",
		Type:    "string",
	}, master.ID)
	require.NoError(t, err)

	assert.NotEqual(t, f1.ID, f2.ID)
}

// ============================================================
// CheckFieldPermission - master can write
// ============================================================

func TestCheckFieldPermission_MasterCanWrite(t *testing.T) {
	svc, _, _, table, master := setupFieldFullTestEnv(t)

	created, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "email",
		Type:    "string",
	}, master.ID)
	require.NoError(t, err)

	err = svc.CheckFieldPermission(master.ID, created.ID, "write")
	assert.NoError(t, err)

	err = svc.CheckFieldPermission(master.ID, created.ID, "manage")
	assert.NoError(t, err)
}

// ============================================================
// ListFields - returns empty for table with no fields
// ============================================================

func TestListFields_EmptyTable(t *testing.T) {
	svc, _, _, table, master := setupFieldFullTestEnv(t)

	fields, err := svc.ListFields(table.ID, master.ID)
	require.NoError(t, err)
	assert.Empty(t, fields)
}

// ============================================================
// sanitizeFieldDescription
// ============================================================

func TestSanitizeFieldDescription(t *testing.T) {
	assert.Equal(t, "hello", sanitizeFieldDescription("  hello  "))
	assert.Equal(t, "hello", sanitizeFieldDescription("hello"))
}

// ============================================================
// validateFieldConfig - allowed type too long (>100 chars)
// ============================================================

func TestValidateFieldConfig_AllowedTypeTooLong(t *testing.T) {
	longType := strings.Repeat("x", 101)

	err := validateFieldConfig(FieldConfig{
		AllowedTypes: []string{longType},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "文件类型规则长度不能超过100个字符")
}

// ============================================================
// validateFieldConfig - valid allowed types
// ============================================================

func TestValidateFieldConfig_ValidAllowedTypes(t *testing.T) {
	err := validateFieldConfig(FieldConfig{
		AllowedTypes: []string{".pdf", "image/*", ".docx"},
	})
	assert.NoError(t, err)
}

// ============================================================
// validateFieldConfig - zero max file size is valid
// ============================================================

func TestValidateFieldConfig_ZeroMaxFileSize(t *testing.T) {
	err := validateFieldConfig(FieldConfig{
		MaxFileSizeMB: 0,
	})
	assert.NoError(t, err)
}

// ============================================================
// validateFieldConfig - empty validation string is ok
// ============================================================

func TestValidateFieldConfig_EmptyValidation(t *testing.T) {
	err := validateFieldConfig(FieldConfig{
		Validation: "",
	})
	assert.NoError(t, err)
}

// ============================================================
// validateFieldConfig - min without max is ok
// ============================================================

func TestValidateFieldConfig_MinOnly(t *testing.T) {
	cfgMin := 5.0
	err := validateFieldConfig(FieldConfig{Min: &cfgMin})
	assert.NoError(t, err)
}

// ============================================================
// validateFieldConfig - max without min is ok
// ============================================================

func TestValidateFieldConfig_MaxOnly(t *testing.T) {
	cfgMax := 100.0
	err := validateFieldConfig(FieldConfig{Max: &cfgMax})
	assert.NoError(t, err)
}

// ============================================================
// validateFieldConfig - min equals max is ok
// ============================================================

func TestValidateFieldConfig_MinEqualsMax(t *testing.T) {
	val := 50.0
	err := validateFieldConfig(FieldConfig{Min: &val, Max: &val})
	assert.NoError(t, err)
}

// ============================================================
// GetField - response format
// ============================================================

func TestGetField_ResponseBodyFormat(t *testing.T) {
	svc, _, _, table, master := setupFieldFullTestEnv(t)

	created, err := svc.CreateField(CreateFieldRequest{
		TableID:  table.ID,
		Name:     "email",
		Type:     "string",
		Required: true,
	}, master.ID)
	require.NoError(t, err)

	resp, err := svc.GetField(created.ID, master.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.CreatedAt)
	assert.NotEmpty(t, resp.UpdatedAt)
	assert.Equal(t, table.ID, resp.TableID)
	assert.False(t, resp.Deprecated)
}

// ============================================================
// DeleteField - no access
// ============================================================

func TestDeleteField_NoAccess(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFieldService(db)

	database := &models.Database{Name: "DelAccessDB"}
	require.NoError(t, db.Create(database).Error)

	table := &models.Table{DatabaseID: database.ID, Name: "del_table"}
	require.NoError(t, db.Create(table).Error)

	created, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "email",
		Type:    "string",
	}, "user1")
	require.NoError(t, err)

	viewer := &models.Token{
		Name:     "viewer",
		Token:    "cs_viewer_del",
		IsMaster: false,
		Scopes:   fmt.Sprintf(`{"databases":{"%s":"viewer"},"tables":{}}`, database.ID),
	}
	require.NoError(t, db.Create(viewer).Error)

	err = svc.DeleteField(created.ID, viewer.ID)
	assert.Error(t, err)
}
