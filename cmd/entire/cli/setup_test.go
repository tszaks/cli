package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"entire.io/cli/cmd/entire/cli/paths"
	"github.com/go-git/go-git/v5"
)

// Note: Tests for hook manipulation functions (addHookToMatcher, hookCommandExists, etc.)
// have been moved to the agent/claudecode package where these functions now reside.
// See cmd/entire/cli/agent/claudecode/hooks_test.go for those tests.

// setupTestDir creates a temp directory, changes to it, and returns it.
// It also registers cleanup to restore the original directory.
func setupTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	paths.ClearRepoRootCache()
	return tmpDir
}

// setupTestRepo creates a temp directory with a git repo initialized.
func setupTestRepo(t *testing.T) {
	t.Helper()
	tmpDir := setupTestDir(t)
	if _, err := git.PlainInit(tmpDir, false); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}
}

// writeSettings writes settings content to the settings file.
func writeSettings(t *testing.T, content string) {
	t.Helper()
	settingsDir := filepath.Dir(EntireSettingsFile)
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("Failed to create settings dir: %v", err)
	}
	if err := os.WriteFile(EntireSettingsFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to write settings file: %v", err)
	}
}

func TestRunEnable(t *testing.T) {
	setupTestDir(t)
	writeSettings(t, testSettingsDisabled)

	var stdout bytes.Buffer
	if err := runEnable(&stdout); err != nil {
		t.Fatalf("runEnable() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "enabled") {
		t.Errorf("Expected output to contain 'enabled', got: %s", stdout.String())
	}

	enabled, err := IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled() error = %v", err)
	}
	if !enabled {
		t.Error("Entire should be enabled after running enable command")
	}
}

