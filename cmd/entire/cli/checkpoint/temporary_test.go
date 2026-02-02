package checkpoint

import "testing"

func TestHashWorktreeID(t *testing.T) {
	tests := []struct {
		name       string
		worktreeID string
		wantLen    int
	}{
		{
			name:       "empty string (main worktree)",
			worktreeID: "",
			wantLen:    6,
		},
		{
			name:       "simple worktree name",
			worktreeID: "test-123",
			wantLen:    6,
		},
		{
			name:       "complex worktree name",
			worktreeID: "feature/auth-system",
			wantLen:    6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HashWorktreeID(tt.worktreeID)
			if len(got) != tt.wantLen {
				t.Errorf("HashWorktreeID(%q) length = %d, want %d", tt.worktreeID, len(got), tt.wantLen)
			}
		})
	}
}

func TestHashWorktreeID_Deterministic(t *testing.T) {
	// Same input should always produce same output
	id := "test-worktree"
	hash1 := HashWorktreeID(id)
	hash2 := HashWorktreeID(id)
	if hash1 != hash2 {
		t.Errorf("HashWorktreeID not deterministic: %q != %q", hash1, hash2)
	}
}

func TestHashWorktreeID_DifferentInputs(t *testing.T) {
	// Different inputs should produce different outputs
	hash1 := HashWorktreeID("worktree-a")
	hash2 := HashWorktreeID("worktree-b")
	if hash1 == hash2 {
		t.Errorf("HashWorktreeID produced same hash for different inputs: %q", hash1)
	}
}
