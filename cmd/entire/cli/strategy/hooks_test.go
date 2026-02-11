package strategy

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/entireio/cli/cmd/entire/cli/paths"
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

	// Disable GPG signing for test commits
	cmd = exec.CommandContext(ctx, "git", "config", "commit.gpgsign", "false")
	cmd.Dir = mainRepo
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to configure commit.gpgsign: %v", err)
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

func TestIsGitSequenceOperation_NoOperation(t *testing.T) {
	// Create a temp directory and initialize a real git repo
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// No sequence operation in progress
	if isGitSequenceOperation() {
		t.Error("isGitSequenceOperation() = true, want false for clean repo")
	}
}

func TestIsGitSequenceOperation_RebaseMerge(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Simulate rebase-merge state
	rebaseMergeDir := filepath.Join(tmpDir, ".git", "rebase-merge")
	if err := os.MkdirAll(rebaseMergeDir, 0o755); err != nil {
		t.Fatalf("failed to create rebase-merge dir: %v", err)
	}

	if !isGitSequenceOperation() {
		t.Error("isGitSequenceOperation() = false, want true during rebase-merge")
	}
}

func TestIsGitSequenceOperation_RebaseApply(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Simulate rebase-apply state
	rebaseApplyDir := filepath.Join(tmpDir, ".git", "rebase-apply")
	if err := os.MkdirAll(rebaseApplyDir, 0o755); err != nil {
		t.Fatalf("failed to create rebase-apply dir: %v", err)
	}

	if !isGitSequenceOperation() {
		t.Error("isGitSequenceOperation() = false, want true during rebase-apply")
	}
}

func TestIsGitSequenceOperation_CherryPick(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Simulate cherry-pick state
	cherryPickHead := filepath.Join(tmpDir, ".git", "CHERRY_PICK_HEAD")
	if err := os.WriteFile(cherryPickHead, []byte("abc123"), 0o644); err != nil {
		t.Fatalf("failed to create CHERRY_PICK_HEAD: %v", err)
	}

	if !isGitSequenceOperation() {
		t.Error("isGitSequenceOperation() = false, want true during cherry-pick")
	}
}

func TestIsGitSequenceOperation_Revert(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Simulate revert state
	revertHead := filepath.Join(tmpDir, ".git", "REVERT_HEAD")
	if err := os.WriteFile(revertHead, []byte("abc123"), 0o644); err != nil {
		t.Fatalf("failed to create REVERT_HEAD: %v", err)
	}

	if !isGitSequenceOperation() {
		t.Error("isGitSequenceOperation() = false, want true during revert")
	}
}

