//go:build integration

package integration

import (
	"testing"

	"github.com/entireio/cli/cmd/entire/cli/session"
	"github.com/entireio/cli/cmd/entire/cli/strategy"
)

// TestCarryForward_NotCondensedIntoUnrelatedCommit verifies that a session with
// carry-forward files is NOT condensed into a commit that doesn't touch those files.
//
// This is a regression test for the bug where sessions with carry-forward files
// would be re-condensed into every subsequent commit indefinitely. The root cause
// was that HandleCondense and HandleCondenseIfFilesTouched only checked hasNew
// (shadow branch has content) but didn't verify that the committed files actually
// overlapped with the session's FilesTouched.
//
// Scenario:
// 1. Session 1 creates file1.txt and file2.txt
// 2. User commits ONLY file1.txt (partial commit)
// 3. Session 1 gets carry-forward: FilesTouched = ["file2.txt"]
// 4. Session 1 ends
// 5. New session 2 creates file3.txt (unrelated to session 1)
// 6. Session 2 commits file3.txt
// 7. Verify: Session 1 was NOT condensed (FilesTouched preserved, StepCount unchanged)
func TestCarryForward_NotCondensedIntoUnrelatedCommit(t *testing.T) {
	t.Parallel()
	env := NewTestEnv(t)
	defer env.Cleanup()

	// ========================================
	// Setup
	// ========================================
	env.InitRepo()
	env.WriteFile("README.md", "# Test Repository")
	env.GitAdd("README.md")
	env.GitCommit("Initial commit")
	env.GitCheckoutNewBranch("feature/carry-forward-test")
	env.InitEntire(strategy.StrategyNameManualCommit)

	// ========================================
	// Phase 1: Session 1 creates multiple files
	// ========================================
	t.Log("Phase 1: Session 1 creates file1.txt and file2.txt")

	session1 := env.NewSession()
	if err := env.SimulateUserPromptSubmit(session1.ID); err != nil {
		t.Fatalf("SimulateUserPromptSubmit for session1 failed: %v", err)
	}

	// Create two files
	env.WriteFile("file1.txt", "content from session 1 - file 1")
	env.WriteFile("file2.txt", "content from session 1 - file 2")
	session1.CreateTranscript(
		"Create file1 and file2",
		[]FileChange{
			{Path: "file1.txt", Content: "content from session 1 - file 1"},
			{Path: "file2.txt", Content: "content from session 1 - file 2"},
		},
	)
	if err := env.SimulateStop(session1.ID, session1.TranscriptPath); err != nil {
		t.Fatalf("SimulateStop for session1 failed: %v", err)
	}

	// Verify session1 is IDLE with both files in FilesTouched
	state1, err := env.GetSessionState(session1.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session1 failed: %v", err)
	}
	if state1.Phase != session.PhaseIdle {
		t.Fatalf("Expected session1 to be IDLE, got %s", state1.Phase)
	}
	t.Logf("Session1 FilesTouched before partial commit: %v", state1.FilesTouched)

	// ========================================
	// Phase 2: Partial commit - only file1.txt
	// ========================================
	t.Log("Phase 2: Committing only file1.txt (partial commit)")

	env.GitAdd("file1.txt")
	// Note: file2.txt is NOT staged
	env.GitCommitWithShadowHooks("Partial commit: only file1", "file1.txt")

	// Verify carry-forward: file2.txt should remain in FilesTouched
	state1, err = env.GetSessionState(session1.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session1 after partial commit failed: %v", err)
	}
	t.Logf("Session1 FilesTouched after partial commit: %v", state1.FilesTouched)

	// file2.txt should be carried forward (not committed yet)
	hasFile2 := false
	for _, f := range state1.FilesTouched {
		if f == "file2.txt" {
			hasFile2 = true
			break
		}
	}
	if !hasFile2 {
		t.Fatalf("Expected file2.txt to be carried forward in FilesTouched, got: %v", state1.FilesTouched)
	}

	// Record state for later comparison
	session1StepCountAfterPartial := state1.StepCount
	session1BaseCommitAfterPartial := state1.BaseCommit

	// ========================================
	// Phase 3: End session 1 (simulating user closes agent)
	// ========================================
	t.Log("Phase 3: Ending session 1")

	state1.Phase = session.PhaseEnded
	if err := env.WriteSessionState(session1.ID, state1); err != nil {
		t.Fatalf("WriteSessionState for session1 failed: %v", err)
	}

	// ========================================
	// Phase 4: New session 2 creates unrelated file
	// ========================================
	t.Log("Phase 4: Session 2 creates file3.txt (unrelated to session 1)")

	session2 := env.NewSession()
	if err := env.SimulateUserPromptSubmit(session2.ID); err != nil {
		t.Fatalf("SimulateUserPromptSubmit for session2 failed: %v", err)
	}

	env.WriteFile("file3.txt", "content from session 2")
	session2.CreateTranscript(
		"Create file3",
		[]FileChange{{Path: "file3.txt", Content: "content from session 2"}},
	)
	if err := env.SimulateStop(session2.ID, session2.TranscriptPath); err != nil {
		t.Fatalf("SimulateStop for session2 failed: %v", err)
	}

	// Set session2 to ACTIVE (simulating mid-turn commit)
	state2, err := env.GetSessionState(session2.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session2 failed: %v", err)
	}
	state2.Phase = session.PhaseActive
	if err := env.WriteSessionState(session2.ID, state2); err != nil {
		t.Fatalf("WriteSessionState for session2 failed: %v", err)
	}

	// ========================================
	// Phase 5: Commit file3.txt (unrelated to session 1's files)
	// ========================================
	t.Log("Phase 5: Committing file3.txt from session 2")

	env.GitAdd("file3.txt")
	env.GitCommitWithShadowHooks("Add file3 from session 2", "file3.txt")

	finalHead := env.GetHeadHash()
	t.Logf("Final HEAD: %s", finalHead[:7])

	// ========================================
	// Phase 6: Verify session 1 was NOT condensed
	// ========================================
	t.Log("Phase 6: Verifying session 1 was NOT condensed into unrelated commit")

	state1After, err := env.GetSessionState(session1.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session1 after session2 commit failed: %v", err)
	}

	// StepCount should be unchanged (condensation resets it)
	if state1After.StepCount != session1StepCountAfterPartial {
		t.Errorf("Session 1 StepCount changed! Expected %d, got %d (was incorrectly condensed)",
			session1StepCountAfterPartial, state1After.StepCount)
	}

	// BaseCommit should be unchanged (ENDED sessions don't get BaseCommit updated)
	if state1After.BaseCommit != session1BaseCommitAfterPartial {
		t.Errorf("Session 1 BaseCommit changed! Expected %s, got %s",
			session1BaseCommitAfterPartial[:7], state1After.BaseCommit[:7])
	}

	// FilesTouched should still have file2.txt (not cleared by condensation)
	hasFile2After := false
	for _, f := range state1After.FilesTouched {
		if f == "file2.txt" {
			hasFile2After = true
			break
		}
	}
	if !hasFile2After {
		t.Errorf("Session 1 FilesTouched was cleared! Expected file2.txt to remain, got: %v",
			state1After.FilesTouched)
	}

	// Phase should still be ENDED
	if state1After.Phase != session.PhaseEnded {
		t.Errorf("Session 1 phase changed! Expected ENDED, got %s", state1After.Phase)
	}

	// ========================================
	// Phase 7: Verify session 2 WAS condensed
	// ========================================
	t.Log("Phase 7: Verifying session 2 WAS condensed")

	state2After, err := env.GetSessionState(session2.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session2 after commit failed: %v", err)
	}

	// Session 2's BaseCommit should be updated to new HEAD
	if state2After.BaseCommit != finalHead {
		t.Errorf("Session 2 BaseCommit should be updated to new HEAD. Expected %s, got %s",
			finalHead[:7], state2After.BaseCommit[:7])
	}

	t.Log("Test completed successfully")
}

