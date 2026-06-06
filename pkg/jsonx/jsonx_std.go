//go:build !((amd64 && go1.17 && !go1.27) || (arm64 && go1.20 && !go1.27))

package jsonx

import stdjson "encoding/json"

type RawMessage = stdjson.RawMessage
type Number = stdjson.Number

func Marshal(v interface{}) ([]byte, error) {
	return stdjson.Marshal(v)
}

func MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	return stdjson.MarshalIndent(v, prefix, indent)
}

func Unmarshal(data []byte, v interface{}) error {
	return stdjson.Unmarshal(data, v)
}

func MarshalString(v interface{}) (string, error) {
	data, err := stdjson.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func UnmarshalString(data string, v interface{}) error {
	return stdjson.Unmarshal([]byte(data), v)
}

func Valid(data []byte) bool {
	return stdjson.Valid(data)
}
