package handlers

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/internal/middleware"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/jiangfire/cornerstone/pkg/db"
	"github.com/jiangfire/cornerstone/pkg/dto"
)

// CreateDatabase
//
// @Summary      Create a database
// @Description  Create a new database owned by the authenticated token.
//
//	The database name must be non-empty. The description field is optional.
//	The returned object contains the generated database ID and creation timestamp.
//
// @Tags         databases
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  dto.DatabaseCreateRequest  true  "Database to create"
// @Success      200  {object}  dto.APIResponse{data=dto.DatabaseObject}
// @Failure      400  {object}  dto.ErrorResponse  "Validation error - invalid request body"
// @Failure      401  {object}  dto.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  dto.ErrorResponse  "Forbidden - master token required"
// @Router       /api/v1/databases [post]
func CreateDatabase(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)

	var req services.CreateDBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "invalid request: "+err.Error())
		return
	}

	dbService := services.NewDatabaseService(db.DB())
	database, err := dbService.CreateDatabase(req, tokenID)
	if err != nil {
		handleCreateServiceError(c, err)
		return
	}

	dto.Success(c, dto.DatabaseObject{
		ID:          database.ID,
		Name:        database.Name,
		Description: database.Description,
	})
}

// ListDatabases
//
// @Summary      List all databases
// @Description  Returns all databases accessible to the authenticated token.
//
//	Master tokens see all databases. Client tokens see only databases they own.
//	Results are sorted by creation time (newest first).
//
// @Tags         databases
// @Produce      json
// @Security     ApiKeyAuth
// @Success      200  {object}  dto.APIResponse{data=dto.DatabaseListData}
// @Failure      401  {object}  dto.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      500  {object}  dto.ErrorResponse
// @Router       /api/v1/databases [get]
func ListDatabases(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)

	dbService := services.NewDatabaseService(db.DB())
	databases, err := dbService.ListDatabases(tokenID)
	if err != nil {
		dto.Error(c, 500, err.Error())
		return
	}

	items := make([]dto.DatabaseObject, len(databases))
	for i, d := range databases {
		items[i] = dto.DatabaseObject{ID: d.ID, Name: d.Name, Description: d.Description}
	}

	dto.Success(c, dto.DatabaseListData{Databases: items, Total: len(items)})
}

// GetDatabase
//
// @Summary      Get a database by ID
// @Description  Retrieve full details of a single database by its ID.
//
//	The authenticated token must own the database or be a Master token.
//	Returns 403 if the token does not have access.
//
// @Tags         databases
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Database ID"
// @Success      200  {object}  dto.APIResponse{data=dto.DatabaseObject}
// @Failure      401  {object}  dto.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  dto.ErrorResponse  "Forbidden - no access to this database"
// @Failure      404  {object}  dto.ErrorResponse  "Database not found"
// @Router       /api/v1/databases/{id} [get]
func GetDatabase(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	dbID := c.Param("id")

	dbService := services.NewDatabaseService(db.DB())
	database, err := dbService.GetDatabase(dbID, tokenID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	dto.Success(c, dto.DatabaseObject{
		ID:          database.ID,
		Name:        database.Name,
		Description: database.Description,
	})
}

// UpdateDatabase
//
// @Summary      Update a database
// @Description  Update database name and/or description.
//
//	The authenticated token must own the database or be a Master token.
//	The name field is required in the request body.
//
// @Tags         databases
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id    path  string                true  "Database ID"
// @Param        body  body  dto.DatabaseUpdateRequest  true  "Database update fields"
// @Success      200  {object}  dto.APIResponse{data=dto.DatabaseObject}
// @Failure      400  {object}  dto.ErrorResponse  "Validation error - invalid request body"
// @Failure      401  {object}  dto.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  dto.ErrorResponse  "Forbidden - no access to this database"
// @Failure      404  {object}  dto.ErrorResponse  "Database not found"
// @Router       /api/v1/databases/{id} [put]
func UpdateDatabase(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	dbID := c.Param("id")

	var req services.UpdateDBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "invalid request: "+err.Error())
		return
	}

	dbService := services.NewDatabaseService(db.DB())
	database, err := dbService.UpdateDatabase(dbID, req, tokenID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	dto.Success(c, dto.DatabaseObject{
		ID:          database.ID,
		Name:        database.Name,
		Description: database.Description,
	})
}

// DeleteDatabase
//
// @Summary      Delete a database
// @Description  Delete a database and all of its associated tables, fields, and records.
//
//	This action is irreversible. The authenticated token must own the database
//	or be a Master token.
//
// @Tags         databases
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Database ID"
// @Success      200  {object}  dto.APIResponse{data=object}
// @Failure      401  {object}  dto.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  dto.ErrorResponse  "Forbidden - no access to this database"
// @Failure      404  {object}  dto.ErrorResponse  "Database not found"
// @Router       /api/v1/databases/{id} [delete]
func DeleteDatabase(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	dbID := c.Param("id")

	dbService := services.NewDatabaseService(db.DB())
	if err := dbService.DeleteDatabase(dbID, tokenID); err != nil {
		handleServiceError(c, err)
		return
	}

	dto.Success(c, dto.MessageData{Message: "database deleted"})
}

