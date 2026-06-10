package handlers

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/internal/middleware"
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
// @Param        body  body  swagger.DatabaseCreateRequest  true  "Database to create"
// @Success      200  {object}  swagger.APIResponse{data=swagger.DatabaseObject}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - invalid request body"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - master token required"
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

	dto.Success(c, gin.H{
		"id":          database.ID,
		"name":        database.Name,
		"description": database.Description,
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
// @Success      200  {object}  swagger.APIResponse{data=swagger.DatabaseListResponse}
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      500  {object}  swagger.ErrorResponse
// @Router       /api/v1/databases [get]
func ListDatabases(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)

	dbService := services.NewDatabaseService(db.DB())
	databases, err := dbService.ListDatabases(tokenID)
	if err != nil {
		dto.Error(c, 500, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"databases": databases,
		"total":     len(databases),
	})
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
// @Success      200  {object}  swagger.APIResponse{data=swagger.DatabaseObject}
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to this database"
// @Failure      404  {object}  swagger.ErrorResponse  "Database not found"
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

	dto.Success(c, database)
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
// @Param        body  body  swagger.DatabaseUpdateRequest  true  "Database update fields"
// @Success      200  {object}  swagger.APIResponse{data=swagger.DatabaseObject}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - invalid request body"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to this database"
// @Failure      404  {object}  swagger.ErrorResponse  "Database not found"
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

	dto.Success(c, gin.H{
		"id":          database.ID,
		"name":        database.Name,
		"description": database.Description,
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
// @Success      200  {object}  swagger.APIResponse{data=object}
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to this database"
// @Failure      404  {object}  swagger.ErrorResponse  "Database not found"
// @Router       /api/v1/databases/{id} [delete]
func DeleteDatabase(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	dbID := c.Param("id")

	dbService := services.NewDatabaseService(db.DB())
	if err := dbService.DeleteDatabase(dbID, tokenID); err != nil {
		handleServiceError(c, err)
		return
	}

	dto.Success(c, gin.H{
		"message": "database deleted",
	})
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
// @Param        body  body  swagger.DatabaseBulkCreateRequest  true  "Database with nested tables and fields"
// @Success      200  {object}  swagger.APIResponse{data=swagger.DatabaseBulkCreateResponse}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - invalid request body"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - master token required"
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

	dto.Success(c, gin.H{
		"database": result.Database,
		"tables":   result.Tables,
		"fields":   result.Fields,
		"summary": gin.H{
			"table_count": len(result.Tables),
			"field_count": len(result.Fields),
		},
	})
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
// @Accept       x-yaml
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  string  true  "YAML document"  example:"name: My App"
// @Success      200  {object}  swagger.APIResponse{data=swagger.DatabaseBulkCreateResponse}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - invalid YAML or missing fields"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - master token required"
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

	dto.Success(c, gin.H{
		"database": result.Database,
		"tables":   result.Tables,
		"fields":   result.Fields,
		"summary": gin.H{
			"table_count": len(result.Tables),
			"field_count": len(result.Fields),
		},
	})
}

// GetImportTemplate returns a YAML template for database import
//
// @Summary      Download import template
// @Description  Returns a commented YAML template with all available options for the import endpoint.
//
//	Can be downloaded as a file or used as a reference for creating import documents.
//
// @Tags         databases
// @Produce      x-yaml
// @Security     ApiKeyAuth
// @Success      200  {file}  binary  "YAML template file"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Router       /api/v1/databases/import/template [get]
func GetImportTemplate(c *gin.Context) {
	template := services.YAMLTemplate()
	c.Header("Content-Disposition", "attachment; filename=cornerstone-import-template.yaml")
	c.Data(http.StatusOK, "application/x-yaml", template)
}
