# Implement Command

Build the agent Go package using test-driven development. Uses the probe report findings and the E2E test suite as the spec.

## Prerequisites

- The probe command's findings (hook events, transcript format, config mechanism)
- The E2E test runner already added (from `e2e-tests` command)
- If neither exists, read the agent's docs and ask the user about hook events, transcript format, and config

## Procedure

### Step 1: Read Implementation Guide

Read these files thoroughly before writing any code:

1. `docs/architecture/agent-guide.md` — Step-by-step implementation guide with code examples
2. `docs/architecture/agent-integration-checklist.md` — Validation criteria
3. `cmd/entire/cli/agent/agent.go` — Agent interface definition
4. `cmd/entire/cli/agent/event.go` — Event types and `ReadAndParseHookInput` helper

### Step 2: Read Reference Implementation

Pick the closest existing agent as a template based on the probe findings:

| If the agent has... | Use as template |
|---------------------|----------------|
| JSON config hook mechanism (like Claude's `.claude/settings.json`) | `claudecode/` |
| JSON config hook mechanism (like Droid's `.factory/settings.json`) | `factoryaidroid/` |
| File-based detection (no native hooks) | `geminicli/` |
| Plugin/extension system | `opencode/` |

Read all files in the chosen reference: `*.go` (skip `*_test.go` on first pass).

### Step 3: Create Package Structure

Create the agent package directory:

```
cmd/entire/cli/agent/$AGENT_SLUG/
```

### Step 4: TDD Cycle — Types

**Red**: Write `types_test.go` with tests for hook input struct parsing:

```go
//go:build !e2e

package $AGENT_SLUG

import (
    "encoding/json"
    "testing"
)

func TestHookInput_Parsing(t *testing.T) {
    t.Parallel()
    // Test that hook JSON payloads deserialize correctly
}
```

**Green**: Write `types.go` with hook input structs:

```go
package $AGENT_SLUG

// HookInput represents the JSON payload from the agent's hooks.
type HookInput struct {
    SessionID      string `json:"session_id"`
    TranscriptPath string `json:"transcript_path"`
    // ... fields from probe report's captured payloads
}
```

**Refactor**: Ensure struct tags match the actual JSON field names from the probe captures.

Run: `mise run test` to verify.

### Step 5: TDD Cycle — Core Agent

**Red**: Write `${AGENT_SLUG}_test.go`:

```go
func TestAgent_Interface(t *testing.T) {
    t.Parallel()
    a := New()
    // Test Identity methods
    assert.Equal(t, agent.AgentName("$AGENT_SLUG"), a.Name())
    assert.NotEmpty(t, a.Description())
    assert.NotEmpty(t, a.ProtectedDirs())
}

func TestAgent_GetSessionDir(t *testing.T) {
    t.Parallel()
    // Test session directory resolution
}
```

**Green**: Write `${AGENT_SLUG}.go`:

```go
package $AGENT_SLUG

import "github.com/entireio/cli/cmd/entire/cli/agent"

func init() {
    agent.Register(New())
}

type Agent struct{}

func New() *Agent { return &Agent{} }

// Implement all 19 Agent interface methods...
```

Run: `mise run test`

### Step 6: TDD Cycle — Lifecycle (ParseHookEvent)

This is the **main contribution surface** — mapping native hooks to Entire events.

**Red**: Write `lifecycle_test.go`:

```go
func TestParseHookEvent_SessionStart(t *testing.T) {
    t.Parallel()
    // Use actual JSON from probe captures
    input := strings.NewReader(`{"session_id": "test-123", ...}`)
    event, err := New().ParseHookEvent("session-start", input)
    require.NoError(t, err)
    assert.Equal(t, agent.SessionStart, event.Type)
    assert.Equal(t, "test-123", event.SessionID)
}

// Repeat for each mapped event type...
```

**Green**: Write `lifecycle.go`:

```go
func (a *Agent) HookNames() []string {
    return []string{...} // From probe report
}

func (a *Agent) ParseHookEvent(hookName string, stdin io.Reader) (*agent.Event, error) {
    switch hookName {
    case "session-start":
        input, err := agent.ReadAndParseHookInput[HookInput](stdin)
        // Map to agent.Event{Type: agent.SessionStart, ...}
    // ... other cases
    }
}
```

Run: `mise run test`

### Step 7: TDD Cycle — Hooks (HookSupport)

**Red**: Write `hooks_test.go`:

```go
func TestInstallHooks(t *testing.T) {
    t.Parallel()
    // Create temp dir, install hooks, verify config file
}

func TestAreHooksInstalled(t *testing.T) {
    t.Parallel()
    // Install then check, uninstall then check
}
```

**Green**: Write `hooks.go`:

```go
func (a *Agent) InstallHooks(localDev bool, force bool) error { ... }
func (a *Agent) UninstallHooks() error { ... }
func (a *Agent) AreHooksInstalled() (bool, error) { ... }
```

Use the probe report to determine:
- Which config file to modify (e.g., `.agent/settings.json`)
- How hooks are registered (JSON objects, env vars, etc.)
- What command format to use (`entire hooks $AGENT_SLUG <verb>`)

Run: `mise run test`

### Step 8: TDD Cycle — Transcript

**Red**: Write `transcript_test.go`:

```go
func TestReadTranscript(t *testing.T) {
    t.Parallel()
    // Write sample transcript, read it back
}

func TestChunkTranscript(t *testing.T) {
    t.Parallel()
    // Test splitting large transcripts
}
```

**Green**: Write `transcript.go` — implement `ReadTranscript`, `ChunkTranscript`, `ReassembleTranscript`.

Use the probe report to understand the transcript format (JSONL, JSON, custom).

Run: `mise run test`

### Step 9: Optional Interfaces

Based on the probe report's feasibility table, implement optional interfaces if applicable:

- **TranscriptAnalyzer** — if the transcript format needs parsing for file extraction
- **HookHandler** — if hooks need pre/post processing beyond ParseHookEvent
- **TokenCalculator** — if the transcript contains token usage data
- **SubagentAwareExtractor** — if the agent supports subagents/tasks
- **FileWatcher** — if the agent doesn't have native hooks (like Gemini)

Each follows the same TDD cycle: test first, implement, refactor.

### Step 10: Register and Wire Up

1. **Register hook commands**: Add the agent to `cmd/entire/cli/commands/hooks.go` (or wherever hook subcommands are registered)
2. **Verify registration**: The `init()` function in `${AGENT_SLUG}.go` should call `agent.Register(New())`
3. **Run full test suite**: `mise run test:ci`

### Step 11: Final Validation

Run the complete validation:

```bash
mise run fmt      # Format
mise run lint     # Lint
mise run test:ci  # All tests (unit + integration)
```

Check against the integration checklist (`docs/architecture/agent-integration-checklist.md`):

- [ ] Full transcript stored at every checkpoint
- [ ] Native format preserved
- [ ] All mappable hook events implemented
- [ ] Session storage working
- [ ] Hook installation/uninstallation working
- [ ] Tests pass with `t.Parallel()`

## Key Patterns to Follow

- **Use `agent.ReadAndParseHookInput[T]`** for parsing hook stdin JSON
- **Use `paths.WorktreeRoot()`** not `os.Getwd()` for git-relative paths
- **Preserve unknown config keys** when modifying agent config files (don't clobber user settings)
- **Use `logging.Debug/Info/Warn/Error`** for internal logging, not `fmt.Print`
- **Keep interface implementations minimal** — only implement what's needed
- **Follow Go idioms** from `.golangci.yml` — check before writing code

## Output

Summarize what was implemented:
- Package directory and files created
- Interfaces implemented (core + optional)
- Hook names registered
- Test coverage (number of test functions, what they cover)
- Any gaps or TODOs remaining
- Commands to run full validation
