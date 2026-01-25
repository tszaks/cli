package validation

import (
	"strings"
	"testing"
)

func TestValidateSessionID(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid session ID with date prefix and uuid",
			sessionID: "2026-01-25-f736da47-b2ca-4f86-bb32-a1bbe582e464",
			wantErr:   false,
		},
		{
			name:      "valid session ID with uuid only",
			sessionID: "f736da47-b2ca-4f86-bb32-a1bbe582e464",
			wantErr:   false,
		},
		{
			name:      "valid session ID with alphanumeric",
			sessionID: "session123",
			wantErr:   false,
		},
		{
			name:      "valid session ID with underscores",
			sessionID: "session_123_test",
			wantErr:   false,
		},
		{
			name:      "valid session ID with dots",
			sessionID: "session.123.test",
			wantErr:   false,
		},
		{
			name:      "empty session ID",
			sessionID: "",
			wantErr:   true,
			errMsg:    "session ID cannot be empty",
		},
		{
			name:      "session ID with forward slash",
			sessionID: "session/123",
			wantErr:   true,
			errMsg:    "contains path separators",
		},
		{
			name:      "session ID with backslash",
			sessionID: "session\\123",
			wantErr:   true,
			errMsg:    "contains path separators",
		},
		{
			name:      "session ID with multiple forward slashes",
			sessionID: "../../etc/passwd",
			wantErr:   true,
			errMsg:    "contains path separators",
		},
		{
			name:      "session ID with only forward slash",
			sessionID: "/",
			wantErr:   true,
			errMsg:    "contains path separators",
		},
		{
			name:      "session ID with only backslash",
			sessionID: "\\",
			wantErr:   true,
			errMsg:    "contains path separators",
		},
		{
			name:      "session ID with path traversal attempt",
			sessionID: "../sensitive-data",
			wantErr:   true,
			errMsg:    "contains path separators",
		},
		{
			name:      "session ID with absolute path",
			sessionID: "/etc/passwd",
			wantErr:   true,
			errMsg:    "contains path separators",
		},
		{
			name:      "session ID with Windows absolute path",
			sessionID: "C:\\Windows\\System32",
			wantErr:   true,
			errMsg:    "contains path separators",
		},
		{
			name:      "session ID with special characters allowed",
			sessionID: "session-2026.01.25_test",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSessionID(tt.sessionID)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateSessionID(%q) expected error containing %q, got nil", tt.sessionID, tt.errMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateSessionID(%q) error = %q, want error containing %q", tt.sessionID, err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("ValidateSessionID(%q) unexpected error: %v", tt.sessionID, err)
			}
		})
	}
}

func TestValidateToolUseID(t *testing.T) {
	tests := []struct {
		name      string
		toolUseID string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid uuid format",
			toolUseID: "f736da47-b2ca-4f86-bb32-a1bbe582e464",
			wantErr:   false,
		},
		{
			name:      "valid anthropic tool use id format",
			toolUseID: "toolu_abc123def456",
			wantErr:   false,
		},
		{
			name:      "valid alphanumeric only",
			toolUseID: "abc123DEF456",
			wantErr:   false,
		},
		{
			name:      "valid with underscores",
			toolUseID: "tool_use_id_123",
			wantErr:   false,
		},
		{
			name:      "valid with hyphens",
			toolUseID: "tool-use-id-123",
			wantErr:   false,
		},
		{
			name:      "valid mixed underscores and hyphens",
			toolUseID: "tool_use-id-123",
			wantErr:   false,
		},
		{
			name:      "empty tool use ID is allowed",
			toolUseID: "",
			wantErr:   false,
		},
		{
			name:      "tool use ID with space",
			toolUseID: "tool use id",
			wantErr:   true,
			errMsg:    "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:      "tool use ID with forward slash",
			toolUseID: "tool/use",
			wantErr:   true,
			errMsg:    "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:      "tool use ID with backslash",
			toolUseID: "tool\\use",
			wantErr:   true,
			errMsg:    "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:      "tool use ID with dot",
			toolUseID: "tool.use.id",
			wantErr:   true,
			errMsg:    "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:      "tool use ID with special characters",
			toolUseID: "tool@use!id",
			wantErr:   true,
			errMsg:    "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:      "tool use ID with path traversal",
			toolUseID: "../../../etc/passwd",
			wantErr:   true,
			errMsg:    "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:      "tool use ID with parentheses",
			toolUseID: "tool(use)id",
			wantErr:   true,
			errMsg:    "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:      "tool use ID with brackets",
			toolUseID: "tool[use]id",
			wantErr:   true,
			errMsg:    "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:      "tool use ID with braces",
			toolUseID: "tool{use}id",
			wantErr:   true,
			errMsg:    "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:      "tool use ID with unicode characters",
			toolUseID: "tool_用_id",
			wantErr:   true,
			errMsg:    "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:      "tool use ID with emoji",
			toolUseID: "tool🎉use",
			wantErr:   true,
			errMsg:    "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:      "tool use ID with newline",
			toolUseID: "tool\nuse",
			wantErr:   true,
			errMsg:    "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:      "tool use ID with tab",
			toolUseID: "tool\tuse",
			wantErr:   true,
			errMsg:    "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:      "tool use ID with null byte",
			toolUseID: "tool\x00use",
			wantErr:   true,
			errMsg:    "must be alphanumeric with underscores/hyphens only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateToolUseID(tt.toolUseID)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateToolUseID(%q) expected error containing %q, got nil", tt.toolUseID, tt.errMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateToolUseID(%q) error = %q, want error containing %q", tt.toolUseID, err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("ValidateToolUseID(%q) unexpected error: %v", tt.toolUseID, err)
			}
		})
	}
}

