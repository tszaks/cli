package strategy

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/entireio/cli/cmd/entire/cli/checkpoint/id"
	"github.com/entireio/cli/cmd/entire/cli/paths"
	"github.com/entireio/cli/cmd/entire/cli/session"
	"github.com/entireio/cli/cmd/entire/cli/trailers"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPostCommit_ActiveSession_NoCondensation verifies that PostCommit on an
// ACTIVE session transitions to ACTIVE_COMMITTED without condensing.
// The shadow branch must be preserved because the session is still active.
func TestPostCommit_ActiveSession_NoCondensation(t *testing.T) {
	dir := setupGitRepo(t)
	t.Chdir(dir)

	repo, err := git.PlainOpen(dir)
	require.NoError(t, err)

	s := &ManualCommitStrategy{}
	sessionID := "test-postcommit-active"

	// Initialize session and save a checkpoint so there is shadow branch content
	setupSessionWithCheckpoint(t, s, repo, dir, sessionID)

	// Set phase to ACTIVE (simulating agent mid-turn)
	state, err := s.loadSessionState(sessionID)
	require.NoError(t, err)
	state.Phase = session.PhaseActive
	require.NoError(t, s.saveSessionState(state))

	// Create a commit WITH the Entire-Checkpoint trailer on the main branch
	commitWithCheckpointTrailer(t, repo, dir, "a1b2c3d4e5f6")

	// Run PostCommit
	err = s.PostCommit()
	require.NoError(t, err)

	// Verify phase transitioned to ACTIVE_COMMITTED
	state, err = s.loadSessionState(sessionID)
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, session.PhaseActiveCommitted, state.Phase,
		"ACTIVE session should transition to ACTIVE_COMMITTED on GitCommit")

	// Verify shadow branch is NOT deleted (session is still active).
	// After PostCommit, BaseCommit is updated to new HEAD, so use the current state.
	shadowBranch := getShadowBranchNameForCommit(state.BaseCommit, state.WorktreeID)
	refName := plumbing.NewBranchReferenceName(shadowBranch)
	_, err = repo.Reference(refName, true)
	assert.NoError(t, err,
		"shadow branch should be preserved when session is still active")
}

// TestPostCommit_IdleSession_Condenses verifies that PostCommit on an IDLE
// session condenses session data and cleans up the shadow branch.
func TestPostCommit_IdleSession_Condenses(t *testing.T) {
	dir := setupGitRepo(t)
	t.Chdir(dir)

	repo, err := git.PlainOpen(dir)
	require.NoError(t, err)

	s := &ManualCommitStrategy{}
	sessionID := "test-postcommit-idle"

	// Initialize session and save a checkpoint so there is shadow branch content
	setupSessionWithCheckpoint(t, s, repo, dir, sessionID)

	// Set phase to IDLE (agent turn finished, waiting for user)
	state, err := s.loadSessionState(sessionID)
	require.NoError(t, err)
	state.Phase = session.PhaseIdle
	state.LastInteractionTime = nil
	require.NoError(t, s.saveSessionState(state))

	// Record shadow branch name before PostCommit
	shadowBranch := getShadowBranchNameForCommit(state.BaseCommit, state.WorktreeID)

	// Create a commit WITH the Entire-Checkpoint trailer
	commitWithCheckpointTrailer(t, repo, dir, "b2c3d4e5f6a1")

	// Run PostCommit
	err = s.PostCommit()
	require.NoError(t, err)

	// Verify condensation happened: the entire/sessions branch should exist
	sessionsRef, err := repo.Reference(plumbing.NewBranchReferenceName(paths.MetadataBranchName), true)
	require.NoError(t, err, "entire/sessions branch should exist after condensation")
	assert.NotNil(t, sessionsRef)

	// Verify shadow branch IS deleted after condensation
	refName := plumbing.NewBranchReferenceName(shadowBranch)
	_, err = repo.Reference(refName, true)
	assert.Error(t, err,
		"shadow branch should be deleted after condensation for IDLE session")
}

