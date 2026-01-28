package cli

import (
	"os"
	"path/filepath"
	"testing"

	"entire.io/cli/cmd/entire/cli/agent"
	"entire.io/cli/cmd/entire/cli/strategy"
)

const (
	testSettingsStrategy = `{"strategy": "manual-commit"}`
	testSettingsEnabled  = `{"strategy": "manual-commit", "enabled": true}`
	testSettingsDisabled = `{"strategy": "manual-commit", "enabled": false}`
)

func TestLoadEntireSettings_EnabledDefaultsToTrue(t *testing.T) {
	// Create a temporary directory and change to it (auto-restored after test)
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Test 1: No settings file exists - should default to enabled
	settings, err := LoadEntireSettings()
	if err != nil {
		t.Fatalf("LoadEntireSettings() error = %v", err)
	}
	if !settings.Enabled {
		t.Error("Enabled should default to true when no settings file exists")
	}

	// Test 2: Settings file exists without enabled field - should default to true
	settingsDir := filepath.Dir(EntireSettingsFile)
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("Failed to create settings dir: %v", err)
	}
	settingsContent := testSettingsStrategy
	if err := os.WriteFile(EntireSettingsFile, []byte(settingsContent), 0o644); err != nil {
		t.Fatalf("Failed to write settings file: %v", err)
	}

	settings, err = LoadEntireSettings()
	if err != nil {
		t.Fatalf("LoadEntireSettings() error = %v", err)
	}
	if !settings.Enabled {
		t.Error("Enabled should default to true when field is missing from JSON")
	}

	// Test 3: Settings file with enabled: false - should be false
	settingsContent = testSettingsDisabled
	if err := os.WriteFile(EntireSettingsFile, []byte(settingsContent), 0o644); err != nil {
		t.Fatalf("Failed to write settings file: %v", err)
	}

	settings, err = LoadEntireSettings()
	if err != nil {
		t.Fatalf("LoadEntireSettings() error = %v", err)
	}
	if settings.Enabled {
		t.Error("Enabled should be false when explicitly set to false")
	}

	// Test 4: Settings file with enabled: true - should be true
	settingsContent = testSettingsEnabled
	if err := os.WriteFile(EntireSettingsFile, []byte(settingsContent), 0o644); err != nil {
		t.Fatalf("Failed to write settings file: %v", err)
	}

	settings, err = LoadEntireSettings()
	if err != nil {
		t.Fatalf("LoadEntireSettings() error = %v", err)
	}
	if !settings.Enabled {
		t.Error("Enabled should be true when explicitly set to true")
	}
}

func TestSaveEntireSettings_PreservesEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Save settings with Enabled = false
	settings := &EntireSettings{
		Strategy: "manual-commit",
		Enabled:  false,
	}
	if err := SaveEntireSettings(settings); err != nil {
		t.Fatalf("SaveEntireSettings() error = %v", err)
	}

	// Load and verify
	loaded, err := LoadEntireSettings()
	if err != nil {
		t.Fatalf("LoadEntireSettings() error = %v", err)
	}
	if loaded.Enabled {
		t.Error("Enabled should be false after saving as false")
	}
}

func TestIsEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Test 1: No settings file - should return true (default)
	enabled, err := IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled() error = %v", err)
	}
	if !enabled {
		t.Error("IsEnabled() should return true when no settings file exists")
	}

	// Test 2: Settings with enabled: false - should return false
	settingsDir := filepath.Dir(EntireSettingsFile)
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("Failed to create settings dir: %v", err)
	}
	settingsContent := `{"enabled": false}`
	if err := os.WriteFile(EntireSettingsFile, []byte(settingsContent), 0o644); err != nil {
		t.Fatalf("Failed to write settings file: %v", err)
	}

	enabled, err = IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled() error = %v", err)
	}
	if enabled {
		t.Error("IsEnabled() should return false when disabled")
	}

	// Test 3: Settings with enabled: true - should return true
	settingsContent = `{"enabled": true}`
	if err := os.WriteFile(EntireSettingsFile, []byte(settingsContent), 0o644); err != nil {
		t.Fatalf("Failed to write settings file: %v", err)
	}

	enabled, err = IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled() error = %v", err)
	}
	if !enabled {
		t.Error("IsEnabled() should return true when enabled")
	}
}