// TestCarryForward_NotCondensedIntoMultipleUnrelatedCommits verifies that a session
// with carry-forward files is NOT condensed into MULTIPLE subsequent unrelated commits.
//
// This is a regression test for the "repeat on every commit" bug where sessions
// with carry-forward files would be re-condensed into every subsequent commit
// indefinitely, polluting checkpoints.
//
// Scenario:
// 1. Session 1 creates file1.txt and file2.txt
// 2. User commits only file1.txt (partial commit)
// 3. Session 1 gets carry-forward: FilesTouched = ["file2.txt"]
// 4. Make 3 unrelated commits (file3.txt, file4.txt, file5.txt)
// 5. Verify: Session 1 was NOT condensed into ANY of those commits
// 6. Finally commit file2.txt
// 7. Verify: Session 1 IS condensed (carry-forward consumed)
func TestCarryForward_NotCondensedIntoMultipleUnrelatedCommits(t *testing.T) {
	t.Parallel()
	env := NewTestEnv(t)
	defer env.Cleanup()

	// ========================================
	// Setup
	// ========================================
	env.InitRepo()
	env.WriteFile("README.md", "# Test Repository")
	env.GitAdd("README.md")
	env.GitCommit("Initial commit")
	env.GitCheckoutNewBranch("feature/repeat-condensation-test")
	env.InitEntire(strategy.StrategyNameManualCommit)

	// ========================================
	// Phase 1: Session 1 creates multiple files
	// ========================================
	t.Log("Phase 1: Session 1 creates file1.txt and file2.txt")

	session1 := env.NewSession()
	if err := env.SimulateUserPromptSubmit(session1.ID); err != nil {
		t.Fatalf("SimulateUserPromptSubmit for session1 failed: %v", err)
	}

	env.WriteFile("file1.txt", "content from session 1 - file 1")
	env.WriteFile("file2.txt", "content from session 1 - file 2")
	session1.CreateTranscript(
		"Create file1 and file2",
		[]FileChange{
			{Path: "file1.txt", Content: "content from session 1 - file 1"},
			{Path: "file2.txt", Content: "content from session 1 - file 2"},
		},
	)
	if err := env.SimulateStop(session1.ID, session1.TranscriptPath); err != nil {
		t.Fatalf("SimulateStop for session1 failed: %v", err)
	}

	// ========================================
	// Phase 2: Partial commit - only file1.txt
	// ========================================
	t.Log("Phase 2: Committing only file1.txt (partial commit)")

	env.GitAdd("file1.txt")
	env.GitCommitWithShadowHooks("Partial commit: only file1", "file1.txt")

	// Verify carry-forward
	state1, err := env.GetSessionState(session1.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session1 after partial commit failed: %v", err)
	}
	t.Logf("Session1 FilesTouched after partial commit: %v", state1.FilesTouched)

	// End session 1 and force hasNew=true by setting CheckpointTranscriptStart=0
	// This simulates the bug scenario where sessionHasNewContent returns true
	// because the transcript has "grown" since the (reset) checkpoint start.
	state1.Phase = session.PhaseEnded
	state1.CheckpointTranscriptStart = 0 // Force hasNew=true for subsequent commits
	if err := env.WriteSessionState(session1.ID, state1); err != nil {
		t.Fatalf("WriteSessionState for session1 failed: %v", err)
	}

	// Record initial state for comparison
	session1InitialStepCount := state1.StepCount

	// ========================================
	// Phase 3: Make multiple unrelated commits
	// ========================================
	unrelatedFiles := []string{"file3.txt", "file4.txt", "file5.txt"}

	for i, fileName := range unrelatedFiles {
		t.Logf("Phase 3.%d: Making unrelated commit with %s", i+1, fileName)

		env.WriteFile(fileName, "unrelated content "+fileName)
		env.GitAdd(fileName)
		env.GitCommitWithShadowHooks("Add "+fileName, fileName)

		// Verify session 1 was NOT condensed after each commit
		state1, err = env.GetSessionState(session1.ID)
		if err != nil {
			t.Fatalf("GetSessionState for session1 after commit %d failed: %v", i+1, err)
		}

		if state1.StepCount != session1InitialStepCount {
			t.Errorf("After commit %d (%s): Session 1 StepCount changed from %d to %d (incorrectly condensed!)",
				i+1, fileName, session1InitialStepCount, state1.StepCount)
		}

		// FilesTouched should still have file2.txt
		hasFile2 := false
		for _, f := range state1.FilesTouched {
			if f == "file2.txt" {
				hasFile2 = true
				break
			}
		}
		if !hasFile2 {
			t.Errorf("After commit %d (%s): Session 1 FilesTouched was cleared! Expected file2.txt, got: %v",
				i+1, fileName, state1.FilesTouched)
		}

		t.Logf("  -> Session 1 correctly NOT condensed (StepCount=%d, FilesTouched=%v)",
			state1.StepCount, state1.FilesTouched)
	}

	// ========================================
	// Phase 4: Finally commit file2.txt (the carry-forward file)
	// ========================================
	t.Log("Phase 4: Committing file2.txt (the carry-forward file)")

	env.GitAdd("file2.txt")
	env.GitCommitWithShadowHooks("Add file2 (carry-forward)", "file2.txt")

	// ========================================
	// Phase 5: Verify session 1 WAS condensed this time
	// ========================================
	t.Log("Phase 5: Verifying session 1 WAS condensed when its file was committed")

	state1, err = env.GetSessionState(session1.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session1 after file2 commit failed: %v", err)
	}

	// StepCount should be reset (condensation happened)
	if state1.StepCount != 0 {
		t.Errorf("Session 1 StepCount should be 0 after condensation, got %d", state1.StepCount)
	}

	// FilesTouched should be empty (carry-forward consumed)
	if len(state1.FilesTouched) != 0 {
		t.Errorf("Session 1 FilesTouched should be empty after condensation, got: %v", state1.FilesTouched)
	}

	t.Log("Test completed successfully - session correctly condensed only when its file was committed")
}