func TestRunEnable_AlreadyEnabled(t *testing.T) {
	setupTestDir(t)
	writeSettings(t, testSettingsEnabled)

	var stdout bytes.Buffer
	if err := runEnable(&stdout); err != nil {
		t.Fatalf("runEnable() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "enabled") {
		t.Errorf("Expected output to mention enabled state, got: %s", stdout.String())
	}
}

func TestRunDisable(t *testing.T) {
	setupTestDir(t)
	writeSettings(t, testSettingsEnabled)

	var stdout bytes.Buffer
	if err := runDisable(&stdout, false); err != nil {
		t.Fatalf("runDisable() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "disabled") {
		t.Errorf("Expected output to contain 'disabled', got: %s", stdout.String())
	}

	enabled, err := IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled() error = %v", err)
	}
	if enabled {
		t.Error("Entire should be disabled after running disable command")
	}
}

func TestRunDisable_AlreadyDisabled(t *testing.T) {
	setupTestDir(t)
	writeSettings(t, testSettingsDisabled)

	var stdout bytes.Buffer
	if err := runDisable(&stdout, false); err != nil {
		t.Fatalf("runDisable() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "disabled") {
		t.Errorf("Expected output to mention disabled state, got: %s", stdout.String())
	}
}

func TestRunStatus_Enabled(t *testing.T) {
	setupTestRepo(t)
	writeSettings(t, testSettingsEnabled)

	var stdout bytes.Buffer
	if err := runStatus(&stdout); err != nil {
		t.Fatalf("runStatus() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "● enabled") {
		t.Errorf("Expected output to show '● enabled', got: %s", stdout.String())
	}
}

func TestRunStatus_Disabled(t *testing.T) {
	setupTestRepo(t)
	writeSettings(t, testSettingsDisabled)

	var stdout bytes.Buffer
	if err := runStatus(&stdout); err != nil {
		t.Fatalf("runStatus() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "○ disabled") {
		t.Errorf("Expected output to show '○ disabled', got: %s", stdout.String())
	}
}

func TestRunStatus_NotSetUp(t *testing.T) {
	setupTestRepo(t)

	var stdout bytes.Buffer
	if err := runStatus(&stdout); err != nil {
		t.Fatalf("runStatus() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "○ not set up") {
		t.Errorf("Expected output to show '○ not set up', got: %s", output)
	}
	if !strings.Contains(output, "entire enable") {
		t.Errorf("Expected output to mention 'entire enable', got: %s", output)
	}
}

func TestRunStatus_NotGitRepository(t *testing.T) {
	setupTestDir(t) // No git init

	var stdout bytes.Buffer
	if err := runStatus(&stdout); err != nil {
		t.Fatalf("runStatus() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "✕ not a git repository") {
		t.Errorf("Expected output to show '✕ not a git repository', got: %s", stdout.String())
	}
}

func TestCheckDisabledGuard(t *testing.T) {
	setupTestDir(t)

	// No settings file - should not be disabled (defaults to enabled)
	var stdout bytes.Buffer
	if checkDisabledGuard(&stdout) {
		t.Error("checkDisabledGuard() should return false when no settings file exists")
	}
	if stdout.String() != "" {
		t.Errorf("checkDisabledGuard() should not print anything when enabled, got: %s", stdout.String())
	}

	// Settings with enabled: true
	writeSettings(t, testSettingsEnabled)
	stdout.Reset()
	if checkDisabledGuard(&stdout) {
		t.Error("checkDisabledGuard() should return false when enabled")
	}

	// Settings with enabled: false
	writeSettings(t, testSettingsDisabled)
	stdout.Reset()
	if !checkDisabledGuard(&stdout) {
		t.Error("checkDisabledGuard() should return true when disabled")
	}
	output := stdout.String()
	if !strings.Contains(output, "Entire is disabled") {
		t.Errorf("Expected disabled message, got: %s", output)
	}
	if !strings.Contains(output, "entire enable") {
		t.Errorf("Expected message to mention 'entire enable', got: %s", output)
	}
}

// writeLocalSettings writes settings content to the local settings file.
func writeLocalSettings(t *testing.T, content string) {
	t.Helper()
	settingsDir := filepath.Dir(EntireSettingsLocalFile)
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("Failed to create settings dir: %v", err)
	}
	if err := os.WriteFile(EntireSettingsLocalFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to write local settings file: %v", err)
	}
}

func TestRunDisable_WithLocalSettings(t *testing.T) {
	setupTestDir(t)
	// Create both settings files with enabled: true
	writeSettings(t, testSettingsEnabled)
	writeLocalSettings(t, `{"enabled": true}`)

	var stdout bytes.Buffer
	if err := runDisable(&stdout, false); err != nil {
		t.Fatalf("runDisable() error = %v", err)
	}

	// Should be disabled because runDisable updates local settings when it exists
	enabled, err := IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled() error = %v", err)
	}
	if enabled {
		t.Error("Entire should be disabled after running disable command (local settings should be updated)")
	}

	// Verify local settings file was updated
	localContent, err := os.ReadFile(EntireSettingsLocalFile)
	if err != nil {
		t.Fatalf("Failed to read local settings: %v", err)
	}
	if !strings.Contains(string(localContent), `"enabled":false`) && !strings.Contains(string(localContent), `"enabled": false`) {
		t.Errorf("Local settings should have enabled:false, got: %s", localContent)
	}
}

func TestRunDisable_WithProjectFlag(t *testing.T) {
	setupTestDir(t)
	// Create both settings files with enabled: true
	writeSettings(t, testSettingsEnabled)
	writeLocalSettings(t, `{"enabled": true}`)

	var stdout bytes.Buffer
	// Use --project flag (useProjectSettings = true)
	if err := runDisable(&stdout, true); err != nil {
		t.Fatalf("runDisable() error = %v", err)
	}

	// Verify project settings file was updated (not local)
	projectContent, err := os.ReadFile(EntireSettingsFile)
	if err != nil {
		t.Fatalf("Failed to read project settings: %v", err)
	}
	if !strings.Contains(string(projectContent), `"enabled":false`) && !strings.Contains(string(projectContent), `"enabled": false`) {
		t.Errorf("Project settings should have enabled:false, got: %s", projectContent)
	}

	// Local settings should still be enabled (untouched)
	localContent, err := os.ReadFile(EntireSettingsLocalFile)
	if err != nil {
		t.Fatalf("Failed to read local settings: %v", err)
	}
	if !strings.Contains(string(localContent), `"enabled": true`) && !strings.Contains(string(localContent), `"enabled":true`) {
		t.Errorf("Local settings should still have enabled:true, got: %s", localContent)
	}
}

func TestRunDisable_NoLocalSettings(t *testing.T) {
	setupTestDir(t)
	// Only create project settings
	writeSettings(t, testSettingsEnabled)

	var stdout bytes.Buffer
	if err := runDisable(&stdout, false); err != nil {
		t.Fatalf("runDisable() error = %v", err)
	}

	// Should be disabled
	enabled, err := IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled() error = %v", err)
	}
	if enabled {
		t.Error("Entire should be disabled after running disable command")
	}

	// Project settings should be updated
	projectContent, err := os.ReadFile(EntireSettingsFile)
	if err != nil {
		t.Fatalf("Failed to read project settings: %v", err)
	}
	if !strings.Contains(string(projectContent), `"enabled":false`) && !strings.Contains(string(projectContent), `"enabled": false`) {
		t.Errorf("Project settings should have enabled:false, got: %s", projectContent)
	}
}

func TestDetermineSettingsTarget_ExplicitLocalFlag(t *testing.T) {
	tmpDir := t.TempDir()

	// Create settings.json
	settingsPath := filepath.Join(tmpDir, paths.SettingsFileName)
	if err := os.WriteFile(settingsPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("Failed to create settings file: %v", err)
	}

	// With --local flag, should always use local
	useLocal, showNotification := determineSettingsTarget(tmpDir, true, false)
	if !useLocal {
		t.Error("determineSettingsTarget() should return useLocal=true with --local flag")
	}
	if showNotification {
		t.Error("determineSettingsTarget() should not show notification with explicit --local flag")
	}
}

func TestDetermineSettingsTarget_ExplicitProjectFlag(t *testing.T) {
	tmpDir := t.TempDir()

	// Create settings.json
	settingsPath := filepath.Join(tmpDir, paths.SettingsFileName)
	if err := os.WriteFile(settingsPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("Failed to create settings file: %v", err)
	}

	// With --project flag, should always use project
	useLocal, showNotification := determineSettingsTarget(tmpDir, false, true)
	if useLocal {
		t.Error("determineSettingsTarget() should return useLocal=false with --project flag")
	}
	if showNotification {
		t.Error("determineSettingsTarget() should not show notification with explicit --project flag")
	}
}

func TestDetermineSettingsTarget_SettingsExists_NoFlags(t *testing.T) {
	tmpDir := t.TempDir()

	// Create settings.json
	settingsPath := filepath.Join(tmpDir, paths.SettingsFileName)
	if err := os.WriteFile(settingsPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("Failed to create settings file: %v", err)
	}

	// Without flags, should auto-redirect to local with notification
	useLocal, showNotification := determineSettingsTarget(tmpDir, false, false)
	if !useLocal {
		t.Error("determineSettingsTarget() should return useLocal=true when settings.json exists")
	}
	if !showNotification {
		t.Error("determineSettingsTarget() should show notification when auto-redirecting to local")
	}
}

func TestDetermineSettingsTarget_SettingsNotExists_NoFlags(t *testing.T) {
	tmpDir := t.TempDir()

	// No settings.json exists

	// Should use project settings (create new)
	useLocal, showNotification := determineSettingsTarget(tmpDir, false, false)
	if useLocal {
		t.Error("determineSettingsTarget() should return useLocal=false when settings.json doesn't exist")
	}
	if showNotification {
		t.Error("determineSettingsTarget() should not show notification when creating new settings")
	}
}
