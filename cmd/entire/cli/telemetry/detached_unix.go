//go:build unix

package telemetry

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// spawnDetachedAnalytics spawns a detached subprocess to send analytics.
// On Unix, this uses process group detachment so the subprocess continues
// after the parent exits.
func spawnDetachedAnalytics(payloadJSON string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}

	//nolint:gosec // G204: payloadJSON is controlled internally, not user input
	cmd := exec.CommandContext(context.Background(), executable, "__send_analytics", payloadJSON)

	// Detach from parent process group so subprocess survives parent exit
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Don't hold the working directory
	cmd.Dir = "/"

	// Inherit environment (may be needed for network config)
	cmd.Env = os.Environ()

	// Don't capture stdout/stderr - let it go to /dev/null
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Start the process (non-blocking)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start subprocess: %w", err)
	}

	// Release the process so it can run independently
	//nolint:errcheck // Best effort - process should continue regardless
	_ = cmd.Process.Release()

	return nil
}
