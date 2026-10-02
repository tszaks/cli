package codex

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParsePSOutput_FiltersCodexProcesses(t *testing.T) {
	t.Parallel()

	output := strings.Join([]string{
		"74495 ttys000 10:49:45 /opt/homebrew/bin/node",
		"74497 ttys000 10:49:45 /opt/homebrew/lib/node_modules/@openai/codex/node_modules/@openai/codex-darwin-arm64/vendor/aarch64-apple-darwin/codex/codex",
		"82158 ttys005 05:24 /opt/homebrew/lib/node_modules/@openai/codex/node_modules/@openai/codex-darwin-arm64/vendor/aarch64-apple-darwin/codex/codex",
	}, "\n")

	processes, err := parsePSOutput(output)
	if err != nil {
		t.Fatalf("parsePSOutput failed: %v", err)
	}
	if len(processes) != 2 {
		t.Fatalf("expected 2 codex processes, got %d", len(processes))
	}
	if processes[0].PID != 74497 || processes[1].PID != 82158 {
		t.Fatalf("unexpected parsed pids: %+v", processes)
	}
}

func TestExtractShellWorkdir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  string
		want string
	}{
		{
			name: "quoted path",
			cmd:  `cd "/Users/tyler/Projects/cli" && git status`,
			want: "/Users/tyler/Projects/cli",
		},
		{
			name: "unquoted path",
			cmd:  "cd /Users/tyler/Projects/archive && git status",
			want: "/Users/tyler/Projects/archive",
		},
		{
			name: "no leading cd",
			cmd:  "git status",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := extractShellWorkdir(tt.cmd); got != tt.want {
				t.Fatalf("extractShellWorkdir(%q) = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestParseElapsedTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw  string
		want int64
	}{
		{raw: "45", want: 45},
		{raw: "05:24", want: 324},
		{raw: "10:49:45", want: 38985},
		{raw: "2-03:04:05", want: 183845},
	}

	for _, tt := range tests {
		got, err := parseElapsedTime(tt.raw)
		if err != nil {
			t.Fatalf("parseElapsedTime(%q) failed: %v", tt.raw, err)
		}
		if got != tt.want {
			t.Fatalf("parseElapsedTime(%q) = %d, want %d", tt.raw, got, tt.want)
		}
	}
}

func TestCollectSessions_IncludeAllAndDetails(t *testing.T) {
	if _, err := exec.LookPath(sqlite3Command); err != nil {
		t.Skipf("sqlite3 not available: %v", err)
	}

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, codexStateDBFile)

	createTestCodexDB(t, dbPath, `
CREATE TABLE threads (
	id TEXT PRIMARY KEY,
	rollout_path TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	source TEXT NOT NULL DEFAULT '',
	model_provider TEXT NOT NULL DEFAULT '',
	cwd TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL DEFAULT '',
	sandbox_policy TEXT NOT NULL DEFAULT '',
	approval_mode TEXT NOT NULL DEFAULT '',
	tokens_used INTEGER NOT NULL DEFAULT 0,
	has_user_event INTEGER NOT NULL DEFAULT 0,
	archived INTEGER NOT NULL DEFAULT 0,
	archived_at INTEGER,
	git_sha TEXT,
	git_branch TEXT,
	git_origin_url TEXT,
	cli_version TEXT NOT NULL DEFAULT '',
	first_user_message TEXT NOT NULL DEFAULT '',
	agent_nickname TEXT,
	agent_role TEXT,
	memory_mode TEXT NOT NULL DEFAULT 'enabled'
);
CREATE TABLE logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ts INTEGER NOT NULL,
	ts_nanos INTEGER NOT NULL DEFAULT 0,
	level TEXT NOT NULL DEFAULT 'INFO',
	target TEXT NOT NULL DEFAULT '',
	message TEXT,
	module_path TEXT,
	file TEXT,
	line INTEGER,
	thread_id TEXT,
	process_uuid TEXT,
	estimated_bytes INTEGER NOT NULL DEFAULT 0
);
INSERT INTO threads (
	id, created_at, updated_at, cwd, title, first_user_message, git_branch, git_origin_url,
	source, model_provider, sandbox_policy, approval_mode, cli_version
) VALUES
	('thread-active', 1772821500, 1772821950, '/Users/tyler', 'Active session title', 'active prompt', 'main', 'git@example.com:cli.git', 'cli', 'openai', 'workspace-write', 'default', '0.1.0'),
	('thread-subagent', 1772821960, 1772821970, '/Users/tyler', 'Subagent session title', 'subagent prompt', 'main', 'git@example.com:cli.git', 'cli', 'openai', 'workspace-write', 'default', '0.1.0'),
	('thread-stale', 1772821000, 1772821100, '/Users/tyler', 'Stale session title', 'stale prompt', 'old', 'git@example.com:old.git', 'cli', 'openai', 'workspace-write', 'default', '0.1.0'),
	('thread-inactive', 1772821400, 1772821800, '/Users/tyler', 'Inactive session title', 'inactive prompt', 'feat/archive', 'git@example.com:archive.git', 'cli', 'openai', 'workspace-write', 'default', '0.1.0');
INSERT INTO logs (ts, level, target, message, thread_id, process_uuid) VALUES
	(1772821950, 'INFO', 'codex_core::stream_events_utils', 'ToolCall: exec_command {"cmd":"git status","workdir":"/Users/tyler/Projects/cli"}', 'thread-active', 'pid:4242:test-process'),
	(1772821970, 'INFO', 'codex_core::stream_events_utils', 'ToolCall: exec_command {"cmd":"git diff","workdir":"/Users/tyler/Projects/subagent"}', 'thread-subagent', 'pid:4242:test-process'),
	(1772821100, 'INFO', 'codex_core::stream_events_utils', 'ToolCall: exec_command {"cmd":"git status","workdir":"/Users/tyler/Projects/stale"}', 'thread-stale', 'pid:4242:old-process'),
	(1772821800, 'INFO', 'codex_core::stream_events_utils', 'ToolCall: exec_command {"cmd":"cd /Users/tyler/Projects/archive && git status"}', 'thread-inactive', 'pid:9999:old-process');
`)

	originalHome := codexHomeDirFunc
	originalHostname := hostnameFunc
	originalNow := nowFunc
	originalList := listLiveCodexProcessesVar
	defer func() {
		codexHomeDirFunc = originalHome
		hostnameFunc = originalHostname
		nowFunc = originalNow
		listLiveCodexProcessesVar = originalList
	}()

	codexHomeDirFunc = func() (string, error) { return tempDir, nil }
	hostnameFunc = func() (string, error) { return "test-host", nil }
	nowFunc = func() time.Time { return time.Unix(1772822000, 0).UTC() }
	listLiveCodexProcessesVar = func(context.Context) ([]liveCodexProcess, error) {
		return []liveCodexProcess{
			{PID: 4242, TTY: "ttys001", AgeSeconds: 125, Command: "/usr/local/bin/codex"},
		}, nil
	}

	snapshot, err := CollectSessions(context.Background(), SessionCollectOptions{
		IncludeAll:     true,
		IncludeDetails: true,
	})
	if err != nil {
		t.Fatalf("CollectSessions failed: %v", err)
	}

	if snapshot.Host != "test-host" {
		t.Fatalf("unexpected host: %q", snapshot.Host)
	}
	if len(snapshot.Sessions) != 4 {
		t.Fatalf("expected 4 sessions, got %d", len(snapshot.Sessions))
	}

	active := snapshot.Sessions[0]
	if active.Status != activeSessionStatus {
		t.Fatalf("expected active session first, got %q", active.Status)
	}
	if active.ThreadID != "thread-active" {
		t.Fatalf("unexpected active thread id: %q", active.ThreadID)
	}
	if active.EffectiveWorkdir != "/Users/tyler/Projects/cli" {
		t.Fatalf("unexpected active workdir: %q", active.EffectiveWorkdir)
	}
	if active.RecentAction == "" {
		t.Fatal("expected recent action for active session")
	}

	inactive := findSessionByThreadID(t, snapshot.Sessions, "thread-inactive")
	if inactive.Status != inactiveSessionStatus {
		t.Fatalf("expected inactive session second, got %q", inactive.Status)
	}
	if inactive.EffectiveWorkdir != "/Users/tyler/Projects/archive" {
		t.Fatalf("unexpected inactive workdir: %q", inactive.EffectiveWorkdir)
	}
}

func TestCollectSessions_MissingStateDatabaseReturnsEmptySnapshot(t *testing.T) {
	originalHome := codexHomeDirFunc
	originalHostname := hostnameFunc
	originalNow := nowFunc
	originalList := listLiveCodexProcessesVar
	defer func() {
		codexHomeDirFunc = originalHome
		hostnameFunc = originalHostname
		nowFunc = originalNow
		listLiveCodexProcessesVar = originalList
	}()

	tempDir := t.TempDir()
	codexHomeDirFunc = func() (string, error) { return tempDir, nil }
	hostnameFunc = func() (string, error) { return "test-host", nil }
	nowFunc = func() time.Time { return time.Unix(1772822000, 0).UTC() }
	listLiveCodexProcessesVar = func(context.Context) ([]liveCodexProcess, error) {
		return []liveCodexProcess{
			{PID: 5150, TTY: "ttys003", AgeSeconds: 10, Command: "/usr/local/bin/codex"},
		}, nil
	}

	snapshot, err := CollectSessions(context.Background(), SessionCollectOptions{})
	if err != nil {
		t.Fatalf("CollectSessions failed: %v", err)
	}
	if len(snapshot.Sessions) != 1 {
		t.Fatalf("expected 1 starting-up session, got %d", len(snapshot.Sessions))
	}
	if snapshot.Sessions[0].Title != startingSessionTitle {
		t.Fatalf("unexpected starting-up title: %q", snapshot.Sessions[0].Title)
	}
}

func createTestCodexDB(t *testing.T, dbPath, sql string) {
	t.Helper()

	cmd := exec.Command(sqlite3Command, dbPath, sql)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to create sqlite test db: %v\n%s", err, output)
	}
}

func findSessionByThreadID(t *testing.T, sessions []SessionSummary, threadID string) SessionSummary {
	t.Helper()

	for _, session := range sessions {
		if session.ThreadID == threadID {
			return session
		}
	}
	t.Fatalf("session %q not found", threadID)
	return SessionSummary{}
}
