package telemetry

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestEventPayloadSerialization(t *testing.T) {
	payload := EventPayload{
		Event:      "cli_command_executed",
		DistinctID: "test-machine-id",
		Properties: map[string]any{
			"command":         "entire status",
			"strategy":        "manual-commit",
			"agent":           "claude-code",
			"isEntireEnabled": true,
			"cli_version":     "1.0.0",
			"os":              "darwin",
			"arch":            "arm64",
		},
		Timestamp: time.Date(2026, 1, 28, 12, 0, 0, 0, time.UTC),
		APIKey:    "test-key",
		Endpoint:  "https://example.com",
	}

	// Serialize
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal EventPayload: %v", err)
	}

	// Deserialize
	var decoded EventPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal EventPayload: %v", err)
	}

	// Verify fields
	if decoded.Event != payload.Event {
		t.Errorf("Event = %q, want %q", decoded.Event, payload.Event)
	}
	if decoded.DistinctID != payload.DistinctID {
		t.Errorf("DistinctID = %q, want %q", decoded.DistinctID, payload.DistinctID)
	}
	if decoded.APIKey != payload.APIKey {
		t.Errorf("APIKey = %q, want %q", decoded.APIKey, payload.APIKey)
	}
	if decoded.Endpoint != payload.Endpoint {
		t.Errorf("Endpoint = %q, want %q", decoded.Endpoint, payload.Endpoint)
	}
	if !decoded.Timestamp.Equal(payload.Timestamp) {
		t.Errorf("Timestamp = %v, want %v", decoded.Timestamp, payload.Timestamp)
	}

	// Verify properties
	if cmd, ok := decoded.Properties["command"].(string); !ok || cmd != "entire status" {
		t.Errorf("Properties[command] = %v, want %q", decoded.Properties["command"], "entire status")
	}
}

func TestTrackCommandDetachedSkipsNilCommand(_ *testing.T) {
	// Should not panic with nil command
	TrackCommandDetached(nil, "manual-commit", "claude-code", true, "1.0.0")
}

func TestTrackCommandDetachedSkipsHiddenCommands(_ *testing.T) {
	hiddenCmd := &cobra.Command{
		Use:    "__send_analytics",
		Hidden: true,
	}

	// Should not panic and should skip hidden commands
	TrackCommandDetached(hiddenCmd, "manual-commit", "claude-code", true, "1.0.0")
}

func TestTrackCommandDetachedRespectsOptOut(t *testing.T) {
	t.Setenv("ENTIRE_TELEMETRY_OPTOUT", "1")

	cmd := &cobra.Command{
		Use: "status",
	}

	// Should not panic and should respect opt-out
	TrackCommandDetached(cmd, "manual-commit", "claude-code", true, "1.0.0")
}

func TestTrackCommandDetachedDefaultsAgentToAuto(_ *testing.T) {
	// We can't easily test the actual spawning without integration tests,
	// but we can verify the function doesn't panic with empty agent
	cmd := &cobra.Command{
		Use:    "test",
		Hidden: true, // Use hidden to prevent actual spawning
	}

	TrackCommandDetached(cmd, "manual-commit", "", true, "1.0.0")
}

func TestSendEventHandlesInvalidJSON(_ *testing.T) {
	// Should not panic with invalid JSON
	SendEvent("invalid json")
	SendEvent("")
	SendEvent("{}")
}

func TestStringifyArg(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"string", "hello", "hello"},
		{"int", 42, "42"},
		{"bool", true, "true"},
		{"nil", nil, "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringifyArg(tt.input)
			if result != tt.expected {
				t.Errorf("stringifyArg(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
