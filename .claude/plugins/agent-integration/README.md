# Agent Integration Plugin

Local plugin providing individual commands for the agent integration workflow.

## Commands

| Command | Description |
|---------|-------------|
| `/agent-integration:research` | Assess a new agent's hook/lifecycle compatibility |
| `/agent-integration:write-tests` | Generate E2E test suite for the agent |
| `/agent-integration:implement` | Build the Go agent package via TDD |

## Loading

```bash
# Pass --plugin-dir when starting Claude Code
claude --plugin-dir .claude/plugins/agent-integration/

# Or add a shell alias
alias claude-dev='claude --plugin-dir .claude/plugins/agent-integration/'
```

## Related

- Orchestrator skill: `.claude/skills/agent-integration/SKILL.md` (`/agent-integration` — runs all 3 phases)
