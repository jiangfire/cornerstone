//go:build (amd64 && go1.17 && !go1.27) || (arm64 && go1.20 && !go1.27)

package jsonx

import (
	stdjson "encoding/json"

	"github.com/bytedance/sonic"
)

type RawMessage = stdjson.RawMessage
type Number = stdjson.Number

func Marshal(v interface{}) ([]byte, error) {
	return sonic.Marshal(v)
}

func MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	return sonic.MarshalIndent(v, prefix, indent)
}

func Unmarshal(data []byte, v interface{}) error {
	return sonic.Unmarshal(data, v)
}

func MarshalString(v interface{}) (string, error) {
	return sonic.MarshalString(v)
}

func UnmarshalString(data string, v interface{}) error {
	return sonic.UnmarshalString(data, v)
}

func Valid(data []byte) bool {
	return sonic.Valid(data)
}
