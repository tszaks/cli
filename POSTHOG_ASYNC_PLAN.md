# PostHog Async Analytics Implementation Plan

## Problem

PostHog Go SDK blocks on `client.Close()` waiting for events to flush. With the Europe endpoint from Tasmania/Australia, this adds ~1 second latency to every CLI command.

## Solution

Spawn a detached subprocess that sends analytics events independently. The parent CLI process exits immediately while the child process handles the HTTP request to PostHog.

## Architecture

```
CLI Command Execution
        │
        ├── Execute command logic
        │
        └── PersistentPostRun
                │
                └── TrackDetached()
                        │
                        ├── Serialize event to JSON
                        ├── Spawn detached subprocess (same binary)
                        └── Exit immediately (0ms latency)

                        Detached Subprocess (__send_analytics)
                                │
                                ├── Parse JSON payload
                                ├── Create PostHog client
                                ├── Enqueue event
                                └── client.Close() (blocks ~1s, but detached)
```

## Implementation Steps

### Step 1: Create analytics package

Create a new file `internal/analytics/tracker.go` (or appropriate path for your project structure):

```go
//go:build unix

package analytics

import (
    "encoding/json"
    "os"
    "os/exec"
    "syscall"
    "time"

    "github.com/posthog/posthog-go"
    "github.com/spf13/cobra"
)

const analyticsSubcmd = "__send_analytics"

var (
    apiKey   string
    endpoint string
)

// Init sets the PostHog configuration
func Init(key, ep string) {
    apiKey = key
    endpoint = ep
}

// TrackDetached sends an event via a detached subprocess (non-blocking)
func TrackDetached(event, distinctID string, properties map[string]any) {
    if apiKey == "" {
        return
    }

    payload, err := json.Marshal(map[string]any{
        "event":       event,
        "distinct_id": distinctID,
        "properties":  properties,
        "timestamp":   time.Now().UTC().Format(time.RFC3339),
    })
    if err != nil {
        return
    }

    exe, err := os.Executable()
    if err != nil {
        return
    }

    cmd := exec.Command(exe, analyticsSubcmd, string(payload))
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setpgid: true,
    }
    cmd.Stdin = nil
    cmd.Stdout = nil
    cmd.Stderr = nil
    cmd.Dir = "/"
    cmd.Env = os.Environ()

    _ = cmd.Start()
}

// RegisterCmd adds the hidden analytics subcommand to the root command
func RegisterCmd(rootCmd *cobra.Command) {
    rootCmd.AddCommand(&cobra.Command{
        Use:    analyticsSubcmd,
        Hidden: true,
        Args:   cobra.ExactArgs(1),
        Run:    runAnalyticsCmd,
    })
}

func runAnalyticsCmd(_ *cobra.Command, args []string) {
    var payload struct {
        Event      string         `json:"event"`
        DistinctID string         `json:"distinct_id"`
        Properties map[string]any `json:"properties"`
        Timestamp  string         `json:"timestamp"`
    }

    if err := json.Unmarshal([]byte(args[0]), &payload); err != nil {
        return
    }

    client, err := posthog.NewWithConfig(apiKey, posthog.Config{
        Endpoint: endpoint,
    })
    if err != nil {
        return
    }
    defer client.Close()

    ts, _ := time.Parse(time.RFC3339, payload.Timestamp)
    if ts.IsZero() {
        ts = time.Now()
    }

    client.Enqueue(posthog.Capture{
        DistinctId: payload.DistinctID,
        Event:      payload.Event,
        Properties: posthog.Properties(payload.Properties),
        Timestamp:  ts,
    })
}
```

### Step 2: Integrate with root command

In your root command file (e.g., `cmd/root.go`):

```go
package cmd

import (
    "os"

    "yourproject/internal/analytics"
    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "yourcli",
    Short: "Your CLI description",
    PersistentPostRun: func(cmd *cobra.Command, args []string) {
        // Skip tracking for the analytics subcommand itself
        if cmd.Name() == "__send_analytics" {
            return
        }

        analytics.TrackDetached("cli_command", getDistinctID(), map[string]any{
            "command": cmd.CommandPath(),
            "args":    len(args),
            // Add any other properties you want to track
        })
    },
}

func init() {
    // Initialize analytics
    analytics.Init(
        os.Getenv("POSTHOG_API_KEY"),      // or hardcode/config
        "https://eu.i.posthog.com",         // your endpoint
    )

    // Register the hidden analytics subcommand
    analytics.RegisterCmd(rootCmd)
}

func getDistinctID() string {
    // Return your user identifier
    // e.g., machine ID, config-stored UUID, etc.
    return "user-id"
}
```

### Step 3: Remove existing blocking PostHog code

Find and remove any existing PostHog tracking code that uses:
- `defer client.Close()`
- Direct `client.Enqueue()` calls in command handlers
- Any synchronous PostHog client usage

### Step 4: Test the implementation

1. Build the CLI:
   ```bash
   go build -o mycli .
   ```

2. Run a command and verify it exits immediately:
   ```bash
   time ./mycli some-command
   # Should complete in normal time without ~1s delay
   ```

3. Verify events arrive in PostHog dashboard (may take a few seconds)

4. Check for zombie processes:
   ```bash
   ps aux | grep __send_analytics
   # Should show briefly then disappear
   ```

## Configuration Options

### Environment-based opt-out

Add analytics opt-out support:

```go
func TrackDetached(event, distinctID string, properties map[string]any) {
    if apiKey == "" || os.Getenv("DO_NOT_TRACK") == "1" {
        return
    }
    // ... rest of function
}
```

### Debug mode

Add optional logging for debugging:

```go
func TrackDetached(event, distinctID string, properties map[string]any) {
    if os.Getenv("ANALYTICS_DEBUG") == "1" {
        fmt.Fprintf(os.Stderr, "[analytics] tracking: %s\n", event)
    }
    // ... rest of function
}
```

## Files to Create/Modify

- [ ] Create `internal/analytics/tracker.go` (new file)
- [ ] Modify `cmd/root.go` to integrate analytics
- [ ] Remove old PostHog tracking code (search for existing `posthog` imports)
- [ ] Update `.gitignore` if needed for any local config

## Verification Checklist

- [ ] CLI commands execute without latency delay
- [ ] Events appear in PostHog dashboard
- [ ] No zombie processes after CLI exits
- [ ] Works on both Linux and macOS
- [ ] Analytics opt-out via `DO_NOT_TRACK=1` works
- [ ] Hidden `__send_analytics` command doesn't appear in help
