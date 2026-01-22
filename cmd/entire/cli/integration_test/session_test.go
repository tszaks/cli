//go:build integration

package integration

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// Test session ID used across tests (format matches real session IDs)
const testSessionID1 = "2025-12-01-8f76b0e8-test-session-one"

func TestSessionList_EmptyRepo(t *testing.T) {
	t.Parallel()
	for _, strat := range AllStrategies() {
		strat := strat // capture for parallel
		t.Run(strat, func(t *testing.T) {
			t.Parallel()
			env := NewTestEnv(t)
			env.InitRepo()
			env.InitEntire(strat)

			// Create initial commit
			env.WriteFile("README.md", "# Test")
			env.GitAdd("README.md")
			env.GitCommit("Initial commit")

			// Run session list - should show no sessions
			output := env.RunCLI("session", "list")

			if !strings.Contains(output, "No sessions found") {
				t.Errorf("expected 'No sessions found', got: %s", output)
			}
		})
	}
}

func TestSessionCurrent_NoCurrentSession(t *testing.T) {
	t.Parallel()
	for _, strat := range AllStrategies() {
		strat := strat // capture for parallel
		t.Run(strat, func(t *testing.T) {
			t.Parallel()
			env := NewTestEnv(t)
			env.InitRepo()
			env.InitEntire(strat)

			// Create initial commit
			env.WriteFile("README.md", "# Test")
			env.GitAdd("README.md")
			env.GitCommit("Initial commit")

			// Run session current - should show no session
			output := env.RunCLI("session", "current")

			if !strings.Contains(output, "No current session set") {
				t.Errorf("expected 'No current session set', got: %s", output)
			}
		})
	}
}

func TestSessionResume_NotFound(t *testing.T) {
	t.Parallel()
	for _, strat := range AllStrategies() {
		strat := strat // capture for parallel
		t.Run(strat, func(t *testing.T) {
			t.Parallel()
			env := NewTestEnv(t)
			env.InitRepo()
			env.InitEntire(strat)

			// Create initial commit
			env.WriteFile("README.md", "# Test")
			env.GitAdd("README.md")
			env.GitCommit("Initial commit")

			// Try to resume non-existent session
			_, err := env.RunCLIWithError("session", "resume", "nonexistent-session")
			if err == nil {
				t.Error("expected error for non-existent session")
			}
		})
	}
}

func TestHooksClaudeCodeSessionStart(t *testing.T) {
	t.Parallel()
	for _, strat := range AllStrategies() {
		strat := strat // capture for parallel
		t.Run(strat, func(t *testing.T) {
			t.Parallel()
			env := NewTestEnv(t)
			env.InitRepo()
			env.InitEntire(strat)

			// Create initial commit
			env.WriteFile("README.md", "# Test")
			env.GitAdd("README.md")
			env.GitCommit("Initial commit")

			// Claude sends a raw session ID (UUID format)
			claudeSessionID := "hook-session-uuid-1234"

			// Run hooks claude-code session-start with JSON input (simulating Claude's SessionStart hook)
			output := env.RunCLIWithStdin(
				`{"session_id": "`+claudeSessionID+`", "transcript_path": "/tmp/transcript.jsonl"}`,
				"hooks", "claude-code", "session-start",
			)

			if !strings.Contains(output, "Current session set to") {
				t.Errorf("expected confirmation message, got: %s", output)
			}

			// Verify the output contains the Claude session ID with a date prefix
			// (unit tests verify the exact format, here we just check it was transformed)
			if !strings.Contains(output, claudeSessionID) {
				t.Errorf("expected output to contain %s, got: %s", claudeSessionID, output)
			}
			// Should have date prefix (not just the raw Claude session ID)
			datePattern := regexp.MustCompile(`\d{4}-\d{2}-\d{2}-` + regexp.QuoteMeta(claudeSessionID))
			if !datePattern.MatchString(output) {
				t.Errorf("expected output to contain date-prefixed session ID, got: %s", output)
			}

			// Verify session current shows the session ID
			// Note: It will say "not found in strategy" since no metadata dir exists yet,
			// but the important thing is the session ID is displayed correctly
			currentOutput := env.RunCLI("session", "current")
			if strings.Contains(currentOutput, "No current session set") {
				t.Errorf("session current should show a session, got: %s", currentOutput)
			}
			if !strings.Contains(currentOutput, claudeSessionID) {
				t.Errorf("session current should contain %s, got: %s", claudeSessionID, currentOutput)
			}
			// Verify the date prefix was added
			if !datePattern.MatchString(currentOutput) {
				t.Errorf("session current should show date-prefixed ID, got: %s", currentOutput)
			}
		})
	}
}

