//go:build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"

	"entire.io/cli/cmd/entire/cli/strategy"
)

// TestConcurrentSessionWarning_BlocksFirstPrompt verifies that when a user starts
// a new Claude session while another session has uncommitted changes (checkpoints),
// the first prompt is blocked with a continue:false JSON response.
func TestConcurrentSessionWarning_BlocksFirstPrompt(t *testing.T) {
	env := NewTestEnv(t)
	defer env.Cleanup()

	env.InitRepo()
	env.WriteFile("README.md", "# Test")
	env.GitAdd("README.md")
	env.GitCommit("Initial commit")
	env.GitCheckoutNewBranch("feature/test")
	env.InitEntire(strategy.StrategyNameManualCommit)

	// Start session A and create a checkpoint
	sessionA := env.NewSession()
	if err := env.SimulateUserPromptSubmit(sessionA.ID); err != nil {
		t.Fatalf("SimulateUserPromptSubmit (sessionA) failed: %v", err)
	}

	env.WriteFile("file.txt", "content from session A")
	sessionA.CreateTranscript("Add file", []FileChange{{Path: "file.txt", Content: "content from session A"}})
	if err := env.SimulateStop(sessionA.ID, sessionA.TranscriptPath); err != nil {
		t.Fatalf("SimulateStop (sessionA) failed: %v", err)
	}

	// Verify session A has checkpoints
	stateA, err := env.GetSessionState(sessionA.ID)
	if err != nil {
		t.Fatalf("GetSessionState (sessionA) failed: %v", err)
	}
	if stateA == nil {
		t.Fatal("Session A state should exist after Stop hook")
	}
	if stateA.CheckpointCount == 0 {
		t.Fatal("Session A should have at least 1 checkpoint")
	}
	t.Logf("Session A has %d checkpoint(s)", stateA.CheckpointCount)

	// Start session B - first prompt should be blocked
	sessionB := env.NewSession()
	output := env.SimulateUserPromptSubmitWithOutput(sessionB.ID)

	// The hook should succeed (exit code 0) but output JSON with continue:false
	if output.Err != nil {
		t.Fatalf("Hook should succeed but output continue:false, got error: %v\nStderr: %s", output.Err, output.Stderr)
	}

	// Parse the JSON response
	var response struct {
		Continue   bool   `json:"continue"`
		StopReason string `json:"stopReason"`
	}
	if err := json.Unmarshal(output.Stdout, &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v\nStdout: %s", err, output.Stdout)
	}

	// Verify continue is false
	if response.Continue {
		t.Error("Expected continue:false in JSON response")
	}

	// Verify stop reason contains expected message
	expectedMessage := "Another session is active"
	if !strings.Contains(response.StopReason, expectedMessage) {
		t.Errorf("StopReason should contain %q, got: %s", expectedMessage, response.StopReason)
	}

	t.Logf("Received expected blocking response: %s", output.Stdout)
}

// TestConcurrentSessionWarning_SetsWarningFlag verifies that after the first prompt
// is blocked, the session state has ConcurrentWarningShown set to true.
func TestConcurrentSessionWarning_SetsWarningFlag(t *testing.T) {
	env := NewTestEnv(t)
	defer env.Cleanup()

	env.InitRepo()
	env.WriteFile("README.md", "# Test")
	env.GitAdd("README.md")
	env.GitCommit("Initial commit")
	env.GitCheckoutNewBranch("feature/test")
	env.InitEntire(strategy.StrategyNameManualCommit)

	// Start session A and create a checkpoint
	sessionA := env.NewSession()
	if err := env.SimulateUserPromptSubmit(sessionA.ID); err != nil {
		t.Fatalf("SimulateUserPromptSubmit (sessionA) failed: %v", err)
	}

	env.WriteFile("file.txt", "content")
	sessionA.CreateTranscript("Add file", []FileChange{{Path: "file.txt", Content: "content"}})
	if err := env.SimulateStop(sessionA.ID, sessionA.TranscriptPath); err != nil {
		t.Fatalf("SimulateStop (sessionA) failed: %v", err)
	}

	// Start session B - first prompt is blocked
	sessionB := env.NewSession()
	_ = env.SimulateUserPromptSubmitWithOutput(sessionB.ID)

	// Verify session B state has ConcurrentWarningShown flag
	stateB, err := env.GetSessionState(sessionB.ID)
	if err != nil {
		t.Fatalf("GetSessionState (sessionB) failed: %v", err)
	}
	if stateB == nil {
		t.Fatal("Session B state should exist after blocked prompt")
	}
	if !stateB.ConcurrentWarningShown {
		t.Error("Session B state should have ConcurrentWarningShown=true")
	}

	t.Logf("Session B state: ConcurrentWarningShown=%v", stateB.ConcurrentWarningShown)
}