func TestIsGitSequenceOperation_Worktree(t *testing.T) {
	// Test that detection works in a worktree (git dir is different)
	tmpDir := t.TempDir()
	mainRepo := filepath.Join(tmpDir, "main")
	worktreeDir := filepath.Join(tmpDir, "worktree")

	if err := os.MkdirAll(mainRepo, 0o755); err != nil {
		t.Fatalf("failed to create main repo dir: %v", err)
	}

	ctx := context.Background()

	// Initialize main repo with a commit
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = mainRepo
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init main repo: %v", err)
	}

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

	// Disable GPG signing for test commits
	cmd = exec.CommandContext(ctx, "git", "config", "commit.gpgsign", "false")
	cmd.Dir = mainRepo
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to configure commit.gpgsign: %v", err)
	}

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

	// Change to worktree
	t.Chdir(worktreeDir)

	// Should not detect sequence operation in clean worktree
	if isGitSequenceOperation() {
		t.Error("isGitSequenceOperation() = true in clean worktree, want false")
	}

	// Get the worktree's git dir and simulate rebase state there
	cmd = exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
	cmd.Dir = worktreeDir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get git dir: %v", err)
	}
	gitDir := strings.TrimSpace(string(output))

	rebaseMergeDir := filepath.Join(gitDir, "rebase-merge")
	if err := os.MkdirAll(rebaseMergeDir, 0o755); err != nil {
		t.Fatalf("failed to create rebase-merge dir in worktree: %v", err)
	}

	// Now should detect sequence operation
	if !isGitSequenceOperation() {
		t.Error("isGitSequenceOperation() = false in worktree during rebase, want true")
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

func TestRemoveGitHook_RemovesInstalledHooks(t *testing.T) {
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

	// Install hooks first
	installCount, err := InstallGitHook(true)
	if err != nil {
		t.Fatalf("InstallGitHook() error = %v", err)
	}
	if installCount == 0 {
		t.Fatal("InstallGitHook() should install hooks")
	}

	// Verify hooks are installed
	if !IsGitHookInstalled() {
		t.Fatal("hooks should be installed before removal test")
	}

	// Remove hooks
	removeCount, err := RemoveGitHook()
	if err != nil {
		t.Fatalf("RemoveGitHook() error = %v", err)
	}
	if removeCount != installCount {
		t.Errorf("RemoveGitHook() returned %d, want %d (same as installed)", removeCount, installCount)
	}

	// Verify hooks are removed
	if IsGitHookInstalled() {
		t.Error("hooks should not be installed after removal")
	}

	// Verify hook files no longer exist
	hooksDir := filepath.Join(tmpDir, ".git", "hooks")
	for _, hookName := range gitHookNames {
		hookPath := filepath.Join(hooksDir, hookName)
		if _, err := os.Stat(hookPath); !os.IsNotExist(err) {
			t.Errorf("hook file %s should not exist after removal", hookName)
		}
	}
}

func TestRemoveGitHook_NoHooksInstalled(t *testing.T) {
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

	// Remove hooks when none are installed - should handle gracefully
	removeCount, err := RemoveGitHook()
	if err != nil {
		t.Fatalf("RemoveGitHook() error = %v", err)
	}
	if removeCount != 0 {
		t.Errorf("RemoveGitHook() returned %d, want 0 (no hooks to remove)", removeCount)
	}
}

func TestRemoveGitHook_IgnoresNonEntireHooks(t *testing.T) {
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

	// Create a non-Entire hook manually
	hooksDir := filepath.Join(tmpDir, ".git", "hooks")
	customHookPath := filepath.Join(hooksDir, "pre-commit")
	customHookContent := "#!/bin/sh\necho 'custom hook'"
	if err := os.WriteFile(customHookPath, []byte(customHookContent), 0o755); err != nil {
		t.Fatalf("failed to create custom hook: %v", err)
	}

	// Remove hooks - should not remove the custom hook
	removeCount, err := RemoveGitHook()
	if err != nil {
		t.Fatalf("RemoveGitHook() error = %v", err)
	}
	if removeCount != 0 {
		t.Errorf("RemoveGitHook() returned %d, want 0 (custom hook should not be removed)", removeCount)
	}

	// Verify custom hook still exists
	if _, err := os.Stat(customHookPath); os.IsNotExist(err) {
		t.Error("custom hook should still exist after RemoveGitHook()")
	}
}

func TestRemoveGitHook_NotAGitRepo(t *testing.T) {
	// Create a temp directory without git init
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Clear cache so paths resolve correctly
	paths.ClearRepoRootCache()

	// Remove hooks in non-git directory - should return error
	_, err := RemoveGitHook()
	if err == nil {
		t.Fatal("RemoveGitHook() should return error for non-git directory")
	}
}

func TestInstallGitHook_BacksUpCustomHook(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
	paths.ClearRepoRootCache()

	// Create a custom prepare-commit-msg hook
	hooksDir := filepath.Join(tmpDir, ".git", "hooks")
	customHookPath := filepath.Join(hooksDir, "prepare-commit-msg")
	customContent := "#!/bin/sh\necho 'my custom hook'\n"
	if err := os.WriteFile(customHookPath, []byte(customContent), 0o755); err != nil {
		t.Fatalf("failed to create custom hook: %v", err)
	}

	count, err := InstallGitHook(true)
	if err != nil {
		t.Fatalf("InstallGitHook() error = %v", err)
	}
	if count == 0 {
		t.Error("InstallGitHook() should install hooks")
	}

	// Verify custom hook was backed up
	backupPath := customHookPath + backupSuffix
	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("backup file should exist at %s: %v", backupPath, err)
	}
	if string(backupData) != customContent {
		t.Errorf("backup content = %q, want %q", string(backupData), customContent)
	}

	// Verify installed hook has our marker and chain call
	hookData, err := os.ReadFile(customHookPath)
	if err != nil {
		t.Fatalf("hook file should exist: %v", err)
	}
	hookContent := string(hookData)
	if !strings.Contains(hookContent, entireHookMarker) {
		t.Error("installed hook should contain Entire marker")
	}
	if !strings.Contains(hookContent, chainComment) {
		t.Error("installed hook should contain chain call")
	}
	if !strings.Contains(hookContent, "prepare-commit-msg"+backupSuffix) {
		t.Error("chain call should reference the backup file")
	}
}

