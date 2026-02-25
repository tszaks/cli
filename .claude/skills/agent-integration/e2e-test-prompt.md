# E2E Tests Command

Generate the E2E test suite for a new agent integration. Uses the probe report's findings and the existing E2E test infrastructure.

## Prerequisites

- The probe command should have been run first (or equivalent knowledge of the agent's hook model)
- If no probe report exists, ask the user about the agent's hook events, transcript format, and config mechanism

## Procedure

### Step 1: Read E2E Test Infrastructure

Read these files to understand the existing test patterns:

1. `cmd/entire/cli/e2e_test/setup_test.go` — `TestMain` builds CLI binary, checks agent availability, manages PATH
2. `cmd/entire/cli/e2e_test/testenv.go` — `TestEnv` and `NewFeatureBranchEnv` helpers (repo setup, git operations, rewind points, checkpoint validation)
3. `cmd/entire/cli/e2e_test/agent_runner.go` — `AgentRunner` interface and existing runners (`ClaudeCodeRunner`, `GeminiCLIRunner`, `FactoryAIDroidRunner`, `OpenCodeRunner`)
4. `cmd/entire/cli/e2e_test/prompts.go` — `PromptTemplate` definitions for deterministic test prompts
5. `cmd/entire/cli/e2e_test/assertions.go` — Shared assertion helpers (`AssertAgentSuccess`, `AssertHelloWorldProgram`, etc.)

### Step 2: Read Existing E2E Scenario Tests

Read these for the test patterns and conventions:

1. `cmd/entire/cli/e2e_test/scenario_basic_workflow_test.go` — Basic: agent creates file, user commits, checkpoint verified
2. `cmd/entire/cli/e2e_test/scenario_checkpoint_test.go` — Checkpoint metadata validation
3. `cmd/entire/cli/e2e_test/scenario_rewind_test.go` — Rewind to previous checkpoint
4. `cmd/entire/cli/e2e_test/scenario_subagent_test.go` — Subagent/task checkpoint creation
5. `cmd/entire/cli/e2e_test/scenario_agent_commit_test.go` — Agent-initiated commits
6. `cmd/entire/cli/e2e_test/scenario_checkpoint_workflows_test.go` — Multi-step checkpoint workflows

### Step 3: Read Checkpoint Scenarios Doc

Read `docs/architecture/checkpoint-scenarios.md` for the state machine and scenarios the tests should cover.

### Step 4: Create AgentRunner

Add a new `AgentRunner` implementation in `cmd/entire/cli/e2e_test/agent_runner.go`:

**Pattern to follow** (based on existing runners):

```go
// ${AGENT_NAME}Runner implements AgentRunner for ${AGENT_NAME}.
type ${AGENT_NAME}Runner struct {
    Model   string
    Timeout time.Duration
}

func New${AGENT_NAME}Runner(config AgentRunnerConfig) *${AGENT_NAME}Runner { ... }
func (r *${AGENT_NAME}Runner) Name() string { return AgentName${AGENT_NAME} }
func (r *${AGENT_NAME}Runner) IsAvailable() (bool, error) { ... }
func (r *${AGENT_NAME}Runner) RunPrompt(ctx, workDir, prompt) (*AgentResult, error) { ... }
func (r *${AGENT_NAME}Runner) RunPromptWithTools(ctx, workDir, prompt, tools) (*AgentResult, error) { ... }
```

Key implementation details:
- Add `AgentName${AGENT_NAME}` constant (e.g., `const AgentNameWindsurf = "windsurf"`)
- Register in `NewAgentRunner` switch statement
- `IsAvailable()` checks binary exists + any auth requirements
- `RunPrompt()` constructs the CLI command using the agent's non-interactive/headless mode
- Use the probe report to determine:
  - CLI flags for non-interactive execution
  - How to pass prompts (arg vs stdin vs flag)
  - How to specify allowed tools (if supported)
  - Any agent-specific env vars or config needed

### Step 5: Update TestEnv (if needed)

Check if `NewFeatureBranchEnv` needs agent-specific setup (like the OpenCode and Droid blocks in `testenv.go`):

- Agent-specific config files that need to be created before `entire enable`
- Permissions or auth config
- `ENTIRE_TEST_*` env vars for hook testing

### Step 6: Write E2E Test Scenarios

Existing tests are agent-agnostic (they use the `AgentRunner` interface), so they should already work with the new agent. **Only create new test files if the agent has unique behaviors** that existing scenarios don't cover.

Check if all existing scenarios work by reviewing:
- Does the agent support non-interactive prompt mode? (required for `RunPrompt`)
- Does the agent create files when prompted? (required for basic workflow)
- Does the agent support git operations? (required for commit scenarios)
- Does the agent support subagents/tasks? (required for subagent scenarios — can be skipped if not supported)

If the agent has unique behaviors, create new scenario files following the naming convention:
```
cmd/entire/cli/e2e_test/scenario_${AGENT_SLUG}_specific_test.go
```

### Step 7: Verify

After writing the code:

1. **Lint check**: `mise run lint` — ensure no lint errors
2. **Compile check**: `go build ./cmd/entire/cli/e2e_test/...` — but do NOT actually use `-tags=e2e` for compile-check, just verify the code structure
3. **List what to run**: Print the exact E2E commands but do NOT run them (they cost money):
   ```bash
   E2E_AGENT=$AGENT_SLUG go test -tags=e2e -run TestE2E_BasicWorkflow ./cmd/entire/cli/e2e_test/...
   ```

## Key Conventions

- **Build tag**: All E2E test files must have `//go:build e2e` as the first line
- **Package**: `package e2e`
- **Parallel**: Always `t.Parallel()` in top-level test functions
- **Strategy**: Use `NewFeatureBranchEnv(t, "manual-commit")` for most tests
- **Prompts**: Use existing `PromptTemplate` variables from `prompts.go`
- **Assertions**: Use existing assertion helpers from `assertions.go`
- **No hardcoded paths**: Use `TestEnv` helpers for all file/git operations
- **Do NOT run E2E tests**: They make real API calls. Only write the code and print commands.

## Output

Summarize what was created/modified:
- Files added or modified
- New agent runner details (how it invokes the agent)
- Any agent-specific test scenarios added
- Commands to run the tests (for user to execute manually)
