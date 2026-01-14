package telemetry

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/cobra"
)

func TestNewClientOptOut(t *testing.T) {
	t.Setenv("ENTIRE_TELEMETRY_OPTOUT", "1")

	client := NewClient("1.0.0")

	if _, ok := client.(*NoOpClient); !ok {
		t.Error("ENTIRE_TELEMETRY_OPTOUT=1 should return NoOpClient")
	}
}

func TestNewClientOptOutWithAnyValue(t *testing.T) {
	t.Setenv("ENTIRE_TELEMETRY_OPTOUT", "yes")

	client := NewClient("1.0.0")

	if _, ok := client.(*NoOpClient); !ok {
		t.Error("ENTIRE_TELEMETRY_OPTOUT with any value should return NoOpClient")
	}
}

func TestNoOpClientMethods(_ *testing.T) {
	client := &NoOpClient{}

	// Should not panic
	client.TrackCommand(nil)
	client.TrackCommand(&cobra.Command{Use: "test"})
	client.Close()
}

func TestWithClientAndGetClient(t *testing.T) {
	ctx := context.Background()
	client := &NoOpClient{}

	ctx = WithClient(ctx, client)
	retrieved := GetClient(ctx)

	if _, ok := retrieved.(*NoOpClient); !ok {
		t.Error("GetClient should return the client set with WithClient")
	}
}

func TestGetClientReturnsNoOpWhenNotSet(t *testing.T) {
	ctx := context.Background()

	client := GetClient(ctx)

	if _, ok := client.(*NoOpClient); !ok {
		t.Error("GetClient should return NoOpClient when no client is set")
	}
}

func TestPostHogClientSkipsHiddenCommands(_ *testing.T) {
	client := &PostHogClient{
		machineID: "test-id",
	}

	hiddenCmd := &cobra.Command{
		Use:    "hidden",
		Hidden: true,
	}

	// Should not panic and should skip hidden commands
	client.TrackCommand(hiddenCmd)
}

func TestPostHogClientSkipsHelpCommand(_ *testing.T) {
	client := &PostHogClient{
		machineID: "test-id",
	}

	helpCmd := &cobra.Command{
		Use: "help",
	}

	// Should not panic and should skip help command
	client.TrackCommand(helpCmd)
}

func TestPostHogClientSkipsCompletionCommand(_ *testing.T) {
	client := &PostHogClient{
		machineID: "test-id",
	}

	completionCmd := &cobra.Command{
		Use: "completion",
	}

	// Should not panic and should skip completion command
	client.TrackCommand(completionCmd)
}

func TestPostHogClientSkipsNilCommand(_ *testing.T) {
	client := &PostHogClient{
		machineID: "test-id",
	}

	// Should not panic with nil command
	client.TrackCommand(nil)
}

func TestPostHogClientClose(_ *testing.T) {
	client := &PostHogClient{
		machineID: "test-id",
		// client is nil, should not panic
	}

	// Should not panic when internal client is nil
	client.Close()
}

func TestCommandStringReturnsFullCommand(t *testing.T) {
	// Save original os.Args
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	// Set test args
	os.Args = []string{"entire", "session", "list", "--all", "-n", "5"}

	result := CommandString()
	expected := "entire session list --all -n 5"

	if result != expected {
		t.Errorf("CommandString() = %q, want %q", result, expected)
	}
}
