//go:build e2e

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/entireio/cli/e2e/testutil"
	"github.com/stretchr/testify/assert"
)

// TestAutoCommitStrategy: agent creates a file with auto-commit strategy.
// The commit and checkpoint should happen automatically without user intervention.
func TestAutoCommitStrategy(t *testing.T) {
	testutil.ForEachAgent(t, 3*time.Minute, func(t *testing.T, s *testutil.RepoState, ctx context.Context) {
		testutil.PatchSettings(t, s.Dir, map[string]any{"strategy": "auto-commit"})

		_, err := s.RunPrompt(t, ctx,
			"create a markdown file at docs/red.md with a paragraph about the colour red. Do not commit the file. Do not ask for confirmation, just make the change.")
		if err != nil {
			t.Fatalf("agent failed: %v", err)
		}

		testutil.AssertFileExists(t, s.Dir, "docs/red.md")

		// Auto-commit is async — give it extra time.
		testutil.WaitForCheckpoint(t, s, 30*time.Second)
		testutil.AssertNewCommits(t, s, 1)
		testutil.AssertCheckpointAdvanced(t, s)

		cpID := testutil.AssertHasCheckpointTrailer(t, s.Dir, "HEAD")
		testutil.AssertCheckpointExists(t, s.Dir, cpID)

		meta := testutil.ReadCheckpointMetadata(t, s.Dir, cpID)
		assert.Equal(t, "auto-commit", meta.Strategy,
			"checkpoint metadata should reflect auto-commit strategy")
	})
}