// TestConcurrentSessionWarning_SubsequentPromptsSucceed verifies that after the
// warning is shown, subsequent prompts in the same session proceed normally.
func TestConcurrentSessionWarning_SubsequentPromptsSucceed(t *testing.T) {
	env := NewTestEnv(t)
	defer env.Cleanup()

	env.InitRepo()
	env.WriteFile("README.md", "# Test")
	env.GitAdd("README.md")
	env.GitCommit("Initial commit")
	env.GitCheckoutNewBranch("feature/test")
	env.InitEntire(strategy.StrategyNameManualCommit)

	// Start session A and create a checkpoint
	sessionA := env.NewSession()
	if err := env.SimulateUserPromptSubmit(sessionA.ID); err != nil {
		t.Fatalf("SimulateUserPromptSubmit (sessionA) failed: %v", err)
	}

	env.WriteFile("file.txt", "content")
	sessionA.CreateTranscript("Add file", []FileChange{{Path: "file.txt", Content: "content"}})
	if err := env.SimulateStop(sessionA.ID, sessionA.TranscriptPath); err != nil {
		t.Fatalf("SimulateStop (sessionA) failed: %v", err)
	}

	// Start session B - first prompt is blocked
	sessionB := env.NewSession()
	output1 := env.SimulateUserPromptSubmitWithOutput(sessionB.ID)

	// Verify first prompt was blocked
	var response1 struct {
		Continue bool `json:"continue"`
	}
	if err := json.Unmarshal(output1.Stdout, &response1); err != nil {
		t.Fatalf("Failed to parse first response: %v", err)
	}
	if response1.Continue {
		t.Fatal("First prompt should have been blocked")
	}
	t.Log("First prompt correctly blocked")

	// Second prompt in session B should PROCEED normally (both sessions capture checkpoints)
	// The warning was shown on first prompt, but subsequent prompts continue to capture state
	output2 := env.SimulateUserPromptSubmitWithOutput(sessionB.ID)

	// The hook should succeed
	if output2.Err != nil {
		t.Errorf("Second prompt should succeed, got error: %v", output2.Err)
	}

	// The hook should process normally (capture state)
	// Output should contain state capture info, not a blocking response
	if len(output2.Stdout) > 0 {
		// Check if it's a blocking JSON response (which it shouldn't be anymore after the first prompt)
		var blockResponse struct {
			Continue bool `json:"continue"`
		}
		if json.Unmarshal(output2.Stdout, &blockResponse) == nil && !blockResponse.Continue {
			t.Errorf("Second prompt should not be blocked after warning was shown, got: %s", output2.Stdout)
		}
	}

	// Warning flag should remain set (for tracking)
	stateB, _ := env.GetSessionState(sessionB.ID)
	if stateB == nil {
		t.Fatal("Session B state should exist")
	}
	if !stateB.ConcurrentWarningShown {
		t.Error("ConcurrentWarningShown should remain true after second prompt")
	}

	t.Log("Second prompt correctly processed (both sessions capture checkpoints)")
}

// TestConcurrentSessionWarning_NoWarningWithoutCheckpoints verifies that starting
// a new session does NOT trigger the warning if the existing session has no checkpoints.
func TestConcurrentSessionWarning_NoWarningWithoutCheckpoints(t *testing.T) {
	env := NewTestEnv(t)
	defer env.Cleanup()

	env.InitRepo()
	env.WriteFile("README.md", "# Test")
	env.GitAdd("README.md")
	env.GitCommit("Initial commit")
	env.GitCheckoutNewBranch("feature/test")
	env.InitEntire(strategy.StrategyNameManualCommit)

	// Start session A but do NOT create any checkpoints
	sessionA := env.NewSession()
	if err := env.SimulateUserPromptSubmit(sessionA.ID); err != nil {
		t.Fatalf("SimulateUserPromptSubmit (sessionA) failed: %v", err)
	}

	// Verify session A has no checkpoints
	stateA, err := env.GetSessionState(sessionA.ID)
	if err != nil {
		t.Fatalf("GetSessionState (sessionA) failed: %v", err)
	}
	if stateA == nil {
		t.Fatal("Session A state should exist after UserPromptSubmit")
	}
	if stateA.CheckpointCount != 0 {
		t.Fatalf("Session A should have 0 checkpoints, got %d", stateA.CheckpointCount)
	}

	// Start session B - should NOT be blocked since session A has no checkpoints
	sessionB := env.NewSession()
	output := env.SimulateUserPromptSubmitWithOutput(sessionB.ID)

	// Check if we got a blocking response
	if len(output.Stdout) > 0 {
		var response struct {
			Continue   bool   `json:"continue"`
			StopReason string `json:"stopReason,omitempty"`
		}
		if json.Unmarshal(output.Stdout, &response) == nil {
			if !response.Continue && strings.Contains(response.StopReason, "another active session") {
				t.Error("Should NOT show concurrent session warning when existing session has no checkpoints")
			}
		}
	}

	// Session B should proceed normally (or fail for other reasons, but not concurrent warning)
	stateB, _ := env.GetSessionState(sessionB.ID)
	if stateB != nil && stateB.ConcurrentWarningShown {
		t.Error("Session B should not have ConcurrentWarningShown set when session A has no checkpoints")
	}

	t.Log("No concurrent session warning shown when existing session has no checkpoints")
}