// TestCarryForward_NewSessionCommitDoesNotCondenseOldSession verifies that when
// an old session has carry-forward files and a NEW session commits unrelated files,
// the old session is NOT condensed into the new session's commit.
//
// This tests the interaction between multiple sessions where:
// 1. Old session has carry-forward files (file2.txt)
// 2. New session creates and commits different files (file6.txt)
// 3. Old session should remain untouched
func TestCarryForward_NewSessionCommitDoesNotCondenseOldSession(t *testing.T) {
	t.Parallel()
	env := NewTestEnv(t)
	defer env.Cleanup()

	// ========================================
	// Setup
	// ========================================
	env.InitRepo()
	env.WriteFile("README.md", "# Test Repository")
	env.GitAdd("README.md")
	env.GitCommit("Initial commit")
	env.GitCheckoutNewBranch("feature/multi-session-carry-forward")
	env.InitEntire(strategy.StrategyNameManualCommit)

	// ========================================
	// Phase 1: Session 1 creates files, partial commit, ends with carry-forward
	// ========================================
	t.Log("Phase 1: Session 1 creates file1.txt and file2.txt")

	session1 := env.NewSession()
	if err := env.SimulateUserPromptSubmit(session1.ID); err != nil {
		t.Fatalf("SimulateUserPromptSubmit for session1 failed: %v", err)
	}

	env.WriteFile("file1.txt", "content from session 1 - file 1")
	env.WriteFile("file2.txt", "content from session 1 - file 2")
	session1.CreateTranscript(
		"Create file1 and file2",
		[]FileChange{
			{Path: "file1.txt", Content: "content from session 1 - file 1"},
			{Path: "file2.txt", Content: "content from session 1 - file 2"},
		},
	)
	if err := env.SimulateStop(session1.ID, session1.TranscriptPath); err != nil {
		t.Fatalf("SimulateStop for session1 failed: %v", err)
	}

	// Partial commit - only file1.txt
	t.Log("Phase 1b: Partial commit - only file1.txt")
	env.GitAdd("file1.txt")
	env.GitCommitWithShadowHooks("Partial commit: only file1", "file1.txt")

	// End session 1
	state1, err := env.GetSessionState(session1.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session1 failed: %v", err)
	}
	state1.Phase = session.PhaseEnded
	if err := env.WriteSessionState(session1.ID, state1); err != nil {
		t.Fatalf("WriteSessionState for session1 failed: %v", err)
	}

	// Verify carry-forward
	state1, err = env.GetSessionState(session1.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session1 failed: %v", err)
	}
	t.Logf("Session1 (ENDED) FilesTouched: %v", state1.FilesTouched)

	session1StepCount := state1.StepCount

	// ========================================
	// Phase 2: Make some unrelated commits (simulating time passing)
	// ========================================
	t.Log("Phase 2: Making unrelated commits")

	for _, fileName := range []string{"file3.txt", "file4.txt"} {
		env.WriteFile(fileName, "unrelated content")
		env.GitAdd(fileName)
		env.GitCommitWithShadowHooks("Add "+fileName, fileName)
	}

	// ========================================
	// Phase 3: NEW session 2 starts and creates file6.txt
	// ========================================
	t.Log("Phase 3: Session 2 starts and creates file6.txt")

	session2 := env.NewSession()
	if err := env.SimulateUserPromptSubmit(session2.ID); err != nil {
		t.Fatalf("SimulateUserPromptSubmit for session2 failed: %v", err)
	}

	env.WriteFile("file6.txt", "content from session 2")
	session2.CreateTranscript(
		"Create file6",
		[]FileChange{{Path: "file6.txt", Content: "content from session 2"}},
	)
	if err := env.SimulateStop(session2.ID, session2.TranscriptPath); err != nil {
		t.Fatalf("SimulateStop for session2 failed: %v", err)
	}

	// Set session2 to ACTIVE
	state2, err := env.GetSessionState(session2.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session2 failed: %v", err)
	}
	state2.Phase = session.PhaseActive
	if err := env.WriteSessionState(session2.ID, state2); err != nil {
		t.Fatalf("WriteSessionState for session2 failed: %v", err)
	}

	// ========================================
	// Phase 4: Commit file6.txt (session 2's file)
	// ========================================
	t.Log("Phase 4: Committing file6.txt from session 2")

	env.GitAdd("file6.txt")
	env.GitCommitWithShadowHooks("Add file6 from session 2", "file6.txt")

	finalHead := env.GetHeadHash()

	// ========================================
	// Phase 5: Verify session 1 was NOT condensed
	// ========================================
	t.Log("Phase 5: Verifying session 1 (with carry-forward) was NOT condensed")

	state1After, err := env.GetSessionState(session1.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session1 after session2 commit failed: %v", err)
	}

	// StepCount should be unchanged
	if state1After.StepCount != session1StepCount {
		t.Errorf("Session 1 StepCount changed! Expected %d, got %d (incorrectly condensed into session 2's commit)",
			session1StepCount, state1After.StepCount)
	}

	// FilesTouched should still have file2.txt
	hasFile2 := false
	for _, f := range state1After.FilesTouched {
		if f == "file2.txt" {
			hasFile2 = true
			break
		}
	}
	if !hasFile2 {
		t.Errorf("Session 1 FilesTouched was cleared! Expected file2.txt, got: %v", state1After.FilesTouched)
	}

	t.Logf("Session 1 correctly preserved: StepCount=%d, FilesTouched=%v", state1After.StepCount, state1After.FilesTouched)

	// ========================================
	// Phase 6: Verify session 2 WAS condensed
	// ========================================
	t.Log("Phase 6: Verifying session 2 WAS condensed")

	state2After, err := env.GetSessionState(session2.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session2 after commit failed: %v", err)
	}

	if state2After.BaseCommit != finalHead {
		t.Errorf("Session 2 BaseCommit should be updated. Expected %s, got %s",
			finalHead[:7], state2After.BaseCommit[:7])
	}

	// ========================================
	// Phase 7: Finally commit file2.txt (session 1's carry-forward file)
	// ========================================
	t.Log("Phase 7: Committing file2.txt (session 1's carry-forward file)")

	env.GitAdd("file2.txt")
	env.GitCommitWithShadowHooks("Add file2 (session 1 carry-forward)", "file2.txt")

	// ========================================
	// Phase 8: Verify session 1 WAS condensed this time
	// ========================================
	t.Log("Phase 8: Verifying session 1 WAS condensed when its carry-forward file was committed")

	state1Final, err := env.GetSessionState(session1.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session1 after file2 commit failed: %v", err)
	}

	// StepCount should be reset to 0 (condensation happened)
	if state1Final.StepCount != 0 {
		t.Errorf("Session 1 StepCount should be 0 after condensation, got %d", state1Final.StepCount)
	}

	// FilesTouched should be empty (carry-forward consumed)
	if len(state1Final.FilesTouched) != 0 {
		t.Errorf("Session 1 FilesTouched should be empty after condensation, got: %v", state1Final.FilesTouched)
	}

	t.Log("Test completed successfully:")
	t.Log("  - Session 1 NOT condensed into session 2's commit (file6.txt)")
	t.Log("  - Session 1 WAS condensed when its own file (file2.txt) was committed")
}