// TestPostCommit_RebaseDuringActive_SkipsTransition verifies that PostCommit
// is a no-op during rebase operations, leaving the session phase unchanged.
func TestPostCommit_RebaseDuringActive_SkipsTransition(t *testing.T) {
	dir := setupGitRepo(t)
	t.Chdir(dir)

	repo, err := git.PlainOpen(dir)
	require.NoError(t, err)

	s := &ManualCommitStrategy{}
	sessionID := "test-postcommit-rebase"

	// Initialize session and save a checkpoint
	setupSessionWithCheckpoint(t, s, repo, dir, sessionID)

	// Set phase to ACTIVE
	state, err := s.loadSessionState(sessionID)
	require.NoError(t, err)
	state.Phase = session.PhaseActive
	require.NoError(t, s.saveSessionState(state))

	// Capture shadow branch name BEFORE any state changes
	shadowBranch := getShadowBranchNameForCommit(state.BaseCommit, state.WorktreeID)
	originalStepCount := state.StepCount

	// Simulate rebase in progress by creating .git/rebase-merge/ directory
	gitDir := filepath.Join(dir, ".git")
	rebaseMergeDir := filepath.Join(gitDir, "rebase-merge")
	require.NoError(t, os.MkdirAll(rebaseMergeDir, 0o755))
	defer os.RemoveAll(rebaseMergeDir)

	// Create a commit WITH the checkpoint trailer
	commitWithCheckpointTrailer(t, repo, dir, "c3d4e5f6a1b2")

	// Run PostCommit
	err = s.PostCommit()
	require.NoError(t, err)

	// Verify phase stayed ACTIVE (no transition during rebase)
	state, err = s.loadSessionState(sessionID)
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, session.PhaseActive, state.Phase,
		"session should stay ACTIVE during rebase (no transition)")

	// Verify StepCount was NOT reset (no condensation happened)
	assert.Equal(t, originalStepCount, state.StepCount,
		"StepCount should be unchanged - no condensation during rebase")

	// Verify NO condensation happened (entire/sessions branch should not exist)
	_, err = repo.Reference(plumbing.NewBranchReferenceName(paths.MetadataBranchName), true)
	require.Error(t, err,
		"entire/sessions branch should NOT exist - no condensation during rebase")

	// Verify shadow branch still exists (not cleaned up during rebase)
	refName := plumbing.NewBranchReferenceName(shadowBranch)
	_, err = repo.Reference(refName, true)
	assert.NoError(t, err,
		"shadow branch should be preserved during rebase")
}

