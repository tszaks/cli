package strategy

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/entireio/cli/cmd/entire/cli/paths"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// createRemoteWithCheckpoints sets up a bare repo with a main branch and an
// entire/checkpoints/v1 branch containing checkpoint data. Returns the bare
// repo dir and the commit hash on the metadata branch.
func createRemoteWithCheckpoints(t *testing.T) (string, plumbing.Hash) {
	t.Helper()

	bareDir := t.TempDir()
	bareRepo, err := git.PlainInit(bareDir, true)
	if err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}

	sig := object.Signature{Name: "Test", Email: "test@test.com"}

	// Create checkpoint data on entire/checkpoints/v1
	checkpointContent := []byte(`{"checkpoint_id": "test123"}`)
	blob := bareRepo.Storer.NewEncodedObject()
	blob.SetType(plumbing.BlobObject)
	blob.SetSize(int64(len(checkpointContent)))
	w, err := blob.Writer()
	if err != nil {
		t.Fatalf("failed to get blob writer: %v", err)
	}
	if _, err := w.Write(checkpointContent); err != nil {
		t.Fatalf("failed to write blob: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close blob writer: %v", err)
	}
	blobHash, err := bareRepo.Storer.SetEncodedObject(blob)
	if err != nil {
		t.Fatalf("failed to store blob: %v", err)
	}

	tree := &object.Tree{
		Entries: []object.TreeEntry{
			{Name: "metadata.json", Mode: 0o100644, Hash: blobHash},
		},
	}
	treeObj := bareRepo.Storer.NewEncodedObject()
	if err := tree.Encode(treeObj); err != nil {
		t.Fatalf("failed to encode tree: %v", err)
	}
	treeHash, err := bareRepo.Storer.SetEncodedObject(treeObj)
	if err != nil {
		t.Fatalf("failed to store tree: %v", err)
	}

	commit := &object.Commit{
		TreeHash: treeHash, Author: sig, Committer: sig,
		Message: "Checkpoint: test123\n",
	}
	commitObj := bareRepo.Storer.NewEncodedObject()
	if err := commit.Encode(commitObj); err != nil {
		t.Fatalf("failed to encode commit: %v", err)
	}
	commitHash, err := bareRepo.Storer.SetEncodedObject(commitObj)
	if err != nil {
		t.Fatalf("failed to store commit: %v", err)
	}

	metadataRef := plumbing.NewHashReference(
		plumbing.NewBranchReferenceName(paths.MetadataBranchName), commitHash,
	)
	if err := bareRepo.Storer.SetReference(metadataRef); err != nil {
		t.Fatalf("failed to set metadata branch ref: %v", err)
	}

	// Create main branch
	mainBlobHash := createBlobObject(t, bareRepo, []byte("test"))
	mainTree := &object.Tree{
		Entries: []object.TreeEntry{
			{Name: "README.md", Mode: 0o100644, Hash: mainBlobHash},
		},
	}
	mainTreeHash := createTreeObject(t, bareRepo, mainTree)

	mainCommit := &object.Commit{
		TreeHash: mainTreeHash, Author: sig, Committer: sig,
		Message: "Initial commit\n",
	}
	mainCommitHash := createCommitObject(t, bareRepo, mainCommit)

	if err := bareRepo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), mainCommitHash)); err != nil {
		t.Fatalf("failed to set main ref: %v", err)
	}
	if err := bareRepo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName("main"))); err != nil {
		t.Fatalf("failed to set HEAD: %v", err)
	}

	return bareDir, commitHash
}

