package session

import (
	"context"

	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
)

func TestSession_IsSubSession(t *testing.T) {
	tests := []struct {
		name     string
		session  Session
		expected bool
	}{
		{
			name: "top-level session with empty ParentID",
			session: Session{
				ID:       "session-123",
				ParentID: "",
			},
			expected: false,
		},
		{
			name: "sub-session with ParentID set",
			session: Session{
				ID:        "session-456",
				ParentID:  "session-123",
				ToolUseID: "toolu_abc",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.session.IsSubSession()
			if result != tt.expected {
				t.Errorf("IsSubSession() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestGetOrCreateEntireSessionID tests the stable session ID generation logic.
func TestGetOrCreateEntireSessionID(t *testing.T) {
	dir := t.TempDir()
	_, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	t.Chdir(dir)

	agentUUID := "a6c3cac2-2f45-43aa-8c69-419f66a3b5e1"

	// First call - should create new session ID with today's date
	sessionID1 := GetOrCreateEntireSessionID(agentUUID)

	// Verify format: YYYY-MM-DD-<uuid>
	if len(sessionID1) < 11 {
		t.Fatalf("Session ID too short: %s", sessionID1)
	}
	expectedSuffix := "-" + agentUUID
	if sessionID1[len(sessionID1)-len(expectedSuffix):] != expectedSuffix {
		t.Errorf("Session ID should end with %s, got %s", expectedSuffix, sessionID1)
	}

	// Create a state store and save a state to simulate existing session
	store, err := NewStateStore()
	if err != nil {
		t.Fatalf("NewStateStore() error = %v", err)
	}

	state := &State{
		SessionID:       sessionID1,
		BaseCommit:      "test123",
		StartedAt:       time.Now(),
		CheckpointCount: 1,
	}
	if err := store.Save(context.Background(), state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Second call - should reuse existing session ID
	sessionID2 := GetOrCreateEntireSessionID(agentUUID)

	if sessionID2 != sessionID1 {
		t.Errorf("Expected to reuse session ID %s, got %s", sessionID1, sessionID2)
	}
}

// TestGetOrCreateEntireSessionID_MultipleStates tests cleanup of duplicate state files.
func TestGetOrCreateEntireSessionID_MultipleStates(t *testing.T) {
	dir := t.TempDir()
	_, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	t.Chdir(dir)

	agentUUID := "b7d4dbd3-3e56-54bb-a70a-52ae77d94c6f"

	// Simulate the bug: create state files from different days
	oldSessionID := "2026-01-22-" + agentUUID
	newSessionID := "2026-01-23-" + agentUUID

	store, err := NewStateStore()
	if err != nil {
		t.Fatalf("NewStateStore() error = %v", err)
	}

	oldState := &State{
		SessionID:       oldSessionID,
		BaseCommit:      "old123",
		StartedAt:       time.Now().Add(-24 * time.Hour),
		CheckpointCount: 2,
	}
	if err := store.Save(context.Background(), oldState); err != nil {
		t.Fatalf("Save(old) error = %v", err)
	}

	newState := &State{
		SessionID:       newSessionID,
		BaseCommit:      "new456",
		StartedAt:       time.Now(),
		CheckpointCount: 3,
	}
	if err := store.Save(context.Background(), newState); err != nil {
		t.Fatalf("Save(new) error = %v", err)
	}

	// Call GetOrCreateEntireSessionID - should pick the newest and cleanup old
	selectedID := GetOrCreateEntireSessionID(agentUUID)

	// Should pick the most recent (2026-01-23)
	if selectedID != newSessionID {
		t.Errorf("Expected most recent session ID %s, got %s", newSessionID, selectedID)
	}

	// Old state file should be cleaned up
	oldStateLoaded, err := store.Load(context.Background(), oldSessionID)
	if err != nil {
		t.Fatalf("Load(old) error = %v", err)
	}
	if oldStateLoaded != nil {
		t.Errorf("Old state file should have been cleaned up, but still exists")
	}

	// New state file should still exist
	newStateLoaded, err := store.Load(context.Background(), newSessionID)
	if err != nil {
		t.Fatalf("Load(new) error = %v", err)
	}
	if newStateLoaded == nil {
		t.Errorf("New state file should exist")
	}
}
func TestStateStore_RemoveAll(t *testing.T) {
	// Create a temp directory for the state store
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, "entire-sessions")

	store := NewStateStoreWithDir(stateDir)
	ctx := context.Background()

	// Create some session states
	states := []*State{
		{
			SessionID:  "session-1",
			BaseCommit: "abc123",
			StartedAt:  time.Now(),
		},
		{
			SessionID:  "session-2",
			BaseCommit: "def456",
			StartedAt:  time.Now(),
		},
		{
			SessionID:  "session-3",
			BaseCommit: "ghi789",
			StartedAt:  time.Now(),
		},
	}

	for _, state := range states {
		if err := store.Save(ctx, state); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	// Verify states were saved
	savedStates, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(savedStates) != len(states) {
		t.Fatalf("List() returned %d states, want %d", len(savedStates), len(states))
	}

	// Verify directory exists
	if _, err := os.Stat(stateDir); os.IsNotExist(err) {
		t.Fatal("state directory should exist before RemoveAll()")
	}

	// Remove all
	if err := store.RemoveAll(); err != nil {
		t.Fatalf("RemoveAll() error = %v", err)
	}

	// Verify directory is removed
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Error("state directory should not exist after RemoveAll()")
	}

	// List should return empty (directory doesn't exist)
	afterStates, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() after RemoveAll() error = %v", err)
	}
	if len(afterStates) != 0 {
		t.Errorf("List() after RemoveAll() returned %d states, want 0", len(afterStates))
	}
}

func TestStateStore_RemoveAll_EmptyDirectory(t *testing.T) {
	// Create a temp directory for the state store
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, "entire-sessions")

	// Create the directory but don't add any files
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		t.Fatalf("failed to create state dir: %v", err)
	}

	store := NewStateStoreWithDir(stateDir)

	// Remove all on empty directory should succeed
	if err := store.RemoveAll(); err != nil {
		t.Fatalf("RemoveAll() on empty directory error = %v", err)
	}

	// Directory should be removed
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Error("state directory should not exist after RemoveAll()")
	}
}

func TestStateStore_RemoveAll_NonExistentDirectory(t *testing.T) {
	// Create a temp directory for the state store
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, "nonexistent-sessions")

	store := NewStateStoreWithDir(stateDir)

	// RemoveAll on non-existent directory should succeed (no-op)
	if err := store.RemoveAll(); err != nil {
		t.Fatalf("RemoveAll() on non-existent directory error = %v", err)
	}
}
