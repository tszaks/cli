package strategy

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"entire.io/cli/cmd/entire/cli/paths"
)

func TestGetGitDirInPath_RegularRepo(t *testing.T) {
	// Create a temp directory and initialize a real git repo
	tmpDir := t.TempDir()

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	result, err := getGitDirInPath(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(tmpDir, ".git")

	// Resolve symlinks for comparison (macOS /var -> /private/var)
	resultResolved, err := filepath.EvalSymlinks(result)
	if err != nil {
		t.Fatalf("failed to resolve symlinks for result: %v", err)
	}
	expectedResolved, err := filepath.EvalSymlinks(expected)
	if err != nil {
		t.Fatalf("failed to resolve symlinks for expected: %v", err)
	}

	if resultResolved != expectedResolved {
		t.Errorf("expected %s, got %s", expectedResolved, resultResolved)
	}
}

func TestGetGitDirInPath_Worktree(t *testing.T) {
	// Create a temp directory with a main repo and a worktree
	tmpDir := t.TempDir()
	mainRepo := filepath.Join(tmpDir, "main")
	worktreeDir := filepath.Join(tmpDir, "worktree")

	// Initialize main repo
	if err := os.MkdirAll(mainRepo, 0o755); err != nil {
		t.Fatalf("failed to create main repo dir: %v", err)
	}

	ctx := context.Background()

	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = mainRepo
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init main repo: %v", err)
	}

	// Configure git user for the commit
	cmd = exec.CommandContext(ctx, "git", "config", "user.email", "test@test.com")
	cmd.Dir = mainRepo
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to configure git email: %v", err)
	}

	cmd = exec.CommandContext(ctx, "git", "config", "user.name", "Test User")
	cmd.Dir = mainRepo
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to configure git name: %v", err)
	}

	// Create an initial commit (required for worktree)
	testFile := filepath.Join(mainRepo, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cmd = exec.CommandContext(ctx, "git", "add", ".")
	cmd.Dir = mainRepo
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}

	cmd = exec.CommandContext(ctx, "git", "commit", "-m", "initial")
	cmd.Dir = mainRepo
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to git commit: %v", err)
	}

	// Create a worktree
	cmd = exec.CommandContext(ctx, "git", "worktree", "add", worktreeDir, "-b", "feature")
	cmd.Dir = mainRepo
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Test that getGitDirInPath works in the worktree
	result, err := getGitDirInPath(worktreeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Resolve symlinks for comparison (macOS /var -> /private/var)
	resultResolved, err := filepath.EvalSymlinks(result)
	if err != nil {
		t.Fatalf("failed to resolve symlinks for result: %v", err)
	}
	expectedPrefix, err := filepath.EvalSymlinks(filepath.Join(mainRepo, ".git", "worktrees"))
	if err != nil {
		t.Fatalf("failed to resolve symlinks for expected prefix: %v", err)
	}

	// The git dir for a worktree should be inside main repo's .git/worktrees/
	if !strings.HasPrefix(resultResolved, expectedPrefix) {
		t.Errorf("expected git dir to be under %s, got %s", expectedPrefix, resultResolved)
	}
}

func TestGetGitDirInPath_NotARepo(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := getGitDirInPath(tmpDir)
	if err == nil {
		t.Fatal("expected error for non-repo directory, got nil")
	}

	expectedMsg := "not a git repository"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestInstallGitHook_Idempotent(t *testing.T) {
	// Create a temp directory and initialize a real git repo
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Clear cache so paths resolve correctly
	paths.ClearRepoRootCache()

	// First install should install hooks
	firstCount, err := InstallGitHook(true)
	if err != nil {
		t.Fatalf("First InstallGitHook() error = %v", err)
	}
	if firstCount == 0 {
		t.Error("First InstallGitHook() should install hooks (count > 0)")
	}

	// Second install should return 0 (all hooks already up to date)
	secondCount, err := InstallGitHook(true)
	if err != nil {
		t.Fatalf("Second InstallGitHook() error = %v", err)
	}
	if secondCount != 0 {
		t.Errorf("Second InstallGitHook() returned %d, want 0 (hooks unchanged)", secondCount)
	}
}
