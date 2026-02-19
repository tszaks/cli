package cursor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/entireio/cli/cmd/entire/cli/agent"
	"github.com/entireio/cli/cmd/entire/cli/jsonutil"
	"github.com/entireio/cli/cmd/entire/cli/paths"
)

// Ensure CursorAgent implements HookSupport and HookHandler
var (
	_ agent.HookSupport = (*CursorAgent)(nil)
	_ agent.HookHandler = (*CursorAgent)(nil)
)

// Cursor hook names - these become subcommands under `entire hooks cursor`
const (
	HookNameSessionStart       = "session-start"
	HookNameSessionEnd         = "session-end"
	HookNameBeforeSubmitPrompt = "before-submit-prompt"
	HookNameStop               = "stop"
	HookNamePreTask            = "pre-task"
	HookNamePostTask           = "post-task"
	HookNamePostTodo           = "post-todo"
)

// HooksFileName is the hooks file used by Cursor.
const HooksFileName = "hooks.json"

// entireHookPrefixes are command prefixes that identify Entire hooks
var entireHookPrefixes = []string{
	"entire ",
	"go run ${CURSOR_PROJECT_DIR}/cmd/entire/main.go ",
}

// GetHookNames returns the hook verbs Cursor supports.
// These become subcommands: entire hooks cursor <verb>
func (c *CursorAgent) GetHookNames() []string {
	return []string{
		HookNameSessionStart,
		HookNameSessionEnd,
		HookNameBeforeSubmitPrompt,
		HookNameStop,
		HookNamePreTask,
		HookNamePostTask,
		HookNamePostTodo,
	}
}

// InstallHooks installs Cursor hooks in .cursor/hooks.json.
// If force is true, removes existing Entire hooks before installing.
// Returns the number of hooks installed.
// Unknown top-level fields and hook types are preserved on round-trip.
func (c *CursorAgent) InstallHooks(localDev bool, force bool) (int, error) {
	repoRoot, err := paths.RepoRoot()
	if err != nil {
		repoRoot, err = os.Getwd() //nolint:forbidigo // Intentional fallback when RepoRoot() fails (tests run outside git repos)
		if err != nil {
			return 0, fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	hooksPath := filepath.Join(repoRoot, ".cursor", HooksFileName)

	// Use raw maps to preserve unknown fields on round-trip
	var rawFile map[string]json.RawMessage
	var rawHooks map[string]json.RawMessage

	existingData, readErr := os.ReadFile(hooksPath) //nolint:gosec // path is constructed from repo root + fixed path
	if readErr == nil {
		if err := json.Unmarshal(existingData, &rawFile); err != nil {
			return 0, fmt.Errorf("failed to parse existing "+HooksFileName+": %w", err)
		}
		if hooksRaw, ok := rawFile["hooks"]; ok {
			if err := json.Unmarshal(hooksRaw, &rawHooks); err != nil {
				return 0, fmt.Errorf("failed to parse hooks in "+HooksFileName+": %w", err)
			}
		}
	} else {
		rawFile = map[string]json.RawMessage{
			"version": json.RawMessage(`1`),
		}
	}

	if rawHooks == nil {
		rawHooks = make(map[string]json.RawMessage)
	}

	// Parse only the hook types we manage
	var sessionStart, sessionEnd, beforeSubmitPrompt, stop, preToolUse, postToolUse []CursorHookEntry
	parseCursorHookType(rawHooks, "sessionStart", &sessionStart)
	parseCursorHookType(rawHooks, "sessionEnd", &sessionEnd)
	parseCursorHookType(rawHooks, "beforeSubmitPrompt", &beforeSubmitPrompt)
	parseCursorHookType(rawHooks, "stop", &stop)
	parseCursorHookType(rawHooks, "preToolUse", &preToolUse)
	parseCursorHookType(rawHooks, "postToolUse", &postToolUse)

	// If force is true, remove all existing Entire hooks first
	if force {
		sessionStart = removeEntireHooks(sessionStart)
		sessionEnd = removeEntireHooks(sessionEnd)
		beforeSubmitPrompt = removeEntireHooks(beforeSubmitPrompt)
		stop = removeEntireHooks(stop)
		preToolUse = removeEntireHooks(preToolUse)
		postToolUse = removeEntireHooks(postToolUse)
	}

	// Define hook commands
	var cmdPrefix string
	if localDev {
		cmdPrefix = "go run ${CURSOR_PROJECT_DIR}/cmd/entire/main.go hooks cursor "
	} else {
		cmdPrefix = "entire hooks cursor "
	}

	sessionStartCmd := cmdPrefix + "session-start"
	sessionEndCmd := cmdPrefix + "session-end"
	beforeSubmitPromptCmd := cmdPrefix + "before-submit-prompt"
	stopCmd := cmdPrefix + "stop"
	preTaskCmd := cmdPrefix + "pre-task"
	postTaskCmd := cmdPrefix + "post-task"
	postTodoCmd := cmdPrefix + "post-todo"

	count := 0

	// Add hooks if they don't exist
	if !hookCommandExists(sessionStart, sessionStartCmd) {
		sessionStart = append(sessionStart, CursorHookEntry{Command: sessionStartCmd})
		count++
	}
	if !hookCommandExists(sessionEnd, sessionEndCmd) {
		sessionEnd = append(sessionEnd, CursorHookEntry{Command: sessionEndCmd})
		count++
	}
	if !hookCommandExists(beforeSubmitPrompt, beforeSubmitPromptCmd) {
		beforeSubmitPrompt = append(beforeSubmitPrompt, CursorHookEntry{Command: beforeSubmitPromptCmd})
		count++
	}
	if !hookCommandExists(stop, stopCmd) {
		stop = append(stop, CursorHookEntry{Command: stopCmd})
		count++
	}
	if !hookCommandExistsWithMatcher(preToolUse, "Task", preTaskCmd) {
		preToolUse = append(preToolUse, CursorHookEntry{Command: preTaskCmd, Matcher: "Task"})
		count++
	}
	if !hookCommandExistsWithMatcher(postToolUse, "Task", postTaskCmd) {
		postToolUse = append(postToolUse, CursorHookEntry{Command: postTaskCmd, Matcher: "Task"})
		count++
	}
	if !hookCommandExistsWithMatcher(postToolUse, "TodoWrite", postTodoCmd) {
		postToolUse = append(postToolUse, CursorHookEntry{Command: postTodoCmd, Matcher: "TodoWrite"})
		count++
	}

	if count == 0 {
		return 0, nil
	}

	// Marshal modified hook types back into rawHooks
	marshalCursorHookType(rawHooks, "sessionStart", sessionStart)
	marshalCursorHookType(rawHooks, "sessionEnd", sessionEnd)
	marshalCursorHookType(rawHooks, "beforeSubmitPrompt", beforeSubmitPrompt)
	marshalCursorHookType(rawHooks, "stop", stop)
	marshalCursorHookType(rawHooks, "preToolUse", preToolUse)
	marshalCursorHookType(rawHooks, "postToolUse", postToolUse)

	// Marshal hooks and update raw file
	hooksJSON, err := json.Marshal(rawHooks)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal hooks: %w", err)
	}
	rawFile["hooks"] = hooksJSON

	// Write to file
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o750); err != nil {
		return 0, fmt.Errorf("failed to create .cursor directory: %w", err)
	}

	output, err := jsonutil.MarshalIndentWithNewline(rawFile, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("failed to marshal "+HooksFileName+": %w", err)
	}

	if err := os.WriteFile(hooksPath, output, 0o600); err != nil {
		return 0, fmt.Errorf("failed to write "+HooksFileName+": %w", err)
	}

	return count, nil
}