func TestValidateAgentID(t *testing.T) {
	tests := []struct {
		name    string
		agentID string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid agent ID alphanumeric",
			agentID: "agent123",
			wantErr: false,
		},
		{
			name:    "valid agent ID with underscores",
			agentID: "agent_test_123",
			wantErr: false,
		},
		{
			name:    "valid agent ID with hyphens",
			agentID: "agent-test-123",
			wantErr: false,
		},
		{
			name:    "valid agent ID mixed case",
			agentID: "AgentTest123",
			wantErr: false,
		},
		{
			name:    "valid agent ID uuid format",
			agentID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			wantErr: false,
		},
		{
			name:    "empty agent ID is allowed",
			agentID: "",
			wantErr: false,
		},
		{
			name:    "agent ID with space",
			agentID: "agent test",
			wantErr: true,
			errMsg:  "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:    "agent ID with forward slash",
			agentID: "agent/test",
			wantErr: true,
			errMsg:  "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:    "agent ID with backslash",
			agentID: "agent\\test",
			wantErr: true,
			errMsg:  "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:    "agent ID with dot",
			agentID: "agent.test",
			wantErr: true,
			errMsg:  "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:    "agent ID with special characters",
			agentID: "agent@test#id",
			wantErr: true,
			errMsg:  "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:    "agent ID with path traversal",
			agentID: "../../agent",
			wantErr: true,
			errMsg:  "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:    "agent ID with colon",
			agentID: "agent:test",
			wantErr: true,
			errMsg:  "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:    "agent ID with asterisk",
			agentID: "agent*test",
			wantErr: true,
			errMsg:  "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:    "agent ID with question mark",
			agentID: "agent?test",
			wantErr: true,
			errMsg:  "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:    "agent ID with pipe",
			agentID: "agent|test",
			wantErr: true,
			errMsg:  "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:    "agent ID with unicode",
			agentID: "agent_テスト",
			wantErr: true,
			errMsg:  "must be alphanumeric with underscores/hyphens only",
		},
		{
			name:    "agent ID with emoji",
			agentID: "agent🤖test",
			wantErr: true,
			errMsg:  "must be alphanumeric with underscores/hyphens only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAgentID(tt.agentID)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateAgentID(%q) expected error containing %q, got nil", tt.agentID, tt.errMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateAgentID(%q) error = %q, want error containing %q", tt.agentID, err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("ValidateAgentID(%q) unexpected error: %v", tt.agentID, err)
			}
		})
	}
}

// TestPathSafeRegexBehavior documents the exact regex behavior for security-critical validation
func TestPathSafeRegexBehavior(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Valid characters
		{name: "lowercase letters", input: "abcdefghijklmnopqrstuvwxyz", expected: true},
		{name: "uppercase letters", input: "ABCDEFGHIJKLMNOPQRSTUVWXYZ", expected: true},
		{name: "digits", input: "0123456789", expected: true},
		{name: "underscore", input: "_", expected: true},
		{name: "hyphen", input: "-", expected: true},
		{name: "mixed valid", input: "abc123_DEF-456", expected: true},

		// Invalid characters (security-relevant)
		{name: "dot", input: ".", expected: false},
		{name: "forward slash", input: "/", expected: false},
		{name: "backslash", input: "\\", expected: false},
		{name: "space", input: " ", expected: false},
		{name: "null byte", input: "\x00", expected: false},
		{name: "newline", input: "\n", expected: false},
		{name: "tab", input: "\t", expected: false},
		{name: "carriage return", input: "\r", expected: false},

		// Path traversal attempts
		{name: "parent directory", input: "..", expected: false},
		{name: "relative path unix", input: "../file", expected: false},
		{name: "relative path windows", input: "..\\file", expected: false},
		{name: "absolute path unix", input: "/etc/passwd", expected: false},
		{name: "absolute path windows", input: "C:\\Windows", expected: false},

		// Special characters that could be dangerous
		{name: "semicolon", input: ";", expected: false},
		{name: "ampersand", input: "&", expected: false},
		{name: "pipe", input: "|", expected: false},
		{name: "backtick", input: "`", expected: false},
		{name: "dollar sign", input: "$", expected: false},
		{name: "parentheses", input: "()", expected: false},
		{name: "square brackets", input: "[]", expected: false},
		{name: "curly braces", input: "{}", expected: false},
		{name: "angle brackets", input: "<>", expected: false},

		// Unicode (should be rejected)
		{name: "unicode characters", input: "用", expected: false},
		{name: "emoji", input: "🎉", expected: false},

		// Edge cases
		{name: "empty string should fail (requires ^ and $)", input: "", expected: false},
		{name: "only underscores", input: "___", expected: true},
		{name: "only hyphens", input: "---", expected: true},
		{name: "starts with hyphen", input: "-abc", expected: true},
		{name: "ends with hyphen", input: "abc-", expected: true},
		{name: "starts with underscore", input: "_abc", expected: true},
		{name: "ends with underscore", input: "abc_", expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := pathSafeRegex.MatchString(tt.input)
			if matched != tt.expected {
				t.Errorf("pathSafeRegex.MatchString(%q) = %v, want %v", tt.input, matched, tt.expected)
			}
		})
	}
}
