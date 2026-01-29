//go:build integration

package integration

import (
	"strings"
	"testing"
)

func TestExplain_NoCurrentSession(t *testing.T) {
	t.Parallel()
	RunForAllStrategies(t, func(t *testing.T, env *TestEnv, strategyName string) {
		// Without any flags, explain shows the branch view (not an error)
		output, err := env.RunCLIWithError("explain")

		if err != nil {
			t.Errorf("expected success for branch view, got error: %v, output: %s", err, output)
			return
		}

		// Should show branch information and checkpoint count
		if !strings.Contains(output, "Branch:") {
			t.Errorf("expected 'Branch:' header in output, got: %s", output)
		}
		if !strings.Contains(output, "Checkpoints:") {
			t.Errorf("expected 'Checkpoints:' in output, got: %s", output)
		}
	})
}

func TestExplain_SessionNotFound(t *testing.T) {
	t.Parallel()
	RunForAllStrategies(t, func(t *testing.T, env *TestEnv, strategyName string) {
		// Try to explain a non-existent session
		output, err := env.RunCLIWithError("explain", "--session", "nonexistent-session-id")

		if err == nil {
			t.Errorf("expected error for nonexistent session, got output: %s", output)
			return
		}

		if !strings.Contains(output, "session not found") {
			t.Errorf("expected 'session not found' error, got: %s", output)
		}
	})
}

func TestExplain_MutualExclusivity(t *testing.T) {
	t.Parallel()
	RunForAllStrategies(t, func(t *testing.T, env *TestEnv, strategyName string) {
		// Try to provide both --session and --commit flags
		output, err := env.RunCLIWithError("explain", "--session", "test-session", "--commit", "abc123")

		if err == nil {
			t.Errorf("expected error when both flags provided, got output: %s", output)
			return
		}

		if !strings.Contains(strings.ToLower(output), "cannot specify multiple") {
			t.Errorf("expected 'cannot specify multiple' error, got: %s", output)
		}
	})
}

func TestExplain_CheckpointNotFound(t *testing.T) {
	t.Parallel()
	RunForAllStrategies(t, func(t *testing.T, env *TestEnv, strategyName string) {
		// Try to explain a non-existent checkpoint
		output, err := env.RunCLIWithError("explain", "--checkpoint", "nonexistent123")

		if err == nil {
			t.Errorf("expected error for nonexistent checkpoint, got output: %s", output)
			return
		}

		if !strings.Contains(output, "checkpoint not found") {
			t.Errorf("expected 'checkpoint not found' error, got: %s", output)
		}
	})
}

func TestExplain_CheckpointMutualExclusivity(t *testing.T) {
	t.Parallel()
	RunForAllStrategies(t, func(t *testing.T, env *TestEnv, strategyName string) {
		// Try to provide --checkpoint with --session
		output, err := env.RunCLIWithError("explain", "--session", "test-session", "--checkpoint", "abc123")

		if err == nil {
			t.Errorf("expected error when both flags provided, got output: %s", output)
			return
		}

		if !strings.Contains(strings.ToLower(output), "cannot specify multiple") {
			t.Errorf("expected 'cannot specify multiple' error, got: %s", output)
		}
	})
}
