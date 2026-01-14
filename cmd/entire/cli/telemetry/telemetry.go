package telemetry

import (
	"context"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/denisbrodbeck/machineid"
	"github.com/posthog/posthog-go"
	"github.com/spf13/cobra"
)

var (
	// PostHogAPIKey is set at build time for production
	PostHogAPIKey = "phc_development_key"
)

// Client defines the telemetry interface
type Client interface {
	TrackCommand(cmd *cobra.Command)
	Close()
}

// contextKey is used for storing the telemetry client in context
type contextKey struct{}

// WithClient returns a new context with the telemetry client attached
func WithClient(ctx context.Context, client Client) context.Context {
	return context.WithValue(ctx, contextKey{}, client)
}

// GetClient retrieves the telemetry client from context
//
//nolint:ireturn // Returns interface for NoOp/PostHog polymorphism
func GetClient(ctx context.Context) Client {
	if client, ok := ctx.Value(contextKey{}).(Client); ok {
		return client
	}
	return &NoOpClient{}
}

// NoOpClient is a no-op implementation for when telemetry is disabled
type NoOpClient struct{}

func (n *NoOpClient) TrackCommand(_ *cobra.Command) {}
func (n *NoOpClient) Close()                        {}

// PostHogClient is the real telemetry client
type PostHogClient struct {
	client     posthog.Client
	machineID  string
	cliVersion string
	mu         sync.RWMutex
}

// NewClient creates a new telemetry client based on opt-out settings
//
//nolint:ireturn // Returns interface for NoOp/PostHog polymorphism
func NewClient(version string) Client {
	if os.Getenv("ENTIRE_TELEMETRY_OPTOUT") != "" {
		return &NoOpClient{}
	}

	id, err := machineid.ProtectedID("entire-cli")
	if err != nil {
		return &NoOpClient{}
	}

	client, err := posthog.NewWithConfig(PostHogAPIKey, posthog.Config{
		Endpoint:     "https://us.i.posthog.com",
		DisableGeoIP: posthog.Ptr(true),
		DefaultEventProperties: posthog.NewProperties().
			Set("cli_version", version).
			Set("os", runtime.GOOS).
			Set("arch", runtime.GOARCH),
	})
	if err != nil {
		return &NoOpClient{}
	}

	return &PostHogClient{
		client:     client,
		machineID:  id,
		cliVersion: version,
	}
}

// TrackCommand records the command execution
func (p *PostHogClient) TrackCommand(cmd *cobra.Command) {
	if cmd == nil {
		return
	}

	// Skip hidden commands
	if cmd.Hidden {
		return
	}

	p.mu.RLock()
	id := p.machineID
	c := p.client
	p.mu.RUnlock()

	if c == nil {
		return
	}

	//nolint:errcheck // Best-effort telemetry, failures should not affect CLI
	_ = c.Enqueue(posthog.Capture{
		DistinctId: id,
		Event:      "cli_command_executed",
		Properties: posthog.NewProperties().
			Set("command", CommandString()),
	})
}

// CommandString returns the full command line for telemetry
func CommandString() string {
	return strings.Join(os.Args, " ")
}

// Close flushes pending events
func (p *PostHogClient) Close() {
	p.mu.RLock()
	c := p.client
	p.mu.RUnlock()

	if c != nil {
		_ = c.Close()
	}
}
