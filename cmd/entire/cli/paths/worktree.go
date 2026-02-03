package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GetWorktreeID returns the internal git worktree identifier for the given path.
// For the main worktree (where .git is a directory), returns empty string.
// For linked worktrees (where .git is a file), extracts the name from
// .git/worktrees/<name>/ path. This name is stable across `git worktree move`.
func GetWorktreeID(worktreePath string) (string, error) {
	gitPath := filepath.Join(worktreePath, ".git")

	info, err := os.Stat(gitPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat .git: %w", err)
	}

	// Main worktree has .git as a directory
	if info.IsDir() {
		return "", nil
	}

	// Linked worktree has .git as a file with content: "gitdir: /path/to/.git/worktrees/<name>"
	content, err := os.ReadFile(gitPath) //nolint:gosec // gitPath is constructed from worktreePath + ".git"
	if err != nil {
		return "", fmt.Errorf("failed to read .git file: %w", err)
	}

	line := strings.TrimSpace(string(content))
	if !strings.HasPrefix(line, "gitdir: ") {
		return "", fmt.Errorf("invalid .git file format: %s", line)
	}

	gitdir := strings.TrimPrefix(line, "gitdir: ")

	// Extract worktree name from path like /repo/.git/worktrees/<name>
	// The path after ".git/worktrees/" is the worktree identifier
	const marker = ".git/worktrees/"
	_, worktreeID, found := strings.Cut(gitdir, marker)
	if !found {
		return "", fmt.Errorf("unexpected gitdir format (no worktrees): %s", gitdir)
	}
	// Remove trailing slashes if any
	worktreeID = strings.TrimSuffix(worktreeID, "/")

	return worktreeID, nil
}