// setupLocalOverrideTestDir creates a temp directory with .entire folder for testing
func setupLocalOverrideTestDir(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	settingsDir := filepath.Dir(EntireSettingsFile)
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("Failed to create settings dir: %v", err)
	}
}

func TestLoadEntireSettings_LocalOverridesStrategy(t *testing.T) {
	setupLocalOverrideTestDir(t)

	baseSettings := testSettingsEnabled
	if err := os.WriteFile(EntireSettingsFile, []byte(baseSettings), 0o644); err != nil {
		t.Fatalf("Failed to write settings file: %v", err)
	}

	localSettings := `{"strategy": "` + strategy.StrategyNameAutoCommit + `"}`
	if err := os.WriteFile(EntireSettingsLocalFile, []byte(localSettings), 0o644); err != nil {
		t.Fatalf("Failed to write local settings file: %v", err)
	}

	settings, err := LoadEntireSettings()
	if err != nil {
		t.Fatalf("LoadEntireSettings() error = %v", err)
	}
	if settings.Strategy != strategy.StrategyNameAutoCommit {
		t.Errorf("Strategy should be 'auto-commit' from local override, got %q", settings.Strategy)
	}
	if !settings.Enabled {
		t.Error("Enabled should remain true from base settings")
	}
}

func TestLoadEntireSettings_LocalOverridesEnabled(t *testing.T) {
	setupLocalOverrideTestDir(t)

	baseSettings := testSettingsEnabled
	if err := os.WriteFile(EntireSettingsFile, []byte(baseSettings), 0o644); err != nil {
		t.Fatalf("Failed to write settings file: %v", err)
	}

	localSettings := `{"enabled": false}`
	if err := os.WriteFile(EntireSettingsLocalFile, []byte(localSettings), 0o644); err != nil {
		t.Fatalf("Failed to write local settings file: %v", err)
	}

	settings, err := LoadEntireSettings()
	if err != nil {
		t.Fatalf("LoadEntireSettings() error = %v", err)
	}
	if settings.Enabled {
		t.Error("Enabled should be false from local override")
	}
	if settings.Strategy != strategy.StrategyNameManualCommit {
		t.Errorf("Strategy should remain 'manual-commit' from base settings, got %q", settings.Strategy)
	}
}

func TestLoadEntireSettings_LocalOverridesLocalDev(t *testing.T) {
	setupLocalOverrideTestDir(t)

	baseSettings := testSettingsStrategy
	if err := os.WriteFile(EntireSettingsFile, []byte(baseSettings), 0o644); err != nil {
		t.Fatalf("Failed to write settings file: %v", err)
	}

	localSettings := `{"local_dev": true}`
	if err := os.WriteFile(EntireSettingsLocalFile, []byte(localSettings), 0o644); err != nil {
		t.Fatalf("Failed to write local settings file: %v", err)
	}

	settings, err := LoadEntireSettings()
	if err != nil {
		t.Fatalf("LoadEntireSettings() error = %v", err)
	}
	if !settings.LocalDev {
		t.Error("LocalDev should be true from local override")
	}
}

func TestLoadEntireSettings_LocalMergesStrategyOptions(t *testing.T) {
	setupLocalOverrideTestDir(t)

	baseSettings := `{"strategy": "manual-commit", "strategy_options": {"key1": "value1", "key2": "value2"}}`
	if err := os.WriteFile(EntireSettingsFile, []byte(baseSettings), 0o644); err != nil {
		t.Fatalf("Failed to write settings file: %v", err)
	}

	localSettings := `{"strategy_options": {"key2": "overridden", "key3": "value3"}}`
	if err := os.WriteFile(EntireSettingsLocalFile, []byte(localSettings), 0o644); err != nil {
		t.Fatalf("Failed to write local settings file: %v", err)
	}

	settings, err := LoadEntireSettings()
	if err != nil {
		t.Fatalf("LoadEntireSettings() error = %v", err)
	}

	if settings.StrategyOptions["key1"] != "value1" {
		t.Errorf("key1 should remain 'value1', got %v", settings.StrategyOptions["key1"])
	}
	if settings.StrategyOptions["key2"] != "overridden" {
		t.Errorf("key2 should be 'overridden', got %v", settings.StrategyOptions["key2"])
	}
	if settings.StrategyOptions["key3"] != "value3" {
		t.Errorf("key3 should be 'value3', got %v", settings.StrategyOptions["key3"])
	}
}

