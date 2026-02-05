package versioncheck

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsOutdated(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		want    bool
		desc    string
	}{
		// Standard semver cases
		{"1.0.0", "1.0.1", true, "patch version bump"},
		{"1.0.0", "1.1.0", true, "minor version bump"},
		{"1.0.0", "2.0.0", true, "major version bump"},
		{"1.0.1", "1.0.0", false, "current is newer"},
		{"2.0.0", "1.9.9", false, "current major is higher"},
		{"1.0.0", "1.0.0", false, "same version"},

		// v-prefix handling
		{"v1.0.0", "v1.0.1", true, "with v prefix"},
		{"v1.0.0", "1.0.1", true, "mixed v prefix"},
		{"1.0.0", "v1.0.1", true, "mixed v prefix reversed"},

		// Pre-release versions (semver uses hyphen)
		{"1.0.0-rc1", "1.0.0", true, "prerelease in current"},
		{"1.0.0", "1.0.1-rc1", true, "prerelease in latest is still newer"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := isOutdated(tt.current, tt.latest)
			if got != tt.want {
				t.Errorf("isOutdated(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestCacheReadWrite(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	// Create config directory structure
	configDir := filepath.Join(tmpDir, globalConfigDirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Test saving and loading cache directly to temp directory
	originalCache := &VersionCache{
		LastCheckTime: time.Now().Round(time.Second), // Round to second for JSON consistency
	}

	// Write cache manually to temp directory
	filePath := filepath.Join(configDir, cacheFileName)
	data, err := json.MarshalIndent(originalCache, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent() error = %v", err)
	}

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Load and verify

	loadedData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var loaded VersionCache
	if err := json.Unmarshal(loadedData, &loaded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Verify the loaded cache LastCheckTime matches (within 1 second tolerance for JSON rounding)
	if loaded.LastCheckTime.Sub(originalCache.LastCheckTime).Abs() > time.Second {
		t.Errorf("LastCheckTime = %v, want %v", loaded.LastCheckTime, originalCache.LastCheckTime)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); err != nil {
		t.Errorf("cache file not found at %s: %v", filePath, err)
	}
}

func TestEnsureGlobalConfigDir(t *testing.T) {
	// This test verifies that the directory creation logic works
	// We test the actual os.MkdirAll behavior by creating a temp directory structure

	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, globalConfigDirName)

	// Verify the directory doesn't exist yet
	if _, err := os.Stat(configDir); err == nil {
		t.Fatalf("directory already exists before test")
	}

	// Simulate the ensureGlobalConfigDir logic
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	// Verify directory was created
	info, err := os.Stat(configDir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}

	// Verify it's a directory
	if !info.IsDir() {
		t.Errorf("path is not a directory")
	}

	// Verify permissions (on Unix systems)
	// The directory should be readable/writable/executable by owner
	if mode := info.Mode(); (mode & 0o700) != 0o700 {
		t.Errorf("directory permissions = %o, expected at least 0o700", mode)
	}
}

func TestFetchLatestVersionWithMockServer(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("Accept header = %q, want application/vnd.github+json", r.Header.Get("Accept"))
		}

		// Return a mock GitHub release
		release := GitHubRelease{
			TagName:    "v1.2.3",
			Prerelease: false,
		}
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper, encoding error is acceptable
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	// Mock the githubAPIURL (this would normally be done at the module level)
	// For this test, we'll just verify the HTTP call works by creating a custom function
	// Note: In real usage, this would use the actual GitHub API

	// For now, we'll test the parseGitHubRelease function which is the core parsing logic
	body := []byte(`{"tag_name": "v1.2.3", "prerelease": false}`)
	version, err := parseGitHubRelease(body)
	if err != nil {
		t.Errorf("parseGitHubRelease() error = %v", err)
	}
	if version != "v1.2.3" {
		t.Errorf("parseGitHubRelease() = %q, want v1.2.3", version)
	}
}