// TestStaleActiveSession_NotCondensedIntoUnrelatedCommit verifies that a stale
// ACTIVE session (agent resumed then killed) is NOT condensed into an unrelated
// commit from a different session.
//
// This is a regression test for the bug where ACTIVE sessions would always be
// condensed (hasNew=true) without checking file overlap.
//
// Scenario:
// 1. Session 1 creates file1.txt, Stop hook fires (checkpoint saved)
// 2. Session 1 is resumed (back to ACTIVE), does more work
// 3. Agent is killed (stays ACTIVE, new work not committed)
// 4. New session 2 creates file2.txt (unrelated)
// 5. Session 2 commits file2.txt
// 6. Verify: Session 1 was NOT condensed (its files weren't in the commit)
func TestStaleActiveSession_NotCondensedIntoUnrelatedCommit(t *testing.T) {
	t.Parallel()
	env := NewTestEnv(t)
	defer env.Cleanup()

	// ========================================
	// Setup
	// ========================================
	env.InitRepo()
	env.WriteFile("README.md", "# Test Repository")
	env.GitAdd("README.md")
	env.GitCommit("Initial commit")
	env.GitCheckoutNewBranch("feature/stale-active-test")
	env.InitEntire(strategy.StrategyNameManualCommit)

	// ========================================
	// Phase 1: Session 1 creates file, Stop hook fires (creates checkpoint)
	// ========================================
	t.Log("Phase 1: Session 1 creates file1.txt, Stop hook fires")

	session1 := env.NewSession()
	if err := env.SimulateUserPromptSubmit(session1.ID); err != nil {
		t.Fatalf("SimulateUserPromptSubmit for session1 failed: %v", err)
	}

	env.WriteFile("file1.txt", "content from session 1")
	session1.CreateTranscript(
		"Create file1",
		[]FileChange{{Path: "file1.txt", Content: "content from session 1"}},
	)
	if err := env.SimulateStop(session1.ID, session1.TranscriptPath); err != nil {
		t.Fatalf("SimulateStop for session1 failed: %v", err)
	}

	// Session 1 is now IDLE with a checkpoint
	state1, err := env.GetSessionState(session1.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session1 failed: %v", err)
	}
	t.Logf("Session1 after Stop: Phase=%s, StepCount=%d, CheckpointTranscriptStart=%d",
		state1.Phase, state1.StepCount, state1.CheckpointTranscriptStart)

	// ========================================
	// Phase 2: Simulate agent resume + crash (ACTIVE with hasNew=true)
	// Set ACTIVE and reset CheckpointTranscriptStart to simulate:
	// - Agent was resumed (ACTIVE)
	// - Did more work (transcript grew, but not saved to checkpoint yet)
	// - Then crashed (stuck ACTIVE with hasNew=true)
	// ========================================
	t.Log("Phase 2: Simulating agent resume then crash (ACTIVE with hasNew=true)")

	state1.Phase = session.PhaseActive
	state1.CheckpointTranscriptStart = 0 // Force hasNew=true (transcript > 0)
	if err := env.WriteSessionState(session1.ID, state1); err != nil {
		t.Fatalf("WriteSessionState for session1 failed: %v", err)
	}

	// Record state for comparison
	session1StepCount := state1.StepCount
	t.Logf("Session1 (stale ACTIVE) StepCount: %d, CheckpointTranscriptStart: %d (forced to 0 for hasNew=true)",
		session1StepCount, state1.CheckpointTranscriptStart)

	// ========================================
	// Phase 2b: Make an unrelated commit to move HEAD forward
	// This ensures Session 1 and Session 2 have DIFFERENT shadow branches
	// (different BaseCommit), isolating their condensation behavior.
	// ========================================
	t.Log("Phase 2b: Making unrelated commit to separate shadow branches")
	env.WriteFile("unrelated.txt", "unrelated content")
	env.GitAdd("unrelated.txt")
	env.GitCommit("Unrelated commit to move HEAD")

	// ========================================
	// Phase 3: New session 2 creates unrelated file
	// ========================================
	t.Log("Phase 3: Session 2 creates file2.txt (unrelated to session 1)")

	session2 := env.NewSession()
	if err := env.SimulateUserPromptSubmit(session2.ID); err != nil {
		t.Fatalf("SimulateUserPromptSubmit for session2 failed: %v", err)
	}

	env.WriteFile("file2.txt", "content from session 2")
	session2.CreateTranscript(
		"Create file2",
		[]FileChange{{Path: "file2.txt", Content: "content from session 2"}},
	)
	if err := env.SimulateStop(session2.ID, session2.TranscriptPath); err != nil {
		t.Fatalf("SimulateStop for session2 failed: %v", err)
	}

	// Set session2 to ACTIVE
	state2, err := env.GetSessionState(session2.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session2 failed: %v", err)
	}
	state2.Phase = session.PhaseActive
	if err := env.WriteSessionState(session2.ID, state2); err != nil {
		t.Fatalf("WriteSessionState for session2 failed: %v", err)
	}

	// ========================================
	// Phase 4: Commit file2.txt (unrelated to session 1)
	// ========================================
	t.Log("Phase 4: Committing file2.txt from session 2")

	env.GitAdd("file2.txt")
	env.GitCommitWithShadowHooks("Add file2 from session 2", "file2.txt")

	finalHead := env.GetHeadHash()
	t.Logf("Final HEAD: %s", finalHead[:7])

	// ========================================
	// Phase 5: Verify stale session 1 was NOT condensed
	// ========================================
	t.Log("Phase 5: Verifying stale session 1 was NOT condensed")

	state1After, err := env.GetSessionState(session1.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session1 after session2 commit failed: %v", err)
	}

	// StepCount should be unchanged (condensation resets it)
	if state1After.StepCount != session1StepCount {
		t.Errorf("Stale session 1 StepCount changed! Expected %d, got %d (was incorrectly condensed)",
			session1StepCount, state1After.StepCount)
	}

	// Note: BaseCommit WILL be updated for ACTIVE sessions (even stale ones) because
	// updateBaseCommitIfChanged only skips IDLE/ENDED sessions. This is fine - the
	// key fix is the overlap check that prevents condensation, not BaseCommit updates.
	// A stale ACTIVE session that gets resumed later needs to know the current HEAD.

	// FilesTouched should still have file1.txt (not cleared by condensation)
	hasFile1 := false
	for _, f := range state1After.FilesTouched {
		if f == "file1.txt" {
			hasFile1 = true
			break
		}
	}
	if !hasFile1 {
		t.Errorf("Stale session 1 FilesTouched was cleared! Expected file1.txt, got: %v",
			state1After.FilesTouched)
	}

	// ========================================
	// Phase 6: Verify session 2 WAS condensed
	// ========================================
	t.Log("Phase 6: Verifying session 2 WAS condensed")

	state2After, err := env.GetSessionState(session2.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session2 after commit failed: %v", err)
	}

	if state2After.BaseCommit != finalHead {
		t.Errorf("Session 2 BaseCommit should be updated to new HEAD. Expected %s, got %s",
			finalHead[:7], state2After.BaseCommit[:7])
	}

	t.Log("Test completed successfully")
}