// UninstallHooks removes Entire hooks from Cursor HooksFileName.
// Unknown top-level fields and hook types are preserved on round-trip.
func (c *CursorAgent) UninstallHooks() error {
	repoRoot, err := paths.RepoRoot()
	if err != nil {
		repoRoot = "."
	}
	hooksPath := filepath.Join(repoRoot, ".cursor", HooksFileName)
	data, err := os.ReadFile(hooksPath) //nolint:gosec // path is constructed from repo root + fixed path
	if err != nil {
		return nil //nolint:nilerr // No hooks file means nothing to uninstall
	}

	var rawFile map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawFile); err != nil {
		return fmt.Errorf("failed to parse "+HooksFileName+": %w", err)
	}

	var rawHooks map[string]json.RawMessage
	if hooksRaw, ok := rawFile["hooks"]; ok {
		if err := json.Unmarshal(hooksRaw, &rawHooks); err != nil {
			return fmt.Errorf("failed to parse hooks in "+HooksFileName+": %w", err)
		}
	}
	if rawHooks == nil {
		rawHooks = make(map[string]json.RawMessage)
	}

	// Parse only the hook types we manage
	var sessionStart, sessionEnd, beforeSubmitPrompt, stop, preToolUse, postToolUse []CursorHookEntry
	parseCursorHookType(rawHooks, "sessionStart", &sessionStart)
	parseCursorHookType(rawHooks, "sessionEnd", &sessionEnd)
	parseCursorHookType(rawHooks, "beforeSubmitPrompt", &beforeSubmitPrompt)
	parseCursorHookType(rawHooks, "stop", &stop)
	parseCursorHookType(rawHooks, "preToolUse", &preToolUse)
	parseCursorHookType(rawHooks, "postToolUse", &postToolUse)

	// Remove Entire hooks from all hook types
	sessionStart = removeEntireHooks(sessionStart)
	sessionEnd = removeEntireHooks(sessionEnd)
	beforeSubmitPrompt = removeEntireHooks(beforeSubmitPrompt)
	stop = removeEntireHooks(stop)
	preToolUse = removeEntireHooks(preToolUse)
	postToolUse = removeEntireHooks(postToolUse)

	// Marshal modified hook types back into rawHooks
	marshalCursorHookType(rawHooks, "sessionStart", sessionStart)
	marshalCursorHookType(rawHooks, "sessionEnd", sessionEnd)
	marshalCursorHookType(rawHooks, "beforeSubmitPrompt", beforeSubmitPrompt)
	marshalCursorHookType(rawHooks, "stop", stop)
	marshalCursorHookType(rawHooks, "preToolUse", preToolUse)
	marshalCursorHookType(rawHooks, "postToolUse", postToolUse)

	// Marshal hooks back (preserving unknown hook types)
	if len(rawHooks) > 0 {
		hooksJSON, err := json.Marshal(rawHooks)
		if err != nil {
			return fmt.Errorf("failed to marshal hooks: %w", err)
		}
		rawFile["hooks"] = hooksJSON
	} else {
		delete(rawFile, "hooks")
	}

	// Write back
	output, err := jsonutil.MarshalIndentWithNewline(rawFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal "+HooksFileName+": %w", err)
	}

	if err := os.WriteFile(hooksPath, output, 0o600); err != nil {
		return fmt.Errorf("failed to write "+HooksFileName+": %w", err)
	}
	return nil
}

