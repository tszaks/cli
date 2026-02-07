# Known Limitations

This document describes known limitations of the Entire CLI.

## Git Operations

### Amending Commits Loses Checkpoint Link

When you amend a commit that has an `Entire-Checkpoint` trailer using `git commit --amend -m "new message"`, the checkpoint link is lost because the `-m` flag replaces the entire commit message.

**Impact:**
- The link between your code commit and the session metadata on `entire/sessions` is broken
- `entire explain` can no longer find the associated session transcript
- The checkpoint data still exists but is orphaned

**Workarounds:**

1. **Amend without `-m`**: Use `git commit --amend` (without `-m`) to open your editor, which preserves the existing message including the trailer

2. **Manually preserve the trailer**: If you must use `-m`, first note the checkpoint ID:
   ```bash
   git log -1 --format=%B | grep "Entire-Checkpoint"
   ```
   Then include it in your new message:
   ```bash
   git commit --amend -m "new message

   Entire-Checkpoint: <id-from-above>"
   ```

3. **Re-add after amend**: If you forgot, you can amend again to add the trailer back (if you still have the checkpoint ID from `entire explain` or the reflog)

**Tracked in:** [ENT-161](https://linear.app/entirehq/issue/ENT-161)

### Git GC Can Corrupt Worktree Indexes

When using git worktrees, `git gc --auto` can corrupt a worktree's index by pruning loose objects that the worktree's index cache-tree references. This manifests as:

```
fatal: unable to read <hash>
error: invalid sha1 pointer in cache-tree of .git/worktrees/<n>/index
```

**Root cause:** Checkpoint saves use go-git's `SetEncodedObject` which creates loose objects. When the count exceeds the `gc.auto` threshold (default 6700), any git operation (e.g., VS Code or Sourcetree background fetch) triggers `git gc --auto`. GC doesn't fully account for worktree index references when pruning, so objects get deleted while the worktree index still points to them.

**Impact:**
- `git status` fails in the affected worktree
- Staged changes in the worktree are lost

**Recovery:**
```bash
# In the affected worktree:
git read-tree HEAD
```
This rebuilds the index from HEAD. Any previously staged changes will need to be re-staged.

**Prevention:** Disable auto-GC and run it manually after commits (when indexes are clean):
```bash
git config gc.auto 0
```

**Tracked in:** [ENT-241](https://linear.app/entirehq/issue/ENT-241)
