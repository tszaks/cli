package redact

import (
	"bytes"
	"strings"
	"testing"
)

// highEntropySecret is a string with Shannon entropy > 4.5 that will trigger redaction.
const highEntropySecret = "sk-ant-api03-xK9mZ2vL8nQ5rT1wY4bC7dF0gH3jE6pA"

func TestBytes_NoSecrets(t *testing.T) {
	input := []byte("hello world, this is normal text")
	result := Bytes(input)
	if string(result) != string(input) {
		t.Errorf("expected unchanged input, got %q", result)
	}
	// Should return the original slice when no changes
	if &result[0] != &input[0] {
		t.Error("expected same underlying slice when no redaction needed")
	}
}

func TestBytes_WithSecret(t *testing.T) {
	input := []byte("my key is " + highEntropySecret + " ok")
	result := Bytes(input)
	if strings.Contains(string(result), highEntropySecret) {
		t.Error("expected secret to be redacted")
	}
	if !strings.Contains(string(result), "[REDACTED]") {
		t.Error("expected [REDACTED] placeholder")
	}
}

func TestJSONLBytes_NoSecrets(t *testing.T) {
	input := []byte(`{"type":"text","content":"hello"}`)
	result, err := JSONLBytes(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != string(input) {
		t.Errorf("expected unchanged input, got %q", result)
	}
	if &result[0] != &input[0] {
		t.Error("expected same underlying slice when no redaction needed")
	}
}

func TestJSONLBytes_WithSecret(t *testing.T) {
	input := []byte(`{"type":"text","content":"key=` + highEntropySecret + `"}`)
	result, err := JSONLBytes(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(result), highEntropySecret) {
		t.Error("expected secret to be redacted in JSONL content")
	}
	if !bytes.Equal(result, []byte(`{"type":"text","content":"[REDACTED]"}`)) {
		t.Error("expected [REDACTED] placeholder in JSONL content")
	}
}

func TestJSONLContent_TopLevelArray(t *testing.T) {
	// Top-level JSON arrays are valid JSONL and should be redacted.
	input := `["` + highEntropySecret + `","normal text"]`
	result, err := JSONLContent(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, highEntropySecret) {
		t.Error("expected secret in top-level array to be redacted")
	}
	if result != `["[REDACTED]","normal text"]` {
		t.Error("expected [REDACTED] placeholder in top-level array")
	}
}

func TestJSONLContent_TopLevelArrayNoSecrets(t *testing.T) {
	input := `["hello","world"]`
	result, err := JSONLContent(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != input {
		t.Errorf("expected unchanged input, got %q", result)
	}
}

func TestCollectJSONLReplacements_Succeeds(t *testing.T) {
	obj := map[string]any{
		"content": "token=" + highEntropySecret,
	}
	repls := collectJSONLReplacements(obj)
	if len(repls) != 1 {
		t.Fatal("expected one replacement for high-entropy secret")
	}
	if !strings.Contains(repls[0][1], "[REDACTED]") {
		t.Errorf("expected replacement to contain [REDACTED], got %q", repls[0][1])
	}
}

func TestShouldSkipJSONLField(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		// Fields ending in "id" should be skipped.
		{"id", true},
		{"session_id", true},
		{"sessionId", true},
		{"checkpoint_id", true},
		{"checkpointID", true},
		{"userId", true},
		// Fields ending in "ids" should be skipped.
		{"ids", true},
		{"session_ids", true},
		{"userIds", true},
		// Exact match "signature" should be skipped.
		{"signature", true},
		// Fields that should NOT be skipped.
		{"content", false},
		{"type", false},
		{"name", false},
		{"video", false},      // ends in "o", not "id"
		{"identify", false},   // ends in "ify", not "id"
		{"signatures", false}, // not exact match "signature"
		{"signal_data", false},
		{"consideration", false}, // contains "id" but doesn't end with it
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := shouldSkipJSONLField(tt.key)
			if got != tt.want {
				t.Errorf("shouldSkipJSONLField(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestShouldSkipJSONLField_RedactionBehavior(t *testing.T) {
	// Verify that secrets in skipped fields are preserved (not redacted).
	obj := map[string]any{
		"session_id": highEntropySecret,
		"content":    highEntropySecret,
	}
	repls := collectJSONLReplacements(obj)
	// Only "content" should produce a replacement; "session_id" should be skipped.
	if len(repls) != 1 {
		t.Fatalf("expected 1 replacement, got %d", len(repls))
	}
	if repls[0][0] != highEntropySecret {
		t.Errorf("expected replacement for secret in content field, got %q", repls[0][0])
	}
}

func TestShouldSkipJSONLObject(t *testing.T) {
	tests := []struct {
		name string
		obj  map[string]any
		want bool
	}{
		{
			name: "image type is skipped",
			obj:  map[string]any{"type": "image", "data": "base64data"},
			want: true,
		},
		{
			name: "text type is not skipped",
			obj:  map[string]any{"type": "text", "content": "hello"},
			want: false,
		},
		{
			name: "no type field is not skipped",
			obj:  map[string]any{"content": "hello"},
			want: false,
		},
		{
			name: "non-string type is not skipped",
			obj:  map[string]any{"type": 42},
			want: false,
		},
		{
			name: "image_url type is not skipped",
			obj:  map[string]any{"type": "image_url"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipJSONLObject(tt.obj)
			if got != tt.want {
				t.Errorf("shouldSkipJSONLObject(%v) = %v, want %v", tt.obj, got, tt.want)
			}
		})
	}
}

func TestShouldSkipJSONLObject_RedactionBehavior(t *testing.T) {
	// Verify that secrets inside image objects are NOT redacted.
	obj := map[string]any{
		"type": "image",
		"data": highEntropySecret,
	}
	repls := collectJSONLReplacements(obj)
	if len(repls) != 0 {
		t.Errorf("expected no replacements for image object, got %d", len(repls))
	}

	// Verify that secrets inside non-image objects ARE redacted.
	obj2 := map[string]any{
		"type":    "text",
		"content": highEntropySecret,
	}
	repls2 := collectJSONLReplacements(obj2)
	if len(repls2) == 0 {
		t.Error("expected replacements for text object with secret")
	}
}
