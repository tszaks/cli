# Entire CLI User Guide

Entire hooks into your git workflow to capture AI agent sessions on every push. Sessions are indexed alongside commits, creating a searchable record of how code was written. Runs locally, stays in your repo.

---

## Quick Start

```bash
# Install
curl -fsSL https://entire.io/install.sh | bash

# Enable in your project
cd your-project && entire enable

# Check status
entire status

# Work with Claude Code
claude

# Resume a previous session
entire resume <branch>
```

---

## Table of Contents

1. [Installation & Setup](#installation--setup)
2. [Configuration](#configuration)
3. [Core Concepts](#core-concepts)
   - [Sessions](#sessions)
   - [Checkpoints](#checkpoints)
   - [Strategies](#strategies)
   - [Hooks](#hooks)
   - [Shadow Branches](#shadow-branches)
   - [The entire/sessions Branch](#the-entiresessions-branch)
4. [Commands Reference](#commands-reference)
5. [Workflow Examples](#workflow-examples)
6. [Collaboration](#collaboration)
7. [Storage & Privacy](#storage--privacy)
8. [Concurrent Usage](#concurrent-usage)
9. [Upgrading Entire](#upgrading-entire)
10. [Uninstallation](#uninstallation)
11. [Troubleshooting](#troubleshooting)
12. [Quick Reference](#quick-reference)
13. [Glossary](#glossary)
14. [Getting Help](#getting-help)

---

## Installation & Setup

### Prerequisites

- Git 2.x or higher
- A git repository to work in
- Claude Code (for AI agent integration)

### Installation

```bash
# Option 1: Install script (recommended)
curl -fsSL https://entire.io/install.sh | bash

# Option 2: Homebrew
brew install entire

# Option 3: Build from source
git clone https://github.com/entireio/cli.git
cd cli
go build -o entire ./cmd/entire
sudo mv entire /usr/local/bin/
```

### Initial Setup

1. Navigate to your git repository:
   ```bash
   cd /path/to/your/project
   ```

2. Enable Entire:
   ```bash
   entire enable
   ```

3. Select a strategy when prompted:
   ```
   > manual-commit  Sessions are only captured when you commit
     auto-commit    Automatically capture sessions after agent response completion
   ```

4. If project settings already exist, choose where to save:
   ```
   > Update project settings (settings.json)
     Use local settings (settings.local.json, gitignored)
   ```

**Expected output:**
```
✓ Claude Code hooks installed
✓ .entire directory created
✓ Project settings saved (.entire/settings.json)
✓ Git hooks installed

✓ manual-commit strategy enabled
```

---

## Configuration

Entire uses two configuration files in the `.entire/` directory:

### settings.json (Project Settings)

Shared across the team, typically committed to git:

```json
{
  "strategy": "manual-commit",
  "agent": "claude-code",
  "enabled": true
}
```

### settings.local.json (Local Settings)

Personal overrides, gitignored by default:

```json
{
  "enabled": false,
  "log_level": "debug"
}
```

### Configuration Options

| Option | Values | Description |
|--------|--------|-------------|
| `strategy` | `manual-commit`, `auto-commit` | Session capture strategy |
| `enabled` | `true`, `false` | Enable/disable Entire |
| `agent` | `claude-code`, `gemini`, etc. | AI agent to integrate with |
| `agent_auto_detect` | `true`, `false` | Auto-detect agent if not set (default: true) |
| `log_level` | `debug`, `info`, `warn`, `error` | Logging verbosity |
| `strategy_options` | object | Strategy-specific settings (e.g., `{"push_sessions": true}`) |
| `agent_options` | object | Agent-specific settings, keyed by agent name |

### Settings Priority

Local settings override project settings. The merge behavior works field-by-field:

```json
// .entire/settings.json (project)
{
  "strategy": "manual-commit",
  "enabled": true,
  "log_level": "info"
}

// .entire/settings.local.json (local override)
{
  "enabled": false,
  "log_level": "debug"
}

// Effective settings (merged result):
// - strategy: "manual-commit" (from project, not overridden)
// - enabled: false (local wins)
// - log_level: "debug" (local wins)
```

When you run `entire status`:
```
Project, enabled (manual-commit)
Local, disabled (manual-commit)
```

The effective setting is the Local line (disabled in this example).

### Shell Autocompletion

During `entire enable`, you'll be prompted to add shell completion to your rc file. You can also set it up manually:

```bash
# Option 1: During enable (non-interactive)
entire enable --strategy manual-commit --setup-shell

# Option 2: Load in current session
source <(entire completion zsh)   # Zsh
source <(entire completion bash)  # Bash

# Option 3: Install permanently (Zsh)
entire completion zsh > "${fpath[1]}/_entire"
```

### The .entire/ Directory

The `.entire/` directory contains all Entire-related files:

```
.entire/
├── .gitignore              # Ignores local/temporary files
├── settings.json           # Project settings (committed)
├── settings.local.json     # Local overrides (gitignored)
├── current_session         # Active session ID (gitignored)
├── metadata/               # Session transcripts during active session (gitignored)
│   └── <session-id>/
│       ├── full.jsonl      # Complete transcript
│       ├── metadata.json   # Session metadata
│       ├── prompt.txt      # User prompts
│       ├── context.md      # Generated context
│       └── summary.txt     # Session summary
├── tmp/                    # Temporary state files (gitignored)
└── logs/                   # Debug logs (gitignored)
```

**What gets committed:** Only `settings.json` is committed to your repository (if you chose project settings during setup). The `settings.local.json` file and everything else is gitignored.

---

## Core Concepts

### Sessions

A **session** represents a complete interaction with your AI agent, from start to finish. Each session captures:

- All prompts you sent to the agent
- All responses from the agent
- Files created, modified, or deleted
- Timestamps and metadata

**Session properties:**
- **ID**: Unique identifier in format `YYYY-MM-DD-<UUID>` (e.g., `2025-01-08-abc123de-f456-7890-abcd-ef1234567890`)
- **Strategy**: Which strategy created this session (`manual-commit` or `auto-commit`)
- **Description**: Human-readable summary (typically derived from your first prompt)
- **Checkpoints**: List of save points within the session

Sessions are stored separately from your code commits on the `entire/sessions` branch.

### Checkpoints

A **checkpoint** is a snapshot within a session that you can rewind to. Think of it as a "save point" in your work.

**When checkpoints are created:**
- **Manual-commit strategy**: When you make a git commit
- **Auto-commit strategy**: After each agent response

**What checkpoints contain:**
- Current file state
- Session transcript up to that point
- Metadata (timestamp, checkpoint ID, etc.)

**Checkpoint IDs** are 12-character hex strings (e.g., `a3b2c4d5e6f7`).

**Checkpoints enable:**
- **Rewinding**: Restore code to any previous checkpoint state
- **Resuming**: Continue sessions with full context restored
- **Cross-machine restoration**: Resume sessions on different machines by fetching the `entire/sessions` branch
- **PR-ready commits**: Squash checkpoint history into clean commits for pull requests

### Strategies

Entire offers two strategies for capturing your work:

| Aspect | Manual-Commit | Auto-Commit |
|--------|---------------|-------------|
| Code commits | None on your branch - you control when to commit | Created automatically after each agent response |
| Checkpoint storage | Shadow branches (`entire/<hash>`) | `entire/sessions` branch |
| Safe on main branch | Yes | No - creates commits |
| Rewind | Always possible, non-destructive | Full rewind on feature branches; logs-only on main |
| Best for | Most workflows - keeps git history clean | Teams wanting automatic code commits |

### Hooks

Hooks are how Entire integrates with Claude Code and other AI agents. When your agent runs, it triggers hooks that allow Entire to:

- Start tracking a new session
- Create checkpoints at appropriate times
- Save session transcripts
- Handle subagent (Task tool) checkpoints

**What gets installed:**

1. **Agent hooks** in `.claude/settings.json` (for Claude Code):
   - `SessionStart` - Begins session tracking
   - `UserPromptSubmit` - Records prompts
   - `Stop` - Saves final checkpoint
   - `PreToolUse` / `PostToolUse` - Handles tool interactions

2. **Git hooks** in `.git/hooks/`:
   - `prepare-commit-msg` - Adds session trailers to commit messages
   - `commit-msg` - Validates and updates trailers
   - `post-commit` - Saves checkpoint after commit
   - `pre-push` - Pushes `entire/sessions` branch alongside your code

**Verifying hooks are working:**
- Check `.claude/settings.json` for hook entries containing "entire hooks"
- Check `.git/hooks/` for Entire-managed hook scripts

Hooks are installed and verified when you run `entire enable`.

### Shadow Branches

Shadow branches are ephemeral branches used by the **manual-commit strategy** to store checkpoints without polluting your working branch.

**How they work:**
- **Created when**: First checkpoint is saved in a manual-commit session
- **Naming**: `entire/<commit-hash>` where hash is your current base commit
- **Purpose**: Store file snapshots and transcripts between your commits
- **Deleted when**: Session is "condensed" after you make a commit (data moves to `entire/sessions`)

**Viewing shadow branches:**
```bash
git branch | grep 'entire/'
```

Shadow branches are local-only and don't get pushed to your remote.

### The entire/sessions Branch

The `entire/sessions` branch is an orphan branch (no parent commits) that stores all session metadata permanently.

**Characteristics:**
- **Auto-created**: On first checkpoint, not during `entire enable`
- **Orphan branch**: Has no parent commits, separate from your code history
- **Sharded storage**: Checkpoints stored in `<first-2-chars>/<remaining-10-chars>/` structure
- **Pushed automatically**: Via `pre-push` hook (unless `--skip-push-sessions` was used)

**Directory structure on entire/sessions:**
```
<checkpoint-id[:2]>/<checkpoint-id[2:]>/
├── metadata.json       # Checkpoint info
├── full.jsonl          # Complete transcript
├── prompt.txt          # User prompts
├── context.md          # Generated context
├── summary.txt         # Session summary
└── content_hash.txt    # SHA256 of transcript
```

**Example**: Checkpoint `a3b2c4d5e6f7` is stored at `a3/b2c4d5e6f7/`.

---

## Commands Reference

### entire enable

Initialize Entire in your repository.

```bash
entire enable                           # Interactive setup
entire enable --strategy manual-commit  # Skip strategy prompt
entire enable --local                   # Save to settings.local.json
entire enable --project                 # Save to settings.json
entire enable --force                   # Reinstall hooks
entire enable --setup-shell             # Add shell completion to rc file
entire enable --skip-push-sessions      # Disable auto-push of session logs
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--strategy` | Strategy to use: `manual-commit` or `auto-commit` |
| `--local` | Write settings to `settings.local.json` |
| `--project` | Write to `settings.json` even if it exists |
| `-f, --force` | Force reinstall hooks |
| `--setup-shell` | Add shell completion to rc file (non-interactive) |
| `--skip-push-sessions` | Disable automatic pushing of session logs on git push |
| `--agent` | Agent to setup hooks for (e.g., `claude-code`, `gemini`) |

**Example output:**
```
✓ Claude Code hooks installed
✓ Gemini CLI hooks verified
✓ Project settings saved (.entire/settings.json)
✓ Git hooks installed

✓ manual-commit strategy enabled
```

### entire disable

Temporarily disable Entire. Hooks will exit silently without errors - this is expected and safe behavior.

```bash
entire disable                  # Writes to settings.local.json
entire disable --project        # Writes to settings.json
```

**Example output:**
```
Entire is now disabled.
```

To re-enable, run `entire enable`.

### entire status

Show current Entire status and configuration.

```bash
entire status
```

**Example output (both settings files exist):**
```
Project, enabled (manual-commit)
Local, disabled (manual-commit)
```

**Example output (not set up):**
```
○ not set up (run `entire enable` to get started)
```

### entire rewind

Restore code to a previous checkpoint.

```bash
entire rewind                        # Interactive selection
entire rewind --list                 # List all rewind points as JSON
entire rewind --to <checkpoint-id>   # Rewind to specific checkpoint
entire rewind --logs-only            # Restore logs only (not files)
```

**Example output (--list):**
```json
[
  {
    "id": "097e675a0ede1789d6e806a477bde5e9af378110",
    "message": "more similar issues with CWD",
    "date": "2026-01-12T21:40:44+01:00",
    "is_task_checkpoint": false,
    "is_logs_only": true,
    "condensation_id": "2372de7ea2ff"
  }
]
```

### entire rewind reset

Reset the shadow branch for the current commit (manual-commit strategy).

```bash
entire rewind reset              # Prompts for confirmation
entire rewind reset --force      # Skip confirmation
```

**When to use:** If you see a shadow branch conflict error, this gives you a clean start. See [Troubleshooting](#troubleshooting).

### entire session

Manage and view sessions. All session-related commands are subcommands of `entire session`.

#### entire session list

List all sessions stored by the current strategy.

```bash
entire session list
```

**Example output:**
```
  session-id           Checkpoints  Description
  ───────────────────  ───────────  ────────────────────────────────────────
  2026-01-13-21a2e002  4            I think the checkpoint id is less important...
  2026-01-13-dc577197  3            can you update CLAUDE.md to reflect the new...
  2026-01-12-deeac353  1            can you look at the changes in this branch...

Resume a session: entire session resume <session-id>
```

#### entire session current

Show details of the current session.

```bash
entire session current
```

**Example output:**
```
Session:     2026-01-13-21a2e002-8b63-45db-8e0d-1978fe0600aa
Strategy:    manual-commit
Description: I think the checkpoint id is less important...
Started:     2026-01-13 20:36
Checkpoints: 4
```

#### entire session raw

Output the raw session transcript for a commit.

```bash
entire session raw <commit-sha>
```

#### entire session resume

Resume a session and restore agent memory. Supports prefix matching for session IDs.

```bash
entire session resume                # Interactive picker
entire session resume <session-id>   # Resume specific session
```

**Prefix matching:** You don't need the full session ID. The first matching session is used:
```bash
entire session resume 2025-01        # First session from Jan 2025
entire session resume 2025-01-13     # First session from Jan 13
entire session resume 2025-01-13-8f  # More specific match
```

#### entire session cleanup

Remove orphaned session data that wasn't cleaned up automatically. This finds and removes:

- **Shadow branches** (`entire/<commit-hash>`) - Created by manual-commit strategy
- **Session state files** (`.git/entire-sessions/`) - Track active sessions
- **Checkpoint metadata** (`entire/sessions` branch) - For auto-commit checkpoints

```bash
entire session cleanup           # Dry run, shows what would be removed
entire session cleanup --force   # Actually delete orphaned items
```

### entire resume

Switch to a branch and resume its session with agent memory.

```bash
entire resume <branch>
entire resume <branch> --force   # Resume without confirmation if newer commits exist
```

**How it differs from `entire session resume`:**
- `entire resume <branch>` - Switches to the branch AND restores its session
- `entire session resume [id]` - Restores a session WITHOUT changing branches

Use `entire resume` when switching between feature branches. Use `entire session resume` when you want to restore a session on your current branch.

**Example:**
```bash
entire resume feature/new-thing
```

**Example output:**
```
Switched to branch 'feature/new-thing'
Session restored to: .claude/projects/.../2025-01-08-abc123.jsonl
Session: 2025-01-08-abc123def456

To continue this session, run:
  claude --resume
```

### entire explain

Get a human-readable explanation of sessions or commits.

```bash
entire explain                       # Explain current session
entire explain --session <id>        # Explain specific session (prefix match supported)
entire explain --commit <sha>        # Explain specific commit
entire explain --no-pager            # Don't use pager for output
```

**Example output:**
```
Session: 2026-01-13-21a2e002-8b63-45db-8e0d-1978fe0600aa
Strategy: manual-commit
Started: 2026-01-13 20:36:00
Source Ref: entire/sessions@abc123def456
Checkpoints: 4

─── Checkpoint 1 [a3b2c4d5e6f7] 2026-01-13 20:40 ───

## Prompt

Can you update the README with installation instructions?

## Responses

I'll update the README with comprehensive installation instructions...

Files Modified (2):
  - README.md
  - docs/INSTALL.md
```

### entire version

Show version and build information.

```bash
entire version
```

**Example output:**
```
Entire CLI v1.0.0 (abc123def)
Go version: go1.24.0
OS/Arch: darwin/arm64
```

---

## Workflow Examples

### Manual-Commit Workflow

The recommended workflow for most users. Entire captures your sessions without creating commits on your working branch.

```bash
# 1. Set up Entire in your project
cd my-project
entire enable                    # Select manual-commit strategy

# 2. Work with Claude Code
claude                           # Entire captures your session automatically

# 3. Check what you've done
entire explain                   # Shows session activity

# 4. Commit your changes
git add .
git commit -m "Implement feature"

# 5. Review completed work
entire explain                   # Now shows what was done in the commit

# 6. Push to remote (sessions pushed automatically)
git push
```

### Auto-Commit Workflow

For teams that want automatic commits after each agent response. Best used on feature branches.

```bash
# 1. Set up with auto-commit strategy
git checkout -b feature/new-thing
entire enable --strategy auto-commit

# 2. Work with Claude Code
claude                           # Creates checkpoints automatically

# 3. Review what happened
entire explain

# 4. Need to undo some changes? Rewind to a checkpoint
entire rewind --list             # See available checkpoints
entire rewind --to <checkpoint>  # Restore to that point

# 5. Continue working
claude

# 6. Review and push
entire explain
git push
```

### Resuming Work

When returning to a branch you were working on:

```bash
# Switch to branch and restore your session in one command
entire resume feature/new-thing

# This does two things:
# 1. Checks out the branch
# 2. Restores your Claude session

# Review where you left off
entire explain

# Continue working with Claude (session context restored)
claude --resume
```

### Reviewing Session History

```bash
# See what happened in current session
entire explain

# Explain a specific session
entire explain --session <session-id>

# Explain what went into a specific commit
entire explain --commit <sha>

# View raw transcript
entire session raw <commit-sha>

# List all sessions
entire session list
```

---

## Collaboration

### Team Setup

1. **Commit project settings:** The `.entire/settings.json` file should be committed so the team shares the same strategy.

2. **Local overrides:** Individual developers can use `.entire/settings.local.json` for personal preferences (this file is gitignored).

3. **Session sharing:** Session data is stored on the `entire/sessions` git branch. Team members can:
   - Fetch this branch to see session history
   - Use `entire explain` to understand what happened in commits
   - Resume sessions started by others (on the same branch)
   - View transcripts to understand how code was written

### Fetching Team Sessions

```bash
git fetch origin entire/sessions:entire/sessions
entire session list
entire explain --session <session-id>
```

---

## Storage & Privacy

### Where Data is Stored

**During active session:**
- Transcripts: `.entire/metadata/<session-id>/` (local, gitignored)
- Session state: `.git/entire-sessions/<session-id>.json`

**After commit (manual-commit strategy):**
- Transcripts condensed to `entire/sessions` branch
- Local `.entire/metadata/` cleaned up
- Shadow branches deleted

**What transcripts contain:**
- Full prompts you sent to the agent
- Full responses from the agent
- File paths that were modified
- Tool usage details

### Sensitive Data Considerations

Transcripts may contain:
- Code snippets and file contents
- API keys or secrets if mentioned in prompts
- Internal project details

**Recommendations:**
- Avoid pasting secrets directly into prompts
- Review transcripts before pushing if working with sensitive code
- Use `.gitignore` to exclude sensitive files from agent context

### Purging Session Data

To completely remove all session data:

```bash
# Delete the entire/sessions branch (local)
git branch -D entire/sessions

# Delete remote branch (if pushed)
git push origin --delete entire/sessions

# Clean up local metadata
rm -rf .entire/metadata/

# Clean up any remaining shadow branches
git branch | grep 'entire/' | xargs git branch -D
```

---

## Concurrent Usage

### Multiple Developers

Each developer gets their own independent sessions:

```bash
# Developer A on their machine
entire enable
claude                    # Creates session 2025-01-13-abc123

# Developer B on their machine (same repo)
entire enable
claude                    # Creates session 2025-01-13-def456
```

**No conflicts:** Sessions use UUIDs, so they're always unique even if started at the same time.

### Multiple Claude Instances

You can run multiple Claude instances in the same repository:

```bash
# Terminal 1
claude                    # Session: 2025-01-13-abc123

# Terminal 2 (same repo, different terminal)
claude                    # Session: 2025-01-13-def456
```

Each instance creates its own session with a unique UUID.

### Different Branches, Same Repo

Sessions are associated with the branch where they were created:

```bash
# On feature/auth branch
git checkout feature/auth
claude                    # Session attached to feature/auth

# Switch to feature/ui branch
git checkout feature/ui
claude                    # New session attached to feature/ui

# Later, resume the auth session
entire resume feature/auth
claude --resume           # Continues the auth session
```

### Parallel Feature Branches

```bash
# Developer A: feature/auth
git checkout -b feature/auth
entire enable
claude                    # Works on auth feature

# Developer B: feature/ui (same time, same repo)
git checkout -b feature/ui
entire enable
claude                    # Works on UI feature

# Both can push without conflicts
git push origin feature/auth    # Pushes auth sessions
git push origin feature/ui      # Pushes UI sessions
```

The `entire/sessions` branch receives all sessions and merges them automatically.

---

## Upgrading Entire

### Checking Your Version

```bash
entire version
```

### Upgrade Methods

**Via install script (same as initial install):**
```bash
curl -fsSL https://entire.io/install.sh | bash
```

**Via Homebrew:**
```bash
brew update && brew upgrade entire
```

**From source:**
```bash
cd cli
git pull
go build -o entire ./cmd/entire
sudo mv entire /usr/local/bin/
```

### After Upgrading

Reinstall hooks to ensure they're up-to-date:

```bash
entire enable --force
```

This reinstalls all hooks without changing your strategy or settings.

---

## Uninstallation

To completely remove Entire from a repository:

### 1. Remove Agent Hooks

For Claude Code, edit `.claude/settings.json` and remove the hooks section containing "entire hooks".

### 2. Delete Entire Branches

```bash
# Delete local branches
git branch -D entire/sessions
git branch | grep 'entire/' | xargs git branch -D 2>/dev/null

# Delete remote branch (if pushed)
git push origin --delete entire/sessions 2>/dev/null
```

### 3. Remove .entire/ Directory

```bash
rm -rf .entire/
```

### 4. Remove Git Hooks (Optional)

Entire's git hooks are in `.git/hooks/`. They're designed to exit silently when Entire is disabled, but you can remove them:

```bash
# Check for Entire hooks
grep -l "entire" .git/hooks/*

# Remove if desired (backup first)
# rm .git/hooks/prepare-commit-msg
# rm .git/hooks/commit-msg
# rm .git/hooks/post-commit
# rm .git/hooks/pre-push
```

### 5. Uninstall the Binary

**If installed via script:**
```bash
sudo rm /usr/local/bin/entire
```

**If installed via Homebrew:**
```bash
brew uninstall entire
```

---

## Troubleshooting

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| "Not a git repository" | Running `entire` outside a git repo | Navigate to a git repository first |
| "Entire is disabled" | `enabled: false` in settings | Run `entire enable` |
| "No rewind points found" | No checkpoints created yet | Work with Claude Code and commit (manual-commit) or wait for agent response (auto-commit) |
| "shadow branch conflict" | Another session using same base commit | Run `entire rewind reset --force` (see [Resetting State](#resetting-state)) |
| "session not found" | Session ID doesn't match any stored sessions | Check available sessions with `entire session list` |

### Debug Mode

Enable debug logging to troubleshoot issues:

```bash
# Via settings.local.json
{
  "log_level": "debug"
}

# Or via environment variable
ENTIRE_LOG_LEVEL=debug entire status
```

### Resetting State

If things get into a bad state:

```bash
# Reset shadow branch for current commit
entire rewind reset --force

# Clean up orphaned data
entire session cleanup --force

# Disable and re-enable
entire disable
entire enable --force
```

### Recovering from Interrupted Sessions

If a session was interrupted (crash, force quit, etc.):

1. **Check for orphaned state:**
   ```bash
   entire session cleanup
   ```

2. **Clean up if needed:**
   ```bash
   entire session cleanup --force
   ```

3. **Start fresh:**
   ```bash
   entire enable --force
   ```

### Accessibility

For screen reader users, enable accessible mode:

```bash
export ACCESSIBLE=1
entire enable
```

This uses simpler text prompts instead of interactive TUI elements.

---

## Quick Reference

| Command | Description |
|---------|-------------|
| `entire enable` | Set up Entire in repository |
| `entire disable` | Temporarily disable Entire |
| `entire status` | Show current status |
| `entire rewind` | Rewind to a checkpoint |
| `entire rewind reset` | Reset shadow branch for current commit |
| `entire session list` | List all sessions |
| `entire session current` | Show current session details |
| `entire session resume` | Resume a session and restore agent memory |
| `entire session cleanup` | Remove orphaned session data |
| `entire resume <branch>` | Switch to branch and resume its session |
| `entire explain` | Explain current session or commit |
| `entire version` | Show version info |

### Flags Quick Reference

| Flag | Commands | Description |
|------|----------|-------------|
| `--project` | enable, disable | Write to settings.json |
| `--local` | enable | Write to settings.local.json |
| `--strategy` | enable | Set strategy (manual-commit or auto-commit) |
| `--setup-shell` | enable | Add shell completion to rc file |
| `--skip-push-sessions` | enable | Don't auto-push session logs |
| `--force, -f` | enable, resume, rewind reset, session cleanup | Skip confirmations / force reinstall |
| `--list` | rewind | List rewind points as JSON |
| `--to <id>` | rewind | Specify checkpoint ID |
| `--logs-only` | rewind | Restore logs only, not files |
| `--session <id>` | explain | Explain specific session |
| `--commit <sha>` | explain | Explain specific commit |
| `--no-pager` | explain | Disable pager output |

---

## Glossary

| Term | Definition |
|------|------------|
| **Checkpoint** | A snapshot of code and transcript at a point in time. Created on commit (manual-commit) or after each response (auto-commit). |
| **Condensation** | The process of merging shadow branch data into the `entire/sessions` branch after a user commit. |
| **Session** | A complete interaction with an AI agent from start to finish, identified by a unique `YYYY-MM-DD-<UUID>` ID. |
| **Shadow branch** | Ephemeral branch (`entire/<commit-hash>`) used by manual-commit strategy to store checkpoints before condensation. |
| **Transcript** | JSONL file containing all prompts and responses in a session. |
| **entire/sessions** | Orphan branch storing all session metadata permanently. |

---

## Getting Help

### In-CLI Help

```bash
entire --help              # General help
entire <command> --help    # Command-specific help
```

### Resources

- **GitHub Issues:** Report bugs or request features at https://github.com/entireio/cli/issues
- **Documentation:** See the project [README](../README.md) and [CONTRIBUTING.md](../CONTRIBUTING.md)
- **Source Code:** https://github.com/entireio/cli

### Reporting Issues

For detailed bug reporting guidelines, see the [Reporting Bugs section in CONTRIBUTING.md](../CONTRIBUTING.md#reporting-bugs).

When reporting issues, please include:

1. Entire version (`entire version`)
2. Operating system and version
3. Steps to reproduce (exact commands)
4. Expected vs actual behavior
5. Debug logs if applicable (`ENTIRE_LOG_LEVEL=debug`)