// AreHooksInstalled checks if Entire hooks are installed.
func (c *CursorAgent) AreHooksInstalled() bool {
	repoRoot, err := paths.RepoRoot()
	if err != nil {
		repoRoot = "."
	}
	hooksPath := filepath.Join(repoRoot, ".cursor", HooksFileName)
	data, err := os.ReadFile(hooksPath) //nolint:gosec // path is constructed from repo root + fixed path
	if err != nil {
		return false
	}

	var hooksFile CursorHooksFile
	if err := json.Unmarshal(data, &hooksFile); err != nil {
		return false
	}

	return hasEntireHook(hooksFile.Hooks.SessionStart) ||
		hasEntireHook(hooksFile.Hooks.SessionEnd) ||
		hasEntireHook(hooksFile.Hooks.BeforeSubmitPrompt) ||
		hasEntireHook(hooksFile.Hooks.Stop) ||
		hasEntireHook(hooksFile.Hooks.PreToolUse) ||
		hasEntireHook(hooksFile.Hooks.PostToolUse)
}

// GetSupportedHooks returns the hook types Cursor supports.
func (c *CursorAgent) GetSupportedHooks() []agent.HookType {
	return []agent.HookType{
		agent.HookSessionStart,
		agent.HookSessionEnd,
		agent.HookUserPromptSubmit,
		agent.HookStop,
		agent.HookPreToolUse,
		agent.HookPostToolUse,
	}
}

// parseCursorHookType parses a specific hook type from rawHooks into the target slice.
// Silently ignores parse errors (leaves target unchanged).
func parseCursorHookType(rawHooks map[string]json.RawMessage, hookType string, target *[]CursorHookEntry) {
	if data, ok := rawHooks[hookType]; ok {
		//nolint:errcheck,gosec // Intentionally ignoring parse errors - leave target as nil/empty
		json.Unmarshal(data, target)
	}
}

// marshalCursorHookType marshals a hook type back into rawHooks.
// If the slice is empty, removes the key from rawHooks.
func marshalCursorHookType(rawHooks map[string]json.RawMessage, hookType string, entries []CursorHookEntry) {
	if len(entries) == 0 {
		delete(rawHooks, hookType)
		return
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return // Silently ignore marshal errors (shouldn't happen)
	}
	rawHooks[hookType] = data
}

// Helper functions for hook management

func hookCommandExists(entries []CursorHookEntry, command string) bool {
	for _, entry := range entries {
		if entry.Command == command {
			return true
		}
	}
	return false
}

func hookCommandExistsWithMatcher(entries []CursorHookEntry, matcher, command string) bool {
	for _, entry := range entries {
		if entry.Matcher == matcher && entry.Command == command {
			return true
		}
	}
	return false
}

func isEntireHook(command string) bool {
	for _, prefix := range entireHookPrefixes {
		if strings.HasPrefix(command, prefix) {
			return true
		}
	}
	return false
}

func hasEntireHook(entries []CursorHookEntry) bool {
	for _, entry := range entries {
		if isEntireHook(entry.Command) {
			return true
		}
	}
	return false
}

func removeEntireHooks(entries []CursorHookEntry) []CursorHookEntry {
	result := make([]CursorHookEntry, 0, len(entries))
	for _, entry := range entries {
		if !isEntireHook(entry.Command) {
			result = append(result, entry)
		}
	}
	return result
}
