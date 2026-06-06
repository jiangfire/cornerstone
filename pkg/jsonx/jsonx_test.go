package jsonx

import (
	stdjson "encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalUnmarshal_RoundTrip(t *testing.T) {
	type payload struct {
		Name   string         `json:"name"`
		Status string         `json:"status"`
		Meta   map[string]any `json:"meta"`
	}

	input := payload{
		Name:   "alice",
		Status: "paid",
		Meta: map[string]any{
			"score": 42,
			"tags":  []string{"a", "b"},
		},
	}

	data, err := Marshal(input)
	require.NoError(t, err)
	assert.True(t, Valid(data))

	var output payload
	require.NoError(t, Unmarshal(data, &output))
	assert.Equal(t, input.Name, output.Name)
	assert.Equal(t, input.Status, output.Status)
	assert.Equal(t, float64(42), output.Meta["score"])
}

func TestTypeAliases_StdCompatibility(t *testing.T) {
	var raw RawMessage = []byte(`{"ok":true}`)
	assert.Equal(t, stdjson.RawMessage(raw), stdjson.RawMessage(`{"ok":true}`))

	var number Number = "42"
	assert.Equal(t, stdjson.Number("42"), stdjson.Number(number))
}

func TestMarshalStringUnmarshalString_RoundTrip(t *testing.T) {
	input := map[string]any{
		"name":   "alice",
		"status": "paid",
		"score":  42,
	}

	data, err := MarshalString(input)
	require.NoError(t, err)
	assert.True(t, Valid([]byte(data)))

	var output map[string]any
	require.NoError(t, UnmarshalString(data, &output))
	assert.Equal(t, "alice", output["name"])
	assert.Equal(t, "paid", output["status"])
	assert.Equal(t, float64(42), output["score"])
}
