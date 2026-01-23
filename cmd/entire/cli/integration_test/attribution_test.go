//go:build integration

package integration

import (
	"encoding/json"
	"testing"

	"entire.io/cli/cmd/entire/cli/checkpoint"
	"entire.io/cli/cmd/entire/cli/strategy"
	"entire.io/cli/cmd/entire/cli/trailers"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// TestManualCommit_Attribution tests the full attribution calculation flow:
// 1. Agent creates checkpoint 1
// 2. User makes changes between checkpoints
// 3. User enters new prompt (attribution calculated at prompt start)
// 4. Agent creates checkpoint 2
// 5. User commits (condensation happens with attribution)
// 6. Verify attribution metadata is correct
func TestManualCommit_Attribution(t *testing.T) {
	env := NewTestEnv(t)
	defer env.Cleanup()

	env.InitRepo()

	// Create initial commit
	env.WriteFile("main.go", "package main\n")
	env.GitAdd("main.go")
	env.GitCommit("Initial commit")

	env.InitEntire(strategy.StrategyNameManualCommit)

	initialHead := env.GetHeadHash()
	t.Logf("Initial HEAD: %s", initialHead[:7])

	// ========================================
	// CHECKPOINT 1: Agent adds function
	// ========================================
	t.Log("Creating checkpoint 1 (agent adds function)")

	session := env.NewSession()
	if err := env.SimulateUserPromptSubmit(session.ID); err != nil {
		t.Fatalf("SimulateUserPromptSubmit (prompt 1) failed: %v", err)
	}

	// Agent adds 4 lines
	checkpoint1Content := "package main\n\nfunc agentFunc() {\n\treturn 42\n}\n"
	env.WriteFile("main.go", checkpoint1Content)

	session.CreateTranscript(
		"Add agent function",
		[]FileChange{{Path: "main.go", Content: checkpoint1Content}},
	)
	if err := env.SimulateStop(session.ID, session.TranscriptPath); err != nil {
		t.Fatalf("SimulateStop (checkpoint 1) failed: %v", err)
	}

	// ========================================
	// USER EDITS between checkpoints
	// ========================================
	t.Log("User makes edits between checkpoints")

	// User adds 5 comment lines
	userContent := checkpoint1Content +
		"// User comment 1\n" +
		"// User comment 2\n" +
		"// User comment 3\n" +
		"// User comment 4\n" +
		"// User comment 5\n"
	env.WriteFile("main.go", userContent)

	// ========================================
	// CHECKPOINT 2: New prompt (attribution calculated)
	// ========================================
	t.Log("User enters new prompt (attribution should capture 5 user lines)")

	// Simulate UserPromptSubmit hook - this calculates attribution at prompt start
	if err := env.SimulateUserPromptSubmit(session.ID); err != nil {
		t.Fatalf("SimulateUserPromptSubmit (prompt 2) failed: %v", err)
	}

	// Agent adds another function (4 more lines)
	checkpoint2Content := userContent + "\nfunc agentFunc2() {\n\treturn 100\n}\n"
	env.WriteFile("main.go", checkpoint2Content)

	session.CreateTranscript(
		"Add second agent function",
		[]FileChange{{Path: "main.go", Content: checkpoint2Content}},
	)
	if err := env.SimulateStop(session.ID, session.TranscriptPath); err != nil {
		t.Fatalf("SimulateStop (checkpoint 2) failed: %v", err)
	}

	// Verify 2 rewind points
	points := env.GetRewindPoints()
	if len(points) != 2 {
		t.Fatalf("Expected 2 rewind points, got %d", len(points))
	}

	// ========================================
	// USER COMMITS: Condensation happens
	// ========================================
	t.Log("User commits (condensation should happen)")

	// Commit using hooks (this triggers condensation)
	env.GitCommitWithShadowHooks("Add functions", "main.go")

	// Get commit hash and checkpoint ID
	headHash := env.GetHeadHash()
	t.Logf("User commit: %s", headHash[:7])

	repo, err := git.PlainOpen(env.RepoDir)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	commitObj, err := repo.CommitObject(plumbing.NewHash(headHash))
	if err != nil {
		t.Fatalf("failed to get commit object: %v", err)
	}

	checkpointID, found := trailers.ParseCheckpoint(commitObj.Message)
	if !found {
		t.Fatal("Commit should have Entire-Checkpoint trailer")
	}
	t.Logf("Checkpoint ID: %s", checkpointID)

	// ========================================
	// VERIFY ATTRIBUTION
	// ========================================
	t.Log("Verifying attribution in metadata")

	// Read metadata from entire/sessions branch
	sessionsRef, err := repo.Reference(plumbing.NewBranchReferenceName("entire/sessions"), true)
	if err != nil {
		t.Fatalf("Failed to get entire/sessions branch: %v", err)
	}

	sessionsCommit, err := repo.CommitObject(sessionsRef.Hash())
	if err != nil {
		t.Fatalf("Failed to get sessions commit: %v", err)
	}

	sessionsTree, err := sessionsCommit.Tree()
	if err != nil {
		t.Fatalf("Failed to get sessions tree: %v", err)
	}

	// Read metadata.json from sharded path
	metadataPath := checkpointID.String()[:2] + "/" + checkpointID.String()[2:] + "/metadata.json"
	metadataFile, err := sessionsTree.File(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata.json at path %s: %v", metadataPath, err)
	}

	metadataContent, err := metadataFile.Contents()
	if err != nil {
		t.Fatalf("Failed to read metadata content: %v", err)
	}

	var metadata checkpoint.CommittedMetadata
	if err := json.Unmarshal([]byte(metadataContent), &metadata); err != nil {
		t.Fatalf("Failed to parse metadata.json: %v", err)
	}

	// Verify InitialAttribution exists
	if metadata.InitialAttribution == nil {
		t.Fatal("InitialAttribution is nil")
	}

	attr := metadata.InitialAttribution
	t.Logf("Attribution: agent=%d, human_added=%d, human_modified=%d, human_removed=%d, total=%d, percentage=%.1f%%",
		attr.AgentLines, attr.HumanAdded, attr.HumanModified, attr.HumanRemoved,
		attr.TotalCommitted, attr.AgentPercentage)

	// Verify attribution was calculated and has reasonable values
	// Note: The shadow branch includes all worktree changes (agent + user),
	// so base→shadow diff includes user edits that were present during SaveChanges.
	// The attribution separates them using PromptAttributions.
	//
	// Expected: agent=13 (base→shadow includes user comments in worktree)
	//           human=5 (from PromptAttribution)
	//           total=18 (net additions)
	//
	// This tests that:
	// 1. Attribution is calculated and stored
	// 2. PromptAttribution captured user edits between checkpoints
	// 3. Percentages are computed
	if attr.AgentLines <= 0 {
		t.Errorf("AgentLines = %d, should be > 0", attr.AgentLines)
	}

	if attr.HumanAdded != 5 {
		t.Errorf("HumanAdded = %d, want 5 (5 comments captured in PromptAttribution)",
			attr.HumanAdded)
	}

	if attr.TotalCommitted <= 0 {
		t.Errorf("TotalCommitted = %d, should be > 0", attr.TotalCommitted)
	}

	if attr.AgentPercentage <= 0 || attr.AgentPercentage >= 100 {
		t.Errorf("AgentPercentage = %.1f%%, should be between 0 and 100",
			attr.AgentPercentage)
	}
}

// TestManualCommit_AttributionDeletionOnly tests attribution for deletion-only commits
func TestManualCommit_AttributionDeletionOnly(t *testing.T) {
	env := NewTestEnv(t)
	defer env.Cleanup()

	env.InitRepo()

	// Create initial commit with content
	initialContent := "package main\n\nfunc oldFunc1() {}\nfunc oldFunc2() {}\nfunc oldFunc3() {}\n"
	env.WriteFile("main.go", initialContent)
	env.GitAdd("main.go")
	env.GitCommit("Initial commit")

	env.InitEntire(strategy.StrategyNameManualCommit)

	// ========================================
	// CHECKPOINT 1: Agent REMOVES a function (deletion, no additions)
	// ========================================
	session := env.NewSession()
	if err := env.SimulateUserPromptSubmit(session.ID); err != nil {
		t.Fatalf("SimulateUserPromptSubmit failed: %v", err)
	}

	// Agent removes one function (keeps 2 functions)
	checkpointContent := "package main\n\nfunc oldFunc2() {}\nfunc oldFunc3() {}\n"
	env.WriteFile("main.go", checkpointContent)

	session.CreateTranscript(
		"Remove oldFunc1",
		[]FileChange{{Path: "main.go", Content: checkpointContent}},
	)
	if err := env.SimulateStop(session.ID, session.TranscriptPath); err != nil {
		t.Fatalf("SimulateStop failed: %v", err)
	}

	// ========================================
	// USER DELETES REMAINING FUNCTIONS
	// ========================================
	t.Log("User deletes remaining functions (deletion-only commit)")

	// Remove remaining functions, keep only package declaration
	env.WriteFile("main.go", "package main\n")

	// Commit using hooks
	env.GitCommitWithShadowHooks("Remove remaining functions", "main.go")

	// Get checkpoint ID
	headHash := env.GetHeadHash()
	repo, err := git.PlainOpen(env.RepoDir)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	commitObj, err := repo.CommitObject(plumbing.NewHash(headHash))
	if err != nil {
		t.Fatalf("failed to get commit object: %v", err)
	}

	checkpointID, found := trailers.ParseCheckpoint(commitObj.Message)
	if !found {
		t.Fatal("Commit should have Entire-Checkpoint trailer")
	}

	// ========================================
	// VERIFY ATTRIBUTION FOR DELETION-ONLY COMMIT
	// ========================================
	t.Log("Verifying attribution for deletion-only commit")

	sessionsRef, err := repo.Reference(plumbing.NewBranchReferenceName("entire/sessions"), true)
	if err != nil {
		t.Fatalf("Failed to get entire/sessions branch: %v", err)
	}

	sessionsCommit, err := repo.CommitObject(sessionsRef.Hash())
	if err != nil {
		t.Fatalf("Failed to get sessions commit: %v", err)
	}

	sessionsTree, err := sessionsCommit.Tree()
	if err != nil {
		t.Fatalf("Failed to get sessions tree: %v", err)
	}

	metadataPath := checkpointID.String()[:2] + "/" + checkpointID.String()[2:] + "/metadata.json"
	metadataFile, err := sessionsTree.File(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata.json: %v", err)
	}

	metadataContent, err := metadataFile.Contents()
	if err != nil {
		t.Fatalf("Failed to read metadata content: %v", err)
	}

	var metadata checkpoint.CommittedMetadata
	if err := json.Unmarshal([]byte(metadataContent), &metadata); err != nil {
		t.Fatalf("Failed to parse metadata.json: %v", err)
	}

	if metadata.InitialAttribution == nil {
		t.Fatal("InitialAttribution is nil")
	}

	attr := metadata.InitialAttribution
	t.Logf("Attribution (deletion-only): agent=%d, human_added=%d, human_removed=%d, total=%d, percentage=%.1f%%",
		attr.AgentLines, attr.HumanAdded, attr.HumanRemoved,
		attr.TotalCommitted, attr.AgentPercentage)

	// For deletion-only commits where agent makes no additions:
	// - Agent removed oldFunc1 (made deletions, not additions)
	// - AgentLines = 0 (no additions)
	// - User removed oldFunc2 and oldFunc3
	// - HumanAdded = 0 (no new lines)
	// - HumanRemoved = number of lines user deleted
	// - TotalCommitted = 0 (no additions from anyone)
	// - AgentPercentage = 0 (by convention for deletion-only)

	if attr.AgentLines != 0 {
		t.Errorf("AgentLines = %d, want 0 (agent made no additions, only deletions)", attr.AgentLines)
	}

	if attr.HumanAdded != 0 {
		t.Errorf("HumanAdded = %d, want 0 (no new lines in deletion-only commit)", attr.HumanAdded)
	}

	// User removed 2 remaining functions + 1 blank line (3 lines total)
	if attr.HumanRemoved != 3 {
		t.Errorf("HumanRemoved = %d, want 3 (removed blank + 2 functions = 3 lines)", attr.HumanRemoved)
	}

	if attr.TotalCommitted != 0 {
		t.Errorf("TotalCommitted = %d, want 0 (deletion-only commit has no net additions)", attr.TotalCommitted)
	}

	if attr.AgentPercentage != 0 {
		t.Errorf("AgentPercentage = %.1f%%, want 0 (deletion-only commit)",
			attr.AgentPercentage)
	}
}