func TestLoadEntireSettings_OnlyLocalFileExists(t *testing.T) {
	setupLocalOverrideTestDir(t)

	// No base settings file
	localSettings := `{"strategy": "auto-commit"}`
	if err := os.WriteFile(EntireSettingsLocalFile, []byte(localSettings), 0o644); err != nil {
		t.Fatalf("Failed to write local settings file: %v", err)
	}

	settings, err := LoadEntireSettings()
	if err != nil {
		t.Fatalf("LoadEntireSettings() error = %v", err)
	}
	if settings.Strategy != "auto-commit" {
		t.Errorf("Strategy should be 'auto-commit' from local file, got %q", settings.Strategy)
	}
	if !settings.Enabled {
		t.Error("Enabled should default to true")
	}
}

func TestLoadEntireSettings_NoLocalFileUsesBase(t *testing.T) {
	setupLocalOverrideTestDir(t)

	baseSettings := `{"strategy": "manual-commit", "enabled": true}`
	if err := os.WriteFile(EntireSettingsFile, []byte(baseSettings), 0o644); err != nil {
		t.Fatalf("Failed to write settings file: %v", err)
	}

	settings, err := LoadEntireSettings()
	if err != nil {
		t.Fatalf("LoadEntireSettings() error = %v", err)
	}
	if settings.Strategy != "manual-commit" {
		t.Errorf("Strategy should be 'shadow' from base settings, got %q", settings.Strategy)
	}
}

func TestLoadEntireSettings_EmptyStrategyInLocalDoesNotOverride(t *testing.T) {
	setupLocalOverrideTestDir(t)

	baseSettings := testSettingsStrategy
	if err := os.WriteFile(EntireSettingsFile, []byte(baseSettings), 0o644); err != nil {
		t.Fatalf("Failed to write settings file: %v", err)
	}

	localSettings := `{"strategy": ""}`
	if err := os.WriteFile(EntireSettingsLocalFile, []byte(localSettings), 0o644); err != nil {
		t.Fatalf("Failed to write local settings file: %v", err)
	}

	settings, err := LoadEntireSettings()
	if err != nil {
		t.Fatalf("LoadEntireSettings() error = %v", err)
	}
	if settings.Strategy != "manual-commit" {
		t.Errorf("Strategy should remain 'shadow', got %q", settings.Strategy)
	}
}

func TestLoadEntireSettings_NeitherFileExistsReturnsDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	settings, err := LoadEntireSettings()
	if err != nil {
		t.Fatalf("LoadEntireSettings() error = %v", err)
	}
	if settings.Strategy != strategy.DefaultStrategyName {
		t.Errorf("Strategy should be default %q, got %q", strategy.DefaultStrategyName, settings.Strategy)
	}
	if !settings.Enabled {
		t.Error("Enabled should default to true")
	}
}

func TestGetAgent_NoSettingsFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Create .claude directory to allow detection
	if err := os.MkdirAll(".claude", 0o755); err != nil {
		t.Fatalf("Failed to create .claude dir: %v", err)
	}

	ag, err := GetAgent()
	if err != nil {
		t.Fatalf("GetAgent() error = %v", err)
	}
	if ag == nil {
		t.Fatal("GetAgent() returned nil agent")
	}
	// With .claude directory present, should detect Claude Code
	if ag.Name() != agent.AgentNameClaudeCode {
		t.Errorf("Expected claude-code agent, got %q", ag.Name())
	}
}

func TestGetAgent_ExplicitAgent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	settingsDir := filepath.Dir(EntireSettingsFile)
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("Failed to create settings dir: %v", err)
	}

	settingsContent := `{"strategy": "manual-commit", "agent": "claude-code"}`
	if err := os.WriteFile(EntireSettingsFile, []byte(settingsContent), 0o644); err != nil {
		t.Fatalf("Failed to write settings file: %v", err)
	}

	ag, err := GetAgent()
	if err != nil {
		t.Fatalf("GetAgent() error = %v", err)
	}
	if ag.Name() != agent.AgentNameClaudeCode {
		t.Errorf("Expected claude-code agent, got %q", ag.Name())
	}
}