func TestInstallGitHook_DoesNotOverwriteExistingBackup(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
	paths.ClearRepoRootCache()

	hooksDir := filepath.Join(tmpDir, ".git", "hooks")

	// Create a backup file manually (simulating a previous backup)
	firstBackupContent := "#!/bin/sh\necho 'first custom hook'\n"
	backupPath := filepath.Join(hooksDir, "prepare-commit-msg"+backupSuffix)
	if err := os.WriteFile(backupPath, []byte(firstBackupContent), 0o755); err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	// Create a second custom hook at the standard path
	secondCustomContent := "#!/bin/sh\necho 'second custom hook'\n"
	hookPath := filepath.Join(hooksDir, "prepare-commit-msg")
	if err := os.WriteFile(hookPath, []byte(secondCustomContent), 0o755); err != nil {
		t.Fatalf("failed to create second custom hook: %v", err)
	}

	_, err := InstallGitHook(true)
	if err != nil {
		t.Fatalf("InstallGitHook() error = %v", err)
	}

	// Verify the original backup was NOT overwritten
	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("backup should still exist: %v", err)
	}
	if string(backupData) != firstBackupContent {
		t.Errorf("backup content = %q, want original %q", string(backupData), firstBackupContent)
	}

	// Verify our hook was installed with chain call
	hookData, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("hook should exist: %v", err)
	}
	if !strings.Contains(string(hookData), entireHookMarker) {
		t.Error("hook should contain Entire marker")
	}
	if !strings.Contains(string(hookData), chainComment) {
		t.Error("hook should contain chain call since backup exists")
	}
}

func TestInstallGitHook_IdempotentWithChaining(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
	paths.ClearRepoRootCache()

	// Create a custom hook, then install
	hooksDir := filepath.Join(tmpDir, ".git", "hooks")
	customHookPath := filepath.Join(hooksDir, "prepare-commit-msg")
	if err := os.WriteFile(customHookPath, []byte("#!/bin/sh\necho custom\n"), 0o755); err != nil {
		t.Fatalf("failed to create custom hook: %v", err)
	}

	firstCount, err := InstallGitHook(true)
	if err != nil {
		t.Fatalf("first InstallGitHook() error = %v", err)
	}
	if firstCount == 0 {
		t.Error("first install should install hooks")
	}

	// Re-install should return 0 (idempotent)
	secondCount, err := InstallGitHook(true)
	if err != nil {
		t.Fatalf("second InstallGitHook() error = %v", err)
	}
	if secondCount != 0 {
		t.Errorf("second InstallGitHook() = %d, want 0 (idempotent)", secondCount)
	}
}

func TestInstallGitHook_NoBackupWhenNoExistingHook(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
	paths.ClearRepoRootCache()

	_, err := InstallGitHook(true)
	if err != nil {
		t.Fatalf("InstallGitHook() error = %v", err)
	}

	// No .pre-entire files should exist
	hooksDir := filepath.Join(tmpDir, ".git", "hooks")
	for _, hook := range gitHookNames {
		backupPath := filepath.Join(hooksDir, hook+backupSuffix)
		if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
			t.Errorf("backup %s should not exist for fresh install", hook+backupSuffix)
		}

		// Hook should not contain chain call
		data, err := os.ReadFile(filepath.Join(hooksDir, hook))
		if err != nil {
			t.Fatalf("hook %s should exist: %v", hook, err)
		}
		if strings.Contains(string(data), chainComment) {
			t.Errorf("hook %s should not contain chain call for fresh install", hook)
		}
	}
}

