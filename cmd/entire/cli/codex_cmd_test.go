package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	codexagent "github.com/entireio/cli/cmd/entire/cli/agent/codex"
)

func TestCodexSessionsCmdJSON(t *testing.T) {
	originalCollect := collectCodexSessions
	defer func() {
		collectCodexSessions = originalCollect
	}()

	collectCodexSessions = func(context.Context, codexagent.SessionCollectOptions) (*codexagent.SessionSnapshot, error) {
		return &codexagent.SessionSnapshot{
			GeneratedAt: time.Unix(1772822000, 0).UTC(),
			Host:        "test-host",
			Sessions: []codexagent.SessionSummary{
				{
					PID:              4242,
					TTY:              "ttys001",
					AgeSeconds:       125,
					ThreadID:         "thread-active",
					Title:            "Active session",
					EffectiveWorkdir: "/Users/tyler/Projects/cli",
					LastActiveAt:     time.Unix(1772821950, 0).UTC(),
					Status:           "active",
				},
			},
		}, nil
	}

	cmd := newCodexSessionsCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("output was not valid json: %v\n%s", err, stdout.String())
	}
	if payload["host"] != "test-host" {
		t.Fatalf("unexpected host in json: %#v", payload["host"])
	}
}

func TestCodexSessionsCmdRejectsWatchWithJSON(t *testing.T) {
	cmd := newCodexSessionsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--watch", "--json"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --watch --json")
	}
	if !strings.Contains(err.Error(), "--watch cannot be used with --json") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCodexSessionsCmdRejectsPositionalArgs(t *testing.T) {
	cmd := newCodexSessionsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"extra"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for positional args")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRootCodexSessionsJSONSkipsVersionCheckOutput(t *testing.T) {
	originalCollect := collectCodexSessions
	originalCheck := checkVersionAndNotify
	defer func() {
		collectCodexSessions = originalCollect
		checkVersionAndNotify = originalCheck
	}()

	collectCodexSessions = func(context.Context, codexagent.SessionCollectOptions) (*codexagent.SessionSnapshot, error) {
		return &codexagent.SessionSnapshot{
			GeneratedAt: time.Unix(1772822000, 0).UTC(),
			Host:        "test-host",
			Sessions: []codexagent.SessionSummary{
				{
					ThreadID:         "thread-active",
					Title:            "Active session",
					EffectiveWorkdir: "/Users/tyler/Projects/cli",
					LastActiveAt:     time.Unix(1772821950, 0).UTC(),
					Status:           "active",
				},
			},
		}, nil
	}
	checkVersionAndNotify = func(_ context.Context, w io.Writer, _ string) {
		_, _ = io.WriteString(w, "\nVERSION-CHECK-SHOULD-NOT-PRINT\n")
	}

	root := NewRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"codex", "sessions", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("root Execute failed: %v", err)
	}
	if strings.Contains(stdout.String(), "VERSION-CHECK-SHOULD-NOT-PRINT") {
		t.Fatalf("version check polluted json output: %s", stdout.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("root output was not valid json: %v\n%s", err, stdout.String())
	}
}

func TestCodexResumeCmdUsesUniquePrefix(t *testing.T) {
	originalCollect := collectCodexSessions
	originalRun := runCodexResume
	defer func() {
		collectCodexSessions = originalCollect
		runCodexResume = originalRun
	}()

	collectCodexSessions = func(context.Context, codexagent.SessionCollectOptions) (*codexagent.SessionSnapshot, error) {
		return &codexagent.SessionSnapshot{
			Sessions: []codexagent.SessionSummary{
				{ThreadID: "019cc45a-f772-7fd0-b7c0-f762df930f57", Status: "active"},
				{ThreadID: "abcd1234-1111-2222-3333-444444444444", Status: "inactive"},
			},
		}, nil
	}

	var resumed string
	runCodexResume = func(_ context.Context, threadID string) error {
		resumed = threadID
		return nil
	}

	cmd := newCodexResumeCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"019cc45a"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if resumed != "019cc45a-f772-7fd0-b7c0-f762df930f57" {
		t.Fatalf("unexpected resumed thread: %q", resumed)
	}
}

func TestCodexResumeCmdRejectsAmbiguousPrefix(t *testing.T) {
	originalCollect := collectCodexSessions
	originalRun := runCodexResume
	defer func() {
		collectCodexSessions = originalCollect
		runCodexResume = originalRun
	}()

	collectCodexSessions = func(context.Context, codexagent.SessionCollectOptions) (*codexagent.SessionSnapshot, error) {
		return &codexagent.SessionSnapshot{
			Sessions: []codexagent.SessionSummary{
				{ThreadID: "019cc45a-f772-7fd0-b7c0-f762df930f57", Status: "active"},
				{ThreadID: "019cc45a-aaaa-bbbb-cccc-555555555555", Status: "inactive"},
			},
		}, nil
	}

	cmd := newCodexResumeCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"019cc45a"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected ambiguous prefix error")
	}
	if !strings.Contains(err.Error(), "multiple Codex sessions match") {
		t.Fatalf("unexpected error: %v", err)
	}
}