func createBlobObject(t *testing.T, repo *git.Repository, content []byte) plumbing.Hash {
	t.Helper()
	blob := repo.Storer.NewEncodedObject()
	blob.SetType(plumbing.BlobObject)
	blob.SetSize(int64(len(content)))
	w, err := blob.Writer()
	if err != nil {
		t.Fatalf("failed to get blob writer: %v", err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatalf("failed to write blob: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close blob writer: %v", err)
	}
	hash, err := repo.Storer.SetEncodedObject(blob)
	if err != nil {
		t.Fatalf("failed to store blob: %v", err)
	}
	return hash
}

func createTreeObject(t *testing.T, repo *git.Repository, tree *object.Tree) plumbing.Hash {
	t.Helper()
	obj := repo.Storer.NewEncodedObject()
	if err := tree.Encode(obj); err != nil {
		t.Fatalf("failed to encode tree: %v", err)
	}
	hash, err := repo.Storer.SetEncodedObject(obj)
	if err != nil {
		t.Fatalf("failed to store tree: %v", err)
	}
	return hash
}

func createCommitObject(t *testing.T, repo *git.Repository, commit *object.Commit) plumbing.Hash {
	t.Helper()
	obj := repo.Storer.NewEncodedObject()
	if err := commit.Encode(obj); err != nil {
		t.Fatalf("failed to encode commit: %v", err)
	}
	hash, err := repo.Storer.SetEncodedObject(obj)
	if err != nil {
		t.Fatalf("failed to store commit: %v", err)
	}
	return hash
}

// cloneAndOpen clones bareDir via native git and opens with go-git.
func cloneAndOpen(t *testing.T, bareDir string) (*git.Repository, string) {
	t.Helper()
	cloneDir := filepath.Join(t.TempDir(), "clone")
	cmd := exec.CommandContext(context.Background(), "git", "clone", bareDir, cloneDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to clone: %v\noutput: %s", err, out)
	}
	repo, err := git.PlainOpenWithOptions(cloneDir, &git.PlainOpenOptions{
		EnableDotGitCommonDir: true,
	})
	if err != nil {
		t.Fatalf("failed to open cloned repo: %v", err)
	}
	return repo, cloneDir
}

// assertMetadataBranchHasData verifies the local metadata branch has non-empty tree
// data matching the expected commit hash.
func assertMetadataBranchHasData(t *testing.T, repo *git.Repository, expectedHash plumbing.Hash) {
	t.Helper()
	refName := plumbing.NewBranchReferenceName(paths.MetadataBranchName)
	localRef, err := repo.Reference(refName, true)
	if err != nil {
		t.Fatalf("local metadata branch not found: %v", err)
	}
	commit, err := repo.CommitObject(localRef.Hash())
	if err != nil {
		t.Fatalf("failed to get commit: %v", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		t.Fatalf("failed to get tree: %v", err)
	}
	if len(tree.Entries) == 0 {
		t.Error("local metadata branch has empty tree — checkpoint data was NOT preserved from remote")
	}
	if localRef.Hash() != expectedHash {
		t.Errorf("local branch hash %s != expected %s", localRef.Hash(), expectedHash)
	}
}

// TestEnsureMetadataBranch_FromRemote tests that EnsureMetadataBranch creates
// the local branch from the remote-tracking branch when available (fresh clone).
func TestEnsureMetadataBranch_FromRemote(t *testing.T) {
	bareDir, commitHash := createRemoteWithCheckpoints(t)
	repo, cloneDir := cloneAndOpen(t, bareDir)

	if err := EnsureMetadataBranch(repo); err != nil {
		t.Fatalf("EnsureMetadataBranch() failed: %v", err)
	}

	assertMetadataBranchHasData(t, repo, commitHash)

	// Verify via native git too
	cmd := exec.CommandContext(context.Background(), "git", "ls-tree", "refs/heads/"+paths.MetadataBranchName)
	cmd.Dir = cloneDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git ls-tree failed: %v\n%s", err, out)
	}
	if len(out) == 0 {
		t.Error("native git shows empty tree for local metadata branch")
	}
}

// TestEnsureMetadataBranch_EmptyOrphanUpdatedFromRemote is the critical test:
// if the local branch was previously created as an empty orphan (e.g., enable ran
// before the remote had data), and now the remote has data, ensure the local
// branch is updated from the remote.
func TestEnsureMetadataBranch_EmptyOrphanUpdatedFromRemote(t *testing.T) {
	bareDir, commitHash := createRemoteWithCheckpoints(t)
	repo, _ := cloneAndOpen(t, bareDir)

	// Simulate a pre-existing empty orphan (as if old enable created it)
	refName := plumbing.NewBranchReferenceName(paths.MetadataBranchName)
	emptyTreeHash := createTreeObject(t, repo, &object.Tree{Entries: []object.TreeEntry{}})

	sig := object.Signature{Name: "Test", Email: "test@test.com"}
	orphanHash := createCommitObject(t, repo, &object.Commit{
		TreeHash: emptyTreeHash, Author: sig, Committer: sig,
		Message: "Initialize metadata branch\n",
	})
	if err := repo.Storer.SetReference(plumbing.NewHashReference(refName, orphanHash)); err != nil {
		t.Fatalf("failed to set orphan ref: %v", err)
	}

	// Verify the local branch is currently empty
	localRef, err := repo.Reference(refName, true)
	if err != nil {
		t.Fatalf("failed to read local ref: %v", err)
	}
	localCommit, err := repo.CommitObject(localRef.Hash())
	if err != nil {
		t.Fatalf("failed to get commit: %v", err)
	}
	localTree, err := localCommit.Tree()
	if err != nil {
		t.Fatalf("failed to get tree: %v", err)
	}
	if len(localTree.Entries) != 0 {
		t.Fatal("pre-condition failed: local branch should be empty")
	}

	// Now call EnsureMetadataBranch — it should detect the empty local and update from remote
	if err := EnsureMetadataBranch(repo); err != nil {
		t.Fatalf("EnsureMetadataBranch() failed: %v", err)
	}

	assertMetadataBranchHasData(t, repo, commitHash)
}

// TestEnsureMetadataBranch_NoRemote tests that EnsureMetadataBranch creates
// an empty orphan when no remote branch exists.
func TestEnsureMetadataBranch_NoRemote(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}
	if _, err := wt.Add("test.txt"); err != nil {
		t.Fatalf("failed to add: %v", err)
	}
	if _, err := wt.Commit("Initial", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@test.com"},
	}); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	if err := EnsureMetadataBranch(repo); err != nil {
		t.Fatalf("EnsureMetadataBranch() failed: %v", err)
	}

	refName := plumbing.NewBranchReferenceName(paths.MetadataBranchName)
	ref, err := repo.Reference(refName, true)
	if err != nil {
		t.Fatalf("metadata branch not found: %v", err)
	}
	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		t.Fatalf("failed to get commit: %v", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		t.Fatalf("failed to get tree: %v", err)
	}
	if len(tree.Entries) != 0 {
		t.Errorf("expected empty tree, got %d entries", len(tree.Entries))
	}

	// Calling again should be a no-op
	if err := EnsureMetadataBranch(repo); err != nil {
		t.Fatalf("second EnsureMetadataBranch() failed: %v", err)
	}
}

