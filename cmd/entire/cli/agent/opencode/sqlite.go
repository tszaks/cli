package opencode

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// openCodeImportTimeout is the maximum time to wait for `opencode import` to complete.
const openCodeImportTimeout = 30 * time.Second

// getOpenCodeDBPath returns the path to OpenCode's SQLite database.
// OpenCode always uses ~/.local/share/opencode/opencode.db (XDG default)
// regardless of platform — it does NOT use ~/Library/Application Support on macOS.
//
// XDG_DATA_HOME overrides the default on all platforms.
func getOpenCodeDBPath() (string, error) {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		dataDir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataDir, "opencode", "opencode.db"), nil
}

// runSQLiteQuery executes a SQL query against OpenCode's SQLite database.
// Returns the combined stdout/stderr output.
func runSQLiteQuery(query string, timeout time.Duration) ([]byte, error) {
	dbPath, err := getOpenCodeDBPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenCode DB path: %w", err)
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("OpenCode database not found: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	//nolint:gosec // G204: query is constructed from sanitized inputs (escapeSQLiteString)
	cmd := exec.CommandContext(ctx, "sqlite3", dbPath, query)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("sqlite3 query failed: %w", err)
	}
	return output, nil
}

// sessionExistsInSQLite checks whether a session with the given ID exists
// in OpenCode's SQLite database.
func sessionExistsInSQLite(sessionID string) bool {
	query := fmt.Sprintf("SELECT count(*) FROM session WHERE id = '%s';", escapeSQLiteString(sessionID))
	output, err := runSQLiteQuery(query, 5*time.Second)
	if err != nil {
		return false
	}
	return len(output) > 0 && output[0] != '0'
}

// deleteMessagesFromSQLite removes all messages (and cascading parts) for a session.
// This is used before reimporting a session during rewind so that `opencode import`
// can insert the checkpoint-state messages (import uses ON CONFLICT DO NOTHING).
func deleteMessagesFromSQLite(sessionID string) error {
	// Enable foreign keys so CASCADE deletes work (parts are deleted with messages).
	query := fmt.Sprintf(
		"PRAGMA foreign_keys = ON; DELETE FROM message WHERE session_id = '%s';",
		escapeSQLiteString(sessionID),
	)
	if output, err := runSQLiteQuery(query, 5*time.Second); err != nil {
		return fmt.Errorf("failed to delete messages from OpenCode DB: %w (output: %s)", err, string(output))
	}
	return nil
}

// runOpenCodeImport runs `opencode import <file>` to import a session into
// OpenCode's SQLite database. The import preserves the original session ID
// from the export file.
func runOpenCodeImport(exportFilePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), openCodeImportTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "opencode", "import", exportFilePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("opencode import timed out after %s", openCodeImportTimeout)
		}
		return fmt.Errorf("opencode import failed: %w (output: %s)", err, string(output))
	}

	return nil
}

// escapeSQLiteString escapes single quotes in a string for safe use in SQLite queries.
func escapeSQLiteString(s string) string {
	result := make([]byte, 0, len(s))
	for i := range len(s) {
		if s[i] == '\'' {
			result = append(result, '\'', '\'')
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}