func TestHooksGeminiSessionStart(t *testing.T) {
	t.Parallel()
	for _, strat := range AllStrategies() {
		strat := strat // capture for parallel
		t.Run(strat, func(t *testing.T) {
			t.Parallel()
			env := NewTestEnv(t)
			env.InitRepo()
			env.InitEntireWithAgent(strat, "gemini")

			// Create initial commit
			env.WriteFile("README.md", "# Test")
			env.GitAdd("README.md")
			env.GitCommit("Initial commit")

			// Gemini sends a raw session ID
			geminiSessionID := "gemini-hook-session-uuid-5678"

			// Run hooks gemini session-start with JSON input (simulating Gemini's SessionStart hook)
			output := env.RunGeminiCLIWithStdin(
				`{"session_id": "`+geminiSessionID+`", "transcript_path": "/tmp/transcript.json"}`,
				"hooks", "gemini", "session-start",
			)

			if !strings.Contains(output, "Current session set to") {
				t.Errorf("expected confirmation message, got: %s", output)
			}

			// Verify the output contains the Gemini session ID with a date prefix
			if !strings.Contains(output, geminiSessionID) {
				t.Errorf("expected output to contain %s, got: %s", geminiSessionID, output)
			}
			// Should have date prefix (not just the raw Gemini session ID)
			datePattern := regexp.MustCompile(`\d{4}-\d{2}-\d{2}-` + regexp.QuoteMeta(geminiSessionID))
			if !datePattern.MatchString(output) {
				t.Errorf("expected output to contain date-prefixed session ID, got: %s", output)
			}

			// Verify session current shows the session ID
			currentOutput := env.RunCLI("session", "current")
			if strings.Contains(currentOutput, "No current session set") {
				t.Errorf("session current should show a session, got: %s", currentOutput)
			}
			if !strings.Contains(currentOutput, geminiSessionID) {
				t.Errorf("session current should contain %s, got: %s", geminiSessionID, currentOutput)
			}
			// Verify the date prefix was added
			if !datePattern.MatchString(currentOutput) {
				t.Errorf("session current should show date-prefixed ID, got: %s", currentOutput)
			}
		})
	}
}

// RunCLI runs the entire CLI with the given arguments and returns stdout.
func (env *TestEnv) RunCLI(args ...string) string {
	env.T.Helper()
	output, err := env.RunCLIWithError(args...)
	if err != nil {
		env.T.Fatalf("CLI command failed: %v\nArgs: %v\nOutput: %s", err, args, output)
	}
	return output
}

// RunCLIWithError runs the entire CLI and returns output and error.
func (env *TestEnv) RunCLIWithError(args ...string) (string, error) {
	env.T.Helper()

	// Run CLI using the shared binary
	cmd := exec.Command(getTestBinary(), args...)
	cmd.Dir = env.RepoDir
	cmd.Env = append(os.Environ(),
		"ENTIRE_TEST_CLAUDE_PROJECT_DIR="+env.ClaudeProjectDir,
	)

	output, err := cmd.CombinedOutput()
	return string(output), err
}

// RunCLIWithStdin runs the CLI with stdin input.
func (env *TestEnv) RunCLIWithStdin(stdin string, args ...string) string {
	env.T.Helper()

	// Run CLI with stdin using the shared binary
	cmd := exec.Command(getTestBinary(), args...)
	cmd.Dir = env.RepoDir
	cmd.Env = append(os.Environ(),
		"ENTIRE_TEST_CLAUDE_PROJECT_DIR="+env.ClaudeProjectDir,
	)
	cmd.Stdin = strings.NewReader(stdin)

	output, err := cmd.CombinedOutput()
	if err != nil {
		env.T.Fatalf("CLI command failed: %v\nArgs: %v\nOutput: %s", err, args, output)
	}
	return string(output)
}

// RunGeminiCLIWithStdin runs the CLI with stdin input for Gemini hooks.
func (env *TestEnv) RunGeminiCLIWithStdin(stdin string, args ...string) string {
	env.T.Helper()

	// Run CLI with stdin using the shared binary
	cmd := exec.Command(getTestBinary(), args...)
	cmd.Dir = env.RepoDir
	cmd.Env = append(os.Environ(),
		"ENTIRE_TEST_GEMINI_PROJECT_DIR="+env.GeminiProjectDir,
	)
	cmd.Stdin = strings.NewReader(stdin)

	output, err := cmd.CombinedOutput()
	if err != nil {
		env.T.Fatalf("CLI command failed: %v\nArgs: %v\nOutput: %s", err, args, output)
	}
	return string(output)
}
