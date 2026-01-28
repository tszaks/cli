package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
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
