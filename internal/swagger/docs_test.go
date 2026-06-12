package swagger

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/swaggo/swag"
)

func TestSwaggerDocRegistered(t *testing.T) {
	doc, err := swag.ReadDoc("swagger")
	require.NoError(t, err, "swagger doc should be registered with swag v1")
	require.NotEmpty(t, doc)

	var spec map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(doc), &spec))

	assert.Equal(t, "2.0", spec["swagger"])
	assert.Equal(t, "Cornerstone API", spec["info"].(map[string]interface{})["title"])
}