// TestEnsureMetadataBranch_LocalWithDataNotOverwritten verifies that if the
// local branch already has checkpoint data, it is NOT overwritten by the remote.
func TestEnsureMetadataBranch_LocalWithDataNotOverwritten(t *testing.T) {
	bareDir, _ := createRemoteWithCheckpoints(t)
	repo, _ := cloneAndOpen(t, bareDir)

	// Create a local branch with different data (simulating local checkpoints)
	refName := plumbing.NewBranchReferenceName(paths.MetadataBranchName)
	localBlobHash := createBlobObject(t, repo, []byte(`{"checkpoint_id": "local456"}`))
	localTreeHash := createTreeObject(t, repo, &object.Tree{
		Entries: []object.TreeEntry{
			{Name: "local_data.json", Mode: 0o100644, Hash: localBlobHash},
		},
	})

	sig := object.Signature{Name: "Test", Email: "test@test.com"}
	localCommitHash := createCommitObject(t, repo, &object.Commit{
		TreeHash: localTreeHash, Author: sig, Committer: sig,
		Message: "Local checkpoint\n",
	})
	if err := repo.Storer.SetReference(plumbing.NewHashReference(refName, localCommitHash)); err != nil {
		t.Fatalf("failed to set local ref: %v", err)
	}

	// EnsureMetadataBranch should NOT overwrite the local data
	if err := EnsureMetadataBranch(repo); err != nil {
		t.Fatalf("EnsureMetadataBranch() failed: %v", err)
	}

	// Verify local branch still has its own data
	localRef, err := repo.Reference(refName, true)
	if err != nil {
		t.Fatalf("failed to read local ref: %v", err)
	}
	if localRef.Hash() != localCommitHash {
		t.Errorf("local branch was modified: got %s, want %s", localRef.Hash(), localCommitHash)
	}
}