func TestInstallGitHook_MixedHooks(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
	paths.ClearRepoRootCache()

	hooksDir := filepath.Join(tmpDir, ".git", "hooks")

	// Only create custom hooks for some hooks
	customHooks := map[string]string{
		"prepare-commit-msg": "#!/bin/sh\necho 'custom pcm'\n",
		"pre-push":           "#!/bin/sh\necho 'custom prepush'\n",
	}
	for name, content := range customHooks {
		hookPath := filepath.Join(hooksDir, name)
		if err := os.WriteFile(hookPath, []byte(content), 0o755); err != nil {
			t.Fatalf("failed to create %s: %v", name, err)
		}
	}

	_, err := InstallGitHook(true)
	if err != nil {
		t.Fatalf("InstallGitHook() error = %v", err)
	}

	// Hooks with pre-existing content should have backups and chain calls
	for name := range customHooks {
		backupPath := filepath.Join(hooksDir, name+backupSuffix)
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			t.Errorf("backup for %s should exist", name)
		}

		data, err := os.ReadFile(filepath.Join(hooksDir, name))
		if err != nil {
			t.Fatalf("hook %s should exist: %v", name, err)
		}
		if !strings.Contains(string(data), chainComment) {
			t.Errorf("hook %s should contain chain call", name)
		}
	}

	// Hooks without pre-existing content should NOT have backups or chain calls
	noCustom := []string{"commit-msg", "post-commit"}
	for _, name := range noCustom {
		backupPath := filepath.Join(hooksDir, name+backupSuffix)
		if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
			t.Errorf("backup for %s should NOT exist", name)
		}

		data, err := os.ReadFile(filepath.Join(hooksDir, name))
		if err != nil {
			t.Fatalf("hook %s should exist: %v", name, err)
		}
		if strings.Contains(string(data), chainComment) {
			t.Errorf("hook %s should NOT contain chain call", name)
		}
	}
}

func TestRemoveGitHook_RestoresBackup(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
	paths.ClearRepoRootCache()

	hooksDir := filepath.Join(tmpDir, ".git", "hooks")

	// Create a custom hook, install (backs it up), then remove
	customContent := "#!/bin/sh\necho 'my custom hook'\n"
	hookPath := filepath.Join(hooksDir, "prepare-commit-msg")
	if err := os.WriteFile(hookPath, []byte(customContent), 0o755); err != nil {
		t.Fatalf("failed to create custom hook: %v", err)
	}

	_, err := InstallGitHook(true)
	if err != nil {
		t.Fatalf("InstallGitHook() error = %v", err)
	}

	removed, err := RemoveGitHook()
	if err != nil {
		t.Fatalf("RemoveGitHook() error = %v", err)
	}
	if removed == 0 {
		t.Error("RemoveGitHook() should remove hooks")
	}

	// Original custom hook should be restored
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("hook should be restored: %v", err)
	}
	if string(data) != customContent {
		t.Errorf("restored hook content = %q, want %q", string(data), customContent)
	}

	// Backup should be gone
	backupPath := hookPath + backupSuffix
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("backup should be removed after restore")
	}
}

func TestRemoveGitHook_CleansUpMovedHooks(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
	paths.ClearRepoRootCache()

	// Install our hooks
	_, err := InstallGitHook(true)
	if err != nil {
		t.Fatalf("InstallGitHook() error = %v", err)
	}

	hooksDir := filepath.Join(tmpDir, ".git", "hooks")

	// Simulate another tool (e.g., husky) moving our hook to .pre-husky
	hookPath := filepath.Join(hooksDir, "prepare-commit-msg")
	movedPath := filepath.Join(hooksDir, "prepare-commit-msg.pre-husky")
	hookData, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("failed to read hook: %v", err)
	}
	if err := os.WriteFile(movedPath, hookData, 0o755); err != nil {
		t.Fatalf("failed to create moved hook: %v", err)
	}

	_, err = RemoveGitHook()
	if err != nil {
		t.Fatalf("RemoveGitHook() error = %v", err)
	}

	// The moved hook with our marker should be cleaned up
	if _, err := os.Stat(movedPath); !os.IsNotExist(err) {
		t.Error("moved hook with Entire marker should be removed")
	}
}