// CreateDatabaseWithTables
//
// @Summary      Create a database with tables and fields
// @Description  Atomically create a database together with nested tables and fields in a single request.
//
//	This is a convenience endpoint that combines database, table, and field creation
//	into one transactional operation. If any part fails, the entire operation is rolled back.
//
//	Each table must have a name and may contain an array of field definitions.
//	Each field definition requires name and type; description and required are optional.
//
// @Tags         databases
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  dto.DatabaseBulkCreateRequest  true  "Database with nested tables and fields"
// @Success      200  {object}  dto.APIResponse{data=dto.BulkCreateData}
// @Failure      400  {object}  dto.ErrorResponse  "Validation error - invalid request body"
// @Failure      401  {object}  dto.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  dto.ErrorResponse  "Forbidden - master token required"
// @Router       /api/v1/databases/with-tables [post]
func CreateDatabaseWithTables(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)

	var req services.CreateDBWithTablesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "invalid request: "+err.Error())
		return
	}

	dbService := services.NewDatabaseService(db.DB())
	result, err := dbService.CreateDatabaseWithTables(req, tokenID)
	if err != nil {
		handleCreateServiceError(c, err)
		return
	}

	dto.Success(c, buildBulkCreateData(result))
}

// ImportDatabaseYAML imports a database from YAML
//
// @Summary      Import database from YAML
// @Description  Import a database definition from YAML format. Creates the database together with nested tables and fields.
//
//	The YAML structure matches the DatabaseBulkCreateRequest JSON schema.
//	Content-Type must be application/x-yaml or text/yaml.
//
// @Tags         databases
// @Accept       application/x-yaml
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  string  true  "YAML document"  example:"name: My App"
// @Success      200  {object}  dto.APIResponse{data=dto.BulkCreateData}
// @Failure      400  {object}  dto.ErrorResponse  "Validation error - invalid YAML or missing fields"
// @Failure      401  {object}  dto.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  dto.ErrorResponse  "Forbidden - master token required"
// @Router       /api/v1/databases/import/yaml [post]
func ImportDatabaseYAML(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		dto.Error(c, 400, "failed to read request body: "+err.Error())
		return
	}

	dbService := services.NewDatabaseService(db.DB())
	result, err := dbService.ImportYAML(body, tokenID)
	if err != nil {
		handleCreateServiceError(c, err)
		return
	}

	dto.Success(c, buildBulkCreateData(result))
}

// GetImportTemplate returns a YAML template for database import
//
// @Summary      Download import template
// @Description  Returns a commented YAML template with all available options for the import endpoint.
//
//	Can be downloaded as a file or used as a reference for creating import documents.
//
// @Tags         databases
// @Produce      application/x-yaml
// @Security     ApiKeyAuth
// @Success      200  {file}  binary  "YAML template file"
// @Failure      401  {object}  dto.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Router       /api/v1/databases/import/template [get]
func GetImportTemplate(c *gin.Context) {
	template := services.YAMLTemplate()
	c.Header("Content-Disposition", "attachment; filename=cornerstone-import-template.yaml")
	c.Data(http.StatusOK, "application/x-yaml", template)
}

func buildBulkCreateData(result *services.CreateDBWithTablesResult) dto.BulkCreateData {
	tables := make([]dto.TableObject, 0, len(result.Tables))
	for _, t := range result.Tables {
		tables = append(tables, dto.TableObject{
			ID:          t.ID,
			DatabaseID:  t.DatabaseID,
			Name:        t.Name,
			Description: t.Description,
		})
	}

	fields := make([]dto.FieldObject, 0, len(result.Fields))
	for _, f := range result.Fields {
		fields = append(fields, dto.FieldObject{
			ID:          f.ID,
			TableID:     f.TableID,
			Name:        f.Name,
			Type:        f.Type,
			Description: f.Description,
			Required:    f.Required,
			Options:     f.Options,
		})
	}

	data := dto.BulkCreateData{
		Database: dto.DatabaseObject{
			ID:          result.Database.ID,
			Name:        result.Database.Name,
			Description: result.Database.Description,
		},
		Tables: tables,
		Fields: fields,
	}
	data.Summary.TableCount = len(result.Tables)
	data.Summary.FieldCount = len(result.Fields)
	return data
}

func tableObjectFromResponse(t *services.TableResponse) dto.TableObject {
	return dto.TableObject{
		ID:          t.ID,
		DatabaseID:  t.DatabaseID,
		Name:        t.Name,
		Description: t.Description,
	}
}

func tableObjectFromModel(t *models.Table) dto.TableObject {
	return dto.TableObject{
		ID:          t.ID,
		DatabaseID:  t.DatabaseID,
		Name:        t.Name,
		Description: t.Description,
	}
}

func fieldObjectFromModel(f *models.Field) dto.FieldObject {
	return dto.FieldObject{
		ID:          f.ID,
		TableID:     f.TableID,
		Name:        f.Name,
		Type:        f.Type,
		Description: f.Description,
		Required:    f.Required,
		Options:     f.Options,
	}
}

func fieldObjectFromResponse(f *services.FieldResponse) dto.FieldObject {
	return dto.FieldObject{
		ID:          f.ID,
		TableID:     f.TableID,
		Name:        f.Name,
		Type:        f.Type,
		Description: f.Description,
		Required:    f.Required,
		Options:     f.Options,
	}
}

func fileObjectFromModel(f *models.File) dto.FileObject {
	return dto.FileObject{
		ID:         f.ID,
		RecordID:   f.RecordID,
		FieldID:    f.FieldID,
		FileName:   f.FileName,
		FileSize:   f.FileSize,
		FileType:   f.FileType,
		StorageURL: f.StorageURL,
	}
}
