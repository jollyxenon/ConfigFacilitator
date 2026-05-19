package jsonc

import (
	"encoding/json"

	tidwalljsonc "github.com/tidwall/jsonc"
)

// Strip removes JSONC comments while preserving line structure.
func Strip(src []byte) []byte {
	return tidwalljsonc.ToJSON(src)
}

// Unmarshal strips disposable JSONC comments before decoding JSON into v.
func Unmarshal(src []byte, v any) error {
	return json.Unmarshal(Strip(src), v)
}

// Normalize strips comments and re-encodes the provided JSON value.
func Normalize(src []byte, v any) ([]byte, error) {
	if v == nil {
		return Strip(src), nil
	}
	return json.Marshal(v)
}
