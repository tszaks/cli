package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/entireio/cli/cmd/entire/cli/paths"
)

func TestInitHookLogging(t *testing.T) {
	// Create a temporary directory to simulate a git repo
	tmpDir := t.TempDir()

	// Change to temp dir (automatically restored after test)
	t.Chdir(tmpDir)

	// Initialize git repo (required for paths.AbsPath to work)
	if err := os.MkdirAll(".git", 0o755); err != nil {
		t.Fatalf("failed to create .git directory: %v", err)
	}

	t.Run("returns cleanup func when no session file exists", func(t *testing.T) {
		cleanup := initHookLogging()
		if cleanup == nil {
			t.Fatal("expected cleanup function, got nil")
		}
		cleanup() // Should not panic
	})

	t.Run("initializes logging when session file exists", func(t *testing.T) {
		// Create .entire directory and session file
		entireDir := filepath.Join(tmpDir, paths.EntireDir)
		if err := os.MkdirAll(entireDir, 0o755); err != nil {
			t.Fatalf("failed to create .entire directory: %v", err)
		}

		sessionID := "test-session-12345"
		sessionFile := filepath.Join(tmpDir, paths.CurrentSessionFile)
		if err := os.WriteFile(sessionFile, []byte(sessionID), 0o600); err != nil {
			t.Fatalf("failed to write session file: %v", err)
		}
		defer os.Remove(sessionFile)

		// Create logs directory (logging.Init will try to create the log file)
		logsDir := filepath.Join(entireDir, "logs")
		if err := os.MkdirAll(logsDir, 0o755); err != nil {
			t.Fatalf("failed to create logs directory: %v", err)
		}

		cleanup := initHookLogging()
		if cleanup == nil {
			t.Fatal("expected cleanup function, got nil")
		}
		defer cleanup()

		// Verify log file was created
		logFile := filepath.Join(logsDir, sessionID+".log")
		if _, err := os.Stat(logFile); os.IsNotExist(err) {
			t.Errorf("expected log file to be created at %s", logFile)
		}
	})

	t.Run("returns cleanup func when session file is empty", func(t *testing.T) {
		// Create empty session file
		sessionFile := filepath.Join(tmpDir, paths.CurrentSessionFile)
		if err := os.WriteFile(sessionFile, []byte(""), 0o600); err != nil {
			t.Fatalf("failed to write empty session file: %v", err)
		}
		defer os.Remove(sessionFile)

		cleanup := initHookLogging()
		if cleanup == nil {
			t.Fatal("expected cleanup function, got nil")
		}
		cleanup() // Should not panic
	})
}
