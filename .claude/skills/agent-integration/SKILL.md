---
name: agent-integration
description: >
  Multi-phase toolkit for integrating a new AI coding agent with the Entire CLI.
  Three commands: "probe" (assess hook/lifecycle compatibility), "e2e-tests"
  (generate E2E test suite), and "implement" (build agent package via TDD).
  Use when the user says "probe agent", "check agent compatibility",
  "write e2e tests for <agent>", "implement <agent> integration",
  or any variation of evaluating/building a new agent integration.
---

# Agent Integration Skill

A three-command toolkit that takes a new AI coding agent from unknown to fully integrated with the Entire CLI. Each command builds on the previous command's output.

## Commands

### `/agent-integration probe`
**Assess compatibility.** Inspect the target agent's CLI, probe for hook/lifecycle support, create a test script, capture payloads, and produce a structured compatibility report.

Detailed instructions: [probe-prompt.md](./probe-prompt.md)

### `/agent-integration e2e-tests`
**Write the E2E test suite.** Using the probe report's findings, generate E2E test files that validate the agent works end-to-end with Entire's checkpoint/rewind system.

Detailed instructions: [e2e-test-prompt.md](./e2e-test-prompt.md)

### `/agent-integration implement`
**Build the agent package via TDD.** Using the probe report and E2E tests as the spec, implement the Go agent package with a red-green-refactor cycle.

Detailed instructions: [implement-prompt.md](./implement-prompt.md)

## Shared Parameters

These parameters are shared across all commands. Collect them on first invocation and reuse across subsequent commands.

| Parameter | Example | Description |
|-----------|---------|-------------|
| `AGENT_NAME` | "Windsurf" | Human-readable agent name |
| `AGENT_SLUG` | "windsurf" | Lowercase slug for file/directory paths |
| `AGENT_BIN` | "windsurf" | CLI binary name |
| `LIVE_COMMAND` | "windsurf --project ." | Full command to launch agent |
| `EVENTS_OR_UNKNOWN` | "unknown" | Known hook event names, or "unknown" |

## Command Routing

When the user invokes this skill:

1. **Parse the argument** after `/agent-integration` to determine which command to run
2. **If no argument**, ask which command they want: `probe`, `e2e-tests`, or `implement`
3. **If parameters aren't set yet**, collect them before proceeding
4. **Load the corresponding prompt file** and follow its instructions

### Typical workflow order

```
/agent-integration probe        # Phase 1: Assess compatibility
/agent-integration e2e-tests    # Phase 2: Write test suite
/agent-integration implement    # Phase 3: Build via TDD
```

Each phase can be run independently, but later phases work best with earlier phase outputs available.
