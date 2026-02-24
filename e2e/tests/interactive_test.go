//go:build e2e

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/entireio/cli/e2e/testutil"
)

func TestInteractiveMultiStep(t *testing.T) {
	testutil.ForEachAgent(t, 3*time.Minute, func(t *testing.T, s *testutil.RepoState, ctx context.Context) {
		prompt := s.Agent.PromptPattern()

		session, err := s.Agent.StartSession(ctx, s.Dir)
		if err != nil {
			t.Fatalf("failed to start interactive session: %v", err)
		}
		if session == nil {
			t.Skipf("agent %s does not support interactive mode", s.Agent.Name())
		}
		defer func() { _ = session.Close() }()

		if _, err = session.WaitFor(prompt, 30*time.Second); err != nil {
			t.Fatalf("waiting for initial prompt: %v", err)
		}

		s.Send(t, session, "create a markdown file at docs/red.md with a paragraph about the colour red. Do not ask for confirmation, just make the change.")
		if _, err = session.WaitFor(prompt, 60*time.Second); err != nil {
			t.Fatalf("waiting for prompt after file creation: %v", err)
		}
		testutil.AssertFileExists(t, s.Dir, "docs/*.md")

		s.Send(t, session, "now commit it")
		if _, err = session.WaitFor(prompt, 60*time.Second); err != nil {
			t.Fatalf("waiting for prompt after commit: %v", err)
		}
		testutil.AssertNewCommits(t, s, 1)

		testutil.WaitForCheckpoint(t, s, 15*time.Second)
		testutil.AssertCommitLinkedToCheckpoint(t, s.Dir, "HEAD")
	})
}