// TestIdleSessionEmptyFilesTouched_NotCondensedIntoUnrelatedCommit verifies that
// an IDLE session with empty FilesTouched (e.g., conversation-only session) is NOT
// condensed into an unrelated commit.
//
// This is a regression test for the fail-open bug where shouldCondenseWithOverlapCheck
// would return true when filesTouchedBefore was empty, causing conversation-only
// sessions to be condensed into every commit.
//
// Scenario:
// 1. Session 1 has a conversation but doesn't modify any files
// 2. New session 2 creates file1.txt
// 3. Session 2 commits file1.txt
// 4. Verify: Session 1 was NOT condensed
func TestIdleSessionEmptyFilesTouched_NotCondensedIntoUnrelatedCommit(t *testing.T) {
	t.Parallel()
	env := NewTestEnv(t)
	defer env.Cleanup()

	// ========================================
	// Setup
	// ========================================
	env.InitRepo()
	env.WriteFile("README.md", "# Test Repository")
	env.GitAdd("README.md")
	env.GitCommit("Initial commit")
	env.GitCheckoutNewBranch("feature/empty-files-test")
	env.InitEntire(strategy.StrategyNameManualCommit)

	// ========================================
	// Phase 1: Session 1 - conversation only, no file changes
	// ========================================
	t.Log("Phase 1: Session 1 has conversation but no file changes")

	session1 := env.NewSession()
	if err := env.SimulateUserPromptSubmit(session1.ID); err != nil {
		t.Fatalf("SimulateUserPromptSubmit for session1 failed: %v", err)
	}

	// Create transcript with NO file changes
	session1.CreateTranscript(
		"What is the meaning of life?",
		[]FileChange{}, // No files touched
	)
	if err := env.SimulateStop(session1.ID, session1.TranscriptPath); err != nil {
		t.Fatalf("SimulateStop for session1 failed: %v", err)
	}

	// Verify session1 is IDLE with empty FilesTouched
	state1, err := env.GetSessionState(session1.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session1 failed: %v", err)
	}
	if state1.Phase != session.PhaseIdle {
		t.Fatalf("Expected session1 to be IDLE, got %s", state1.Phase)
	}
	if len(state1.FilesTouched) != 0 {
		t.Fatalf("Expected session1 FilesTouched to be empty, got: %v", state1.FilesTouched)
	}

	session1StepCount := state1.StepCount
	session1BaseCommit := state1.BaseCommit
	t.Logf("Session1 (conversation-only) BaseCommit: %s, StepCount: %d",
		session1BaseCommit[:7], session1StepCount)

	// ========================================
	// Phase 2: Session 2 creates a file
	// ========================================
	t.Log("Phase 2: Session 2 creates file1.txt")

	session2 := env.NewSession()
	if err := env.SimulateUserPromptSubmit(session2.ID); err != nil {
		t.Fatalf("SimulateUserPromptSubmit for session2 failed: %v", err)
	}

	env.WriteFile("file1.txt", "content from session 2")
	session2.CreateTranscript(
		"Create file1",
		[]FileChange{{Path: "file1.txt", Content: "content from session 2"}},
	)
	if err := env.SimulateStop(session2.ID, session2.TranscriptPath); err != nil {
		t.Fatalf("SimulateStop for session2 failed: %v", err)
	}

	// Set session2 to ACTIVE
	state2, err := env.GetSessionState(session2.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session2 failed: %v", err)
	}
	state2.Phase = session.PhaseActive
	if err := env.WriteSessionState(session2.ID, state2); err != nil {
		t.Fatalf("WriteSessionState for session2 failed: %v", err)
	}

	// ========================================
	// Phase 3: Commit file1.txt
	// ========================================
	t.Log("Phase 3: Committing file1.txt from session 2")

	env.GitAdd("file1.txt")
	env.GitCommitWithShadowHooks("Add file1 from session 2", "file1.txt")

	finalHead := env.GetHeadHash()

	// ========================================
	// Phase 4: Verify conversation-only session 1 was NOT condensed
	// ========================================
	t.Log("Phase 4: Verifying conversation-only session 1 was NOT condensed")

	state1After, err := env.GetSessionState(session1.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session1 after session2 commit failed: %v", err)
	}

	// StepCount should be unchanged
	if state1After.StepCount != session1StepCount {
		t.Errorf("Conversation-only session 1 StepCount changed! Expected %d, got %d",
			session1StepCount, state1After.StepCount)
	}

	// BaseCommit should be unchanged
	if state1After.BaseCommit != session1BaseCommit {
		t.Errorf("Conversation-only session 1 BaseCommit changed! Expected %s, got %s",
			session1BaseCommit[:7], state1After.BaseCommit[:7])
	}

	// FilesTouched should still be empty
	if len(state1After.FilesTouched) != 0 {
		t.Errorf("Conversation-only session 1 FilesTouched should remain empty, got: %v",
			state1After.FilesTouched)
	}

	// ========================================
	// Phase 6: Verify session 2 WAS condensed
	// ========================================
	t.Log("Phase 6: Verifying session 2 WAS condensed")

	state2After, err := env.GetSessionState(session2.ID)
	if err != nil {
		t.Fatalf("GetSessionState for session2 after commit failed: %v", err)
	}

	if state2After.BaseCommit != finalHead {
		t.Errorf("Session 2 BaseCommit should be updated to new HEAD. Expected %s, got %s",
			finalHead[:7], state2After.BaseCommit[:7])
	}

	t.Log("Test completed successfully")
}
