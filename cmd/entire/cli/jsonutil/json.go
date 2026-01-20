// Package jsonutil provides JSON utilities with consistent formatting.
package jsonutil

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// MarshalIndentWithNewline is like json.MarshalIndent but adds a trailing newline.
// This ensures JSON files have proper POSIX line endings.
func MarshalIndentWithNewline(v any, prefix, indent string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent(prefix, indent)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("encoding JSON: %w", err)
	}
	return buf.Bytes(), nil
}
