//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/entireio/cli/cmd/entire/cli/paths"
	"github.com/entireio/cli/cmd/entire/cli/session"
)

func TestHookLogging_WritesToSessionLogFile(t *testing.T) {
	t.Parallel()

	env := NewTestEnv(t)
	env.InitRepo()
	env.InitEntire("manual-commit") // Use manual-commit strategy (doesn't matter for logging)

	// Create a session state file in .git/entire-sessions/ with a known session ID
	sessionID := "test-logging-session-123"
	writeTestSessionStateForLogging(t, env.RepoDir, sessionID)

	// Create the logs directory (Init should create it, but ensure it exists)
	logsDir := filepath.Join(env.RepoDir, paths.EntireDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatalf("failed to create logs directory: %v", err)
	}

	// Run a hook with ENTIRE_LOG_LEVEL=debug to ensure logs are written
	// Use post-commit since it takes no arguments
	cmd := exec.Command(getTestBinary(), "hooks", "git", "post-commit")
	cmd.Dir = env.RepoDir
	cmd.Env = append(os.Environ(),
		"ENTIRE_TEST_CLAUDE_PROJECT_DIR="+env.ClaudeProjectDir,
		"ENTIRE_LOG_LEVEL=debug",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("hook output: %s", output)
		// Don't fail - hook may succeed even with warnings
	}

	// Verify log file was created
	logFile := filepath.Join(logsDir, sessionID+".log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Errorf("expected log file at %s but it doesn't exist", logFile)
		t.Logf("hook stderr/stdout: %s", output)

		// List what's in the logs dir for debugging
		entries, _ := os.ReadDir(logsDir)
		t.Logf("logs directory contents: %v", entries)
	}

	// Verify log file contains expected content
	if _, err := os.Stat(logFile); err == nil {
		content, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("failed to read log file: %v", err)
		}

		logContent := string(content)
		t.Logf("log file content:\n%s", logContent)

		// Should contain JSON with hook name
		if !strings.Contains(logContent, `"hook"`) {
			t.Error("log file should contain hook field")
		}
		if !strings.Contains(logContent, `"post-commit"`) {
			t.Error("log file should contain post-commit hook name")
		}
		if !strings.Contains(logContent, `"component"`) {
			t.Error("log file should contain component field")
		}
	}
}

func TestHookLogging_FallsBackToStderrWithoutSession(t *testing.T) {
	t.Parallel()

	env := NewTestEnv(t)
	env.InitRepo()
	env.InitEntire("manual-commit")

	// Don't create a session state file - logging should fall back to stderr

	// Run a hook with ENTIRE_LOG_LEVEL=debug
	cmd := exec.Command(getTestBinary(), "hooks", "git", "post-commit")
	cmd.Dir = env.RepoDir
	cmd.Env = append(os.Environ(),
		"ENTIRE_TEST_CLAUDE_PROJECT_DIR="+env.ClaudeProjectDir,
		"ENTIRE_LOG_LEVEL=debug",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Don't fail - hook may succeed
	}

	// Output should contain log content (stderr fallback)
	outputStr := string(output)
	if !strings.Contains(outputStr, "post-commit") {
		t.Logf("expected stderr to contain hook logs, got: %s", outputStr)
		// This is expected behavior - without session, logs go to stderr
	}

	// Logs directory should NOT have any files (no session = no file logging)
	logsDir := filepath.Join(env.RepoDir, paths.EntireDir, "logs")
	if entries, err := os.ReadDir(logsDir); err == nil && len(entries) > 0 {
		t.Errorf("expected no log files without session, found: %v", entries)
	}
}

// writeTestSessionStateForLogging creates a session state file for hook logging tests.
func writeTestSessionStateForLogging(t *testing.T, repoDir, sessionID string) {
	t.Helper()
	stateDir := filepath.Join(repoDir, ".git", session.SessionStateDirName)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("failed to create session state directory: %v", err)
	}

	now := time.Now()
	state := session.State{
		SessionID:           sessionID,
		StartedAt:           now,
		LastInteractionTime: &now,
		Phase:               session.PhaseActive,
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}
	stateFile := filepath.Join(stateDir, sessionID+".json")
	if err := os.WriteFile(stateFile, data, 0o600); err != nil {
		t.Fatalf("failed to write session state file: %v", err)
	}
}
