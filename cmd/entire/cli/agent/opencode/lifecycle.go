package opencode

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/entireio/cli/cmd/entire/cli/agent"
	"github.com/entireio/cli/cmd/entire/cli/paths"
)

// Hook name constants — these become CLI subcommands under `entire hooks opencode`.
const (
	HookNameSessionStart = "session-start"
	HookNameSessionEnd   = "session-end"
	HookNameTurnStart    = "turn-start"
	HookNameTurnEnd      = "turn-end"
	HookNameCompaction   = "compaction"
)

// HookNames returns the hook verbs this agent supports.
func (a *OpenCodeAgent) HookNames() []string {
	return []string{
		HookNameSessionStart,
		HookNameSessionEnd,
		HookNameTurnStart,
		HookNameTurnEnd,
		HookNameCompaction,
	}
}

// ParseHookEvent translates OpenCode hook calls into normalized lifecycle events.
func (a *OpenCodeAgent) ParseHookEvent(hookName string, stdin io.Reader) (*agent.Event, error) {
	switch hookName {
	case HookNameSessionStart:
		raw, err := agent.ReadAndParseHookInput[sessionInfoRaw](stdin)
		if err != nil {
			return nil, err
		}
		return &agent.Event{
			Type:      agent.SessionStart,
			SessionID: raw.SessionID,
			Timestamp: time.Now(),
		}, nil

	case HookNameTurnStart:
		raw, err := agent.ReadAndParseHookInput[turnStartRaw](stdin)
		if err != nil {
			return nil, err
		}
		// Get the temp file path for this session (may not exist yet, but needed for pre-prompt state)
		repoRoot, _ := paths.RepoRoot() //nolint:errcheck // fallback to empty string is fine for path construction
		tmpDir := filepath.Join(repoRoot, paths.EntireTmpDir)
		transcriptPath := filepath.Join(tmpDir, raw.SessionID+".json")
		return &agent.Event{
			Type:       agent.TurnStart,
			SessionID:  raw.SessionID,
			SessionRef: transcriptPath,
			Prompt:     raw.Prompt,
			Timestamp:  time.Now(),
		}, nil

	case HookNameTurnEnd:
		raw, err := agent.ReadAndParseHookInput[sessionInfoRaw](stdin)
		if err != nil {
			return nil, err
		}
		// Call `opencode export` to get the transcript and write to temp file
		transcriptPath, exportErr := a.fetchAndCacheExport(raw.SessionID)
		if exportErr != nil {
			return nil, fmt.Errorf("failed to export session: %w", exportErr)
		}
		return &agent.Event{
			Type:       agent.TurnEnd,
			SessionID:  raw.SessionID,
			SessionRef: transcriptPath,
			Timestamp:  time.Now(),
		}, nil

	case HookNameCompaction:
		raw, err := agent.ReadAndParseHookInput[sessionInfoRaw](stdin)
		if err != nil {
			return nil, err
		}
		return &agent.Event{
			Type:      agent.Compaction,
			SessionID: raw.SessionID,
			Timestamp: time.Now(),
		}, nil

	case HookNameSessionEnd:
		raw, err := agent.ReadAndParseHookInput[sessionInfoRaw](stdin)
		if err != nil {
			return nil, err
		}
		return &agent.Event{
			Type:      agent.SessionEnd,
			SessionID: raw.SessionID,
			Timestamp: time.Now(),
		}, nil

	default:
		return nil, nil //nolint:nilnil // nil event = no lifecycle action for unknown hooks
	}
}

// fetchAndCacheExport calls `opencode export <sessionID>` and writes the result
// to a temporary file. Returns the path to the temp file.
//
// For integration testing: if a file already exists at the expected location
// (.entire/tmp/<sessionID>.json), it is used directly instead of calling
// `opencode export`. This allows tests to provide mock transcript data.
func (a *OpenCodeAgent) fetchAndCacheExport(sessionID string) (string, error) {
	// Get repo root for the temp directory
	repoRoot, err := paths.RepoRoot()
	if err != nil {
		repoRoot = "."
	}

	tmpDir := filepath.Join(repoRoot, paths.EntireTmpDir)
	tmpFile := filepath.Join(tmpDir, sessionID+".json")

	// Check if file already exists (e.g., pre-written by integration tests)
	if _, err := os.Stat(tmpFile); err == nil {
		return tmpFile, nil
	}

	// Call opencode export to get the transcript
	data, err := runOpenCodeExport(sessionID)
	if err != nil {
		return "", fmt.Errorf("opencode export failed: %w", err)
	}

	// Write to temp directory under .entire
	if err := os.MkdirAll(tmpDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		return "", fmt.Errorf("failed to write export file: %w", err)
	}

	return tmpFile, nil
}