func TestGetAgent_AutoDetectDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	settingsDir := filepath.Dir(EntireSettingsFile)
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("Failed to create settings dir: %v", err)
	}

	// Create .claude directory but disable auto-detect
	if err := os.MkdirAll(".claude", 0o755); err != nil {
		t.Fatalf("Failed to create .claude dir: %v", err)
	}

	settingsContent := `{"strategy": "manual-commit", "agent_auto_detect": false}`
	if err := os.WriteFile(EntireSettingsFile, []byte(settingsContent), 0o644); err != nil {
		t.Fatalf("Failed to write settings file: %v", err)
	}

	ag, err := GetAgent()
	if err != nil {
		t.Fatalf("GetAgent() error = %v", err)
	}
	// Should fall back to default when auto-detect is disabled and no explicit agent
	if ag.Name() != agent.DefaultAgentName {
		t.Errorf("Expected default agent %q, got %q", agent.DefaultAgentName, ag.Name())
	}
}

func TestGetAgentOptions_ReturnsOptions(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	settingsDir := filepath.Dir(EntireSettingsFile)
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("Failed to create settings dir: %v", err)
	}

	settingsContent := `{
		"strategy": "manual-commit",
		"agent_options": {
			"claude-code": {
				"ignore_untracked": true
			}
		}
	}`
	if err := os.WriteFile(EntireSettingsFile, []byte(settingsContent), 0o644); err != nil {
		t.Fatalf("Failed to write settings file: %v", err)
	}

	opts := GetAgentOptions("claude-code")
	if opts == nil {
		t.Fatal("GetAgentOptions() returned nil")
	}
	if v, ok := opts["ignore_untracked"]; !ok || v != true {
		t.Errorf("Expected ignore_untracked=true, got %v", v)
	}
}

func TestGetAgentOptions_ReturnsNilForUnknownAgent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	settingsDir := filepath.Dir(EntireSettingsFile)
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("Failed to create settings dir: %v", err)
	}

	settingsContent := `{
		"strategy": "manual-commit",
		"agent_options": {
			"claude-code": {
				"ignore_untracked": true
			}
		}
	}`
	if err := os.WriteFile(EntireSettingsFile, []byte(settingsContent), 0o644); err != nil {
		t.Fatalf("Failed to write settings file: %v", err)
	}

	opts := GetAgentOptions("unknown-agent")
	if opts != nil {
		t.Error("GetAgentOptions() should return nil for unknown agent")
	}
}

func TestGetAgentOptions_ReturnsNilWhenNoSettings(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	opts := GetAgentOptions("claude-code")
	if opts != nil {
		t.Error("GetAgentOptions() should return nil when no settings file")
	}
}

func TestLoadEntireSettings_AgentFields(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	settingsDir := filepath.Dir(EntireSettingsFile)
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("Failed to create settings dir: %v", err)
	}

	settingsContent := `{
		"strategy": "manual-commit",
		"agent": "claude-code",
		"agent_auto_detect": false,
		"agent_options": {
			"claude-code": {"option1": "value1"}
		}
	}`
	if err := os.WriteFile(EntireSettingsFile, []byte(settingsContent), 0o644); err != nil {
		t.Fatalf("Failed to write settings file: %v", err)
	}

	settings, err := LoadEntireSettings()
	if err != nil {
		t.Fatalf("LoadEntireSettings() error = %v", err)
	}

	if settings.Agent != "claude-code" {
		t.Errorf("Agent should be 'claude-code', got %q", settings.Agent)
	}
	if settings.AgentAutoDetect == nil || *settings.AgentAutoDetect {
		t.Error("AgentAutoDetect should be false")
	}
	if settings.AgentOptions == nil {
		t.Fatal("AgentOptions should not be nil")
	}
	if _, ok := settings.AgentOptions["claude-code"]; !ok {
		t.Error("AgentOptions should have claude-code entry")
	}
}

func TestLoadEntireSettings_LocalOverridesAgent(t *testing.T) {
	setupLocalOverrideTestDir(t)

	baseSettings := `{"strategy": "manual-commit", "agent": "cursor"}`
	if err := os.WriteFile(EntireSettingsFile, []byte(baseSettings), 0o644); err != nil {
		t.Fatalf("Failed to write settings file: %v", err)
	}

	localSettings := `{"agent": "claude-code"}`
	if err := os.WriteFile(EntireSettingsLocalFile, []byte(localSettings), 0o644); err != nil {
		t.Fatalf("Failed to write local settings file: %v", err)
	}

	settings, err := LoadEntireSettings()
	if err != nil {
		t.Fatalf("LoadEntireSettings() error = %v", err)
	}
	if settings.Agent != "claude-code" {
		t.Errorf("Agent should be 'claude-code' from local override, got %q", settings.Agent)
	}
}