// TestPostCommit_ShadowBranch_PreservedWhenActiveSessionExists verifies that
// the shadow branch is preserved when ANY session on it is still active,
// even if another session on the same branch is IDLE and gets condensed.
func TestPostCommit_ShadowBranch_PreservedWhenActiveSessionExists(t *testing.T) {
	dir := setupGitRepo(t)
	t.Chdir(dir)

	repo, err := git.PlainOpen(dir)
	require.NoError(t, err)

	s := &ManualCommitStrategy{}
	idleSessionID := "test-postcommit-idle-multi"
	activeSessionID := "test-postcommit-active-multi"

	// Initialize the idle session with a checkpoint
	setupSessionWithCheckpoint(t, s, repo, dir, idleSessionID)

	// Get worktree path and base commit from the idle session
	idleState, err := s.loadSessionState(idleSessionID)
	require.NoError(t, err)
	worktreePath := idleState.WorktreePath
	baseCommit := idleState.BaseCommit
	worktreeID := idleState.WorktreeID

	// Set idle session to IDLE phase
	idleState.Phase = session.PhaseIdle
	idleState.LastInteractionTime = nil
	require.NoError(t, s.saveSessionState(idleState))

	// Create a second session with the SAME base commit and worktree (concurrent session)
	// Save the active session with ACTIVE phase and some checkpoints
	now := time.Now()
	activeState := &SessionState{
		SessionID:           activeSessionID,
		BaseCommit:          baseCommit,
		WorktreePath:        worktreePath,
		WorktreeID:          worktreeID,
		StartedAt:           now,
		Phase:               session.PhaseActive,
		LastInteractionTime: &now,
		StepCount:           1,
	}
	require.NoError(t, s.saveSessionState(activeState))

	// Create a commit WITH the checkpoint trailer
	commitWithCheckpointTrailer(t, repo, dir, "d4e5f6a1b2c3")

	// Run PostCommit
	err = s.PostCommit()
	require.NoError(t, err)

	// Verify the ACTIVE session's phase is now ACTIVE_COMMITTED
	activeState, err = s.loadSessionState(activeSessionID)
	require.NoError(t, err)
	assert.Equal(t, session.PhaseActiveCommitted, activeState.Phase,
		"ACTIVE session should transition to ACTIVE_COMMITTED on GitCommit")

	// Verify the IDLE session actually condensed (entire/sessions branch should exist)
	idleState, err = s.loadSessionState(idleSessionID)
	require.NoError(t, err)
	sessionsRef, err := repo.Reference(plumbing.NewBranchReferenceName(paths.MetadataBranchName), true)
	require.NoError(t, err, "entire/sessions branch should exist after IDLE session condensation")
	require.NotNil(t, sessionsRef)

	// Verify IDLE session's StepCount was reset by condensation
	assert.Equal(t, 0, idleState.StepCount,
		"IDLE session StepCount should be reset after condensation")

	// Verify shadow branch is NOT deleted because the ACTIVE session still needs it.
	// After PostCommit, BaseCommit is updated to new HEAD via migration.
	newShadowBranch := getShadowBranchNameForCommit(activeState.BaseCommit, activeState.WorktreeID)
	refName := plumbing.NewBranchReferenceName(newShadowBranch)
	_, err = repo.Reference(refName, true)
	assert.NoError(t, err,
		"shadow branch should be preserved when an active session still exists on it")
}

// setupSessionWithCheckpoint initializes a session and creates one checkpoint
// on the shadow branch so there is content available for condensation.
func setupSessionWithCheckpoint(t *testing.T, s *ManualCommitStrategy, _ *git.Repository, dir, sessionID string) {
	t.Helper()

	// Create metadata directory with a transcript file
	metadataDir := ".entire/metadata/" + sessionID
	metadataDirAbs := filepath.Join(dir, metadataDir)
	require.NoError(t, os.MkdirAll(metadataDirAbs, 0o755))

	transcript := `{"type":"human","message":{"content":"test prompt"}}
{"type":"assistant","message":{"content":"test response"}}
`
	require.NoError(t, os.WriteFile(
		filepath.Join(metadataDirAbs, paths.TranscriptFileName),
		[]byte(transcript), 0o644))

	// SaveChanges creates the shadow branch and checkpoint
	err := s.SaveChanges(SaveContext{
		SessionID:      sessionID,
		ModifiedFiles:  []string{},
		NewFiles:       []string{},
		DeletedFiles:   []string{},
		MetadataDir:    metadataDir,
		MetadataDirAbs: metadataDirAbs,
		CommitMessage:  "Checkpoint 1",
		AuthorName:     "Test",
		AuthorEmail:    "test@test.com",
	})
	require.NoError(t, err, "SaveChanges should succeed to create shadow branch content")
}

// commitWithCheckpointTrailer creates a commit on the current branch with the
// Entire-Checkpoint trailer in the commit message. This simulates what happens
// after PrepareCommitMsg adds the trailer and the user completes the commit.
func commitWithCheckpointTrailer(t *testing.T, repo *git.Repository, dir, checkpointIDStr string) {
	t.Helper()

	cpID := id.MustCheckpointID(checkpointIDStr)

	// Modify a file so there is something to commit
	testFile := filepath.Join(dir, "test.txt")
	content := "updated at " + time.Now().String()
	require.NoError(t, os.WriteFile(testFile, []byte(content), 0o644))

	wt, err := repo.Worktree()
	require.NoError(t, err)

	_, err = wt.Add("test.txt")
	require.NoError(t, err)

	commitMsg := "test commit\n\n" + trailers.CheckpointTrailerKey + ": " + cpID.String() + "\n"
	_, err = wt.Commit(commitMsg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err, "commit with checkpoint trailer should succeed")
}