func TestRemoveGitHook_IgnoresMovedNonEntireHooks(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
	paths.ClearRepoRootCache()

	// Install our hooks
	_, err := InstallGitHook(true)
	if err != nil {
		t.Fatalf("InstallGitHook() error = %v", err)
	}

	hooksDir := filepath.Join(tmpDir, ".git", "hooks")

	// Create a .pre-foobar file that does NOT contain our marker
	otherContent := "#!/bin/sh\necho 'some other tool hook'\n"
	otherPath := filepath.Join(hooksDir, "prepare-commit-msg.pre-foobar")
	if err := os.WriteFile(otherPath, []byte(otherContent), 0o755); err != nil {
		t.Fatalf("failed to create other hook: %v", err)
	}

	_, err = RemoveGitHook()
	if err != nil {
		t.Fatalf("RemoveGitHook() error = %v", err)
	}

	// The non-Entire moved hook should be left alone
	data, err := os.ReadFile(otherPath)
	if err != nil {
		t.Fatal("non-Entire moved hook should still exist")
	}
	if string(data) != otherContent {
		t.Error("non-Entire moved hook content should be unchanged")
	}
}

func TestRemoveGitHook_RestoresBackupWhenHookAlreadyGone(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
	paths.ClearRepoRootCache()

	hooksDir := filepath.Join(tmpDir, ".git", "hooks")

	// Create custom hook, install (creates backup), then delete the main hook
	customContent := "#!/bin/sh\necho 'original'\n"
	hookPath := filepath.Join(hooksDir, "prepare-commit-msg")
	if err := os.WriteFile(hookPath, []byte(customContent), 0o755); err != nil {
		t.Fatalf("failed to create custom hook: %v", err)
	}

	_, err := InstallGitHook(true)
	if err != nil {
		t.Fatalf("InstallGitHook() error = %v", err)
	}

	// Simulate another tool deleting our hook
	if err := os.Remove(hookPath); err != nil {
		t.Fatalf("failed to remove hook: %v", err)
	}

	_, err = RemoveGitHook()
	if err != nil {
		t.Fatalf("RemoveGitHook() error = %v", err)
	}

	// Backup should be restored even though the main hook was already gone
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal("backup should be restored to main hook path")
	}
	if string(data) != customContent {
		t.Errorf("restored hook content = %q, want %q", string(data), customContent)
	}

	// Backup file should be gone
	backupPath := hookPath + backupSuffix
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("backup file should not exist after restore")
	}
}

func TestRemoveGitHook_PermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Test cannot run as root (permission checks are bypassed)")
	}

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

	// Install hooks first
	_, err := InstallGitHook(true)
	if err != nil {
		t.Fatalf("InstallGitHook() error = %v", err)
	}

	// Remove write permissions from hooks directory to cause permission error
	hooksDir := filepath.Join(tmpDir, ".git", "hooks")
	if err := os.Chmod(hooksDir, 0o555); err != nil {
		t.Fatalf("failed to change hooks dir permissions: %v", err)
	}
	// Restore permissions on cleanup
	t.Cleanup(func() {
		_ = os.Chmod(hooksDir, 0o755) //nolint:errcheck // Cleanup, best-effort
	})

	// Remove hooks should now fail with permission error
	removed, err := RemoveGitHook()
	if err == nil {
		t.Fatal("RemoveGitHook() should return error when hooks cannot be deleted")
	}
	if removed != 0 {
		t.Errorf("RemoveGitHook() removed %d hooks, expected 0 when all fail", removed)
	}
	if !strings.Contains(err.Error(), "failed to remove hooks") {
		t.Errorf("error should mention 'failed to remove hooks', got: %v", err)
	}
}
