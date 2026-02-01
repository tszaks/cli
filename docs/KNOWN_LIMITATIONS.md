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
