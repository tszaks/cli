# Agent Integration Skill

A multi-command toolkit for taking a new AI coding agent from unknown to fully integrated with the Entire CLI.

## Files

- `SKILL.md` — Skill definition, command routing, shared parameters
- `probe-prompt.md` — Probe command: assess hook/lifecycle compatibility
- `e2e-test-prompt.md` — E2E Tests command: generate test suite
- `implement-prompt.md` — Implement command: build agent package via TDD
- `README.md` — This file

## Commands

| Command | Purpose | Output |
|---------|---------|--------|
| `/agent-integration probe` | Assess compatibility | Compatibility report + test script |
| `/agent-integration e2e-tests` | Write E2E test suite | AgentRunner + test scenarios |
| `/agent-integration implement` | Build agent via TDD | Go package under `cmd/entire/cli/agent/` |

## Typical Workflow

```
/agent-integration probe         # 1. Can this agent integrate?
/agent-integration e2e-tests     # 2. What should the tests look like?
/agent-integration implement     # 3. Build it with TDD
```

Each command can be run independently, but later commands benefit from earlier outputs.

## Parameters

Collected once and reused across commands:

| Parameter | Example | Description |
|-----------|---------|-------------|
| `AGENT_NAME` | "Windsurf" | Human-readable name |
| `AGENT_SLUG` | "windsurf" | Lowercase slug |
| `AGENT_BIN` | "windsurf" | CLI binary name |
| `LIVE_COMMAND` | "windsurf --project ." | Launch command |
| `EVENTS_OR_UNKNOWN` | "unknown" | Known hook events or "unknown" |

## Architecture References

- Agent interface: `cmd/entire/cli/agent/agent.go`
- Event types: `cmd/entire/cli/agent/event.go`
- Implementation guide: `docs/architecture/agent-guide.md`
- Integration checklist: `docs/architecture/agent-integration-checklist.md`
- E2E test infrastructure: `cmd/entire/cli/e2e_test/`
- Existing agents: Discover via `Glob("cmd/entire/cli/agent/*/")`
