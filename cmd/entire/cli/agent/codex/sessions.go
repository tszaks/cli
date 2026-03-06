package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	psCommand                 = "ps"
	sqlite3Command            = "sqlite3"
	codexStateDBFile          = "state_5.sqlite"
	sessionLogPreviewLimit    = 5
	codexToolLogTarget        = "codex_core::stream_events_utils"
	activeSessionStatus       = "active"
	inactiveSessionStatus     = "inactive"
	startingSessionTitle      = "(starting up)"
	defaultRecentActionSuffix = "ToolCall: "
)

var (
	changeDirCommandRegex     = regexp.MustCompile(`^\s*cd\s+(?:(['"])(.*?)\1|([^&|;]+))\s*&&`)
	codexHomeDirFunc          = codexHomeDir
	hostnameFunc              = os.Hostname
	nowFunc                   = time.Now
	listLiveCodexProcessesVar = listLiveCodexProcesses
	runSQLiteJSONVar          = runSQLiteJSON
)

// SessionCollectOptions controls which Codex sessions are returned.
type SessionCollectOptions struct {
	IncludeAll     bool
	IncludeDetails bool
}

// SessionSnapshot is the public output shape for live Codex session discovery.
type SessionSnapshot struct {
	GeneratedAt time.Time        `json:"generated_at"`
	Host        string           `json:"host"`
	Sessions    []SessionSummary `json:"sessions"`
}

// SessionSummary is the normalized view of one Codex session.
type SessionSummary struct {
	PID              int       `json:"pid,omitempty"`
	TTY              string    `json:"tty,omitempty"`
	AgeSeconds       int64     `json:"age_seconds,omitempty"`
	ThreadID         string    `json:"thread_id,omitempty"`
	Title            string    `json:"title,omitempty"`
	FirstUserMessage string    `json:"first_user_message,omitempty"`
	SessionCWD       string    `json:"session_cwd,omitempty"`
	EffectiveWorkdir string    `json:"effective_workdir,omitempty"`
	LastActiveAt     time.Time `json:"last_active_at,omitempty"`
	GitBranch        string    `json:"git_branch,omitempty"`
	GitOriginURL     string    `json:"git_origin_url,omitempty"`
	Status           string    `json:"status"`
	RecentAction     string    `json:"recent_action,omitempty"`
}

type liveCodexProcess struct {
	PID        int
	TTY        string
	AgeSeconds int64
	Command    string
}

type processThreadRow struct {
	ProcessUUID string `json:"process_uuid"`
	ThreadID    string `json:"thread_id"`
}

type threadRow struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	FirstUserMessage string `json:"first_user_message"`
	CWD              string `json:"cwd"`
	UpdatedAt        int64  `json:"updated_at"`
	GitBranch        string `json:"git_branch"`
	GitOriginURL     string `json:"git_origin_url"`
}

type threadLogRow struct {
	ThreadID string `json:"thread_id"`
	Target   string `json:"target"`
	Message  string `json:"message"`
	TS       int64  `json:"ts"`
}

type toolCall struct {
	Name string
	Args map[string]any
}

// CollectSessions returns the current Codex sessions using local process state plus
// Codex's SQLite registry under ~/.codex/state_5.sqlite.
func CollectSessions(ctx context.Context, opts SessionCollectOptions) (*SessionSnapshot, error) {
	codexHome, err := codexHomeDirFunc()
	if err != nil {
		return nil, err
	}

	dbPath := filepath.Join(codexHome, codexStateDBFile)

	generatedAt := nowFunc().UTC()
	host, _ := hostnameFunc()

	liveProcesses, err := listLiveCodexProcessesVar(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			sessions := make([]SessionSummary, 0, len(liveProcesses))
			for _, proc := range liveProcesses {
				sessions = append(sessions, SessionSummary{
					PID:          proc.PID,
					TTY:          proc.TTY,
					AgeSeconds:   proc.AgeSeconds,
					Title:        startingSessionTitle,
					LastActiveAt: generatedAt,
					Status:       activeSessionStatus,
				})
			}
			sortSessions(sessions)
			return &SessionSnapshot{
				GeneratedAt: generatedAt,
				Host:        host,
				Sessions:    sessions,
			}, nil
		}
		return nil, fmt.Errorf("failed to access codex state database: %w", err)
	}

	activeByThread := make(map[string]SessionSummary)
	var startingUp []SessionSummary
	threadIDs := make([]string, 0, len(liveProcesses))

	for _, proc := range liveProcesses {
		processStartCutoff := generatedAt.Unix() - proc.AgeSeconds - 30
		mapping, err := queryPrimaryThreadForPID(ctx, dbPath, proc.PID, processStartCutoff)
		if err != nil {
			return nil, err
		}
		if mapping.ThreadID == "" {
			startingUp = append(startingUp, SessionSummary{
				PID:          proc.PID,
				TTY:          proc.TTY,
				AgeSeconds:   proc.AgeSeconds,
				Title:        startingSessionTitle,
				LastActiveAt: generatedAt,
				Status:       activeSessionStatus,
			})
			continue
		}

		if _, exists := activeByThread[mapping.ThreadID]; exists {
			continue
		}

		activeByThread[mapping.ThreadID] = SessionSummary{
			PID:        proc.PID,
			TTY:        proc.TTY,
			AgeSeconds: proc.AgeSeconds,
			ThreadID:   mapping.ThreadID,
			Status:     activeSessionStatus,
		}
		threadIDs = append(threadIDs, mapping.ThreadID)
	}

	threadsByID := make(map[string]threadRow)
	var allThreads []threadRow
	switch {
	case opts.IncludeAll:
		allThreads, err = queryThreads(ctx, dbPath, nil)
	case len(threadIDs) > 0:
		allThreads, err = queryThreads(ctx, dbPath, threadIDs)
	default:
		allThreads = nil
	}
	if err != nil {
		return nil, err
	}
	for _, row := range allThreads {
		threadsByID[row.ID] = row
	}

	logsByThread, err := queryRecentToolLogs(ctx, dbPath, threadIDsForLogs(opts.IncludeAll, allThreads, threadIDs))
	if err != nil {
		return nil, err
	}

	sessions := make([]SessionSummary, 0, len(activeByThread)+len(startingUp))
	for threadID, session := range activeByThread {
		row, ok := threadsByID[threadID]
		if ok {
			enrichSession(&session, row, logsByThread[threadID], opts.IncludeDetails)
		} else {
			session.LastActiveAt = generatedAt
		}
		sessions = append(sessions, session)
	}

	sessions = append(sessions, startingUp...)

	if opts.IncludeAll {
		for _, row := range allThreads {
			if _, active := activeByThread[row.ID]; active {
				continue
			}

			session := SessionSummary{
				ThreadID: row.ID,
				Status:   inactiveSessionStatus,
			}
			enrichSession(&session, row, logsByThread[row.ID], opts.IncludeDetails)
			sessions = append(sessions, session)
		}
	}

	sortSessions(sessions)

	return &SessionSnapshot{
		GeneratedAt: generatedAt,
		Host:        host,
		Sessions:    sessions,
	}, nil
}

func enrichSession(session *SessionSummary, row threadRow, logs []threadLogRow, includeDetails bool) {
	session.Title = firstNonEmpty(row.Title, row.FirstUserMessage, startingSessionTitle)
	session.FirstUserMessage = row.FirstUserMessage
	session.SessionCWD = row.CWD
	session.EffectiveWorkdir = inferEffectiveWorkdir(logs, row.CWD)
	session.LastActiveAt = unixSecondsToTime(row.UpdatedAt)
	session.GitBranch = row.GitBranch
	session.GitOriginURL = row.GitOriginURL
	if includeDetails {
		session.RecentAction = summarizeRecentAction(logs)
	}
}

func sortSessions(sessions []SessionSummary) {
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].Status != sessions[j].Status {
			return sessions[i].Status == activeSessionStatus
		}
		if !sessions[i].LastActiveAt.Equal(sessions[j].LastActiveAt) {
			return sessions[i].LastActiveAt.After(sessions[j].LastActiveAt)
		}
		if sessions[i].ThreadID != sessions[j].ThreadID {
			return sessions[i].ThreadID < sessions[j].ThreadID
		}
		return sessions[i].PID < sessions[j].PID
	})
}

func threadIDsForLogs(includeAll bool, allThreads []threadRow, activeThreadIDs []string) []string {
	if includeAll {
		ids := make([]string, 0, len(allThreads))
		for _, row := range allThreads {
			ids = append(ids, row.ID)
		}
		return ids
	}
	return activeThreadIDs
}

func queryPrimaryThreadForPID(ctx context.Context, dbPath string, pid int, minTS int64) (processThreadRow, error) {
	query := fmt.Sprintf(
		`SELECT logs.process_uuid, threads.id AS thread_id
FROM threads
JOIN (
	SELECT process_uuid, thread_id
	FROM logs
	WHERE process_uuid LIKE %s
		AND thread_id IS NOT NULL
		AND thread_id != ''
		AND ts >= %d
	GROUP BY process_uuid, thread_id
) AS logs ON logs.thread_id = threads.id
WHERE threads.archived = 0
ORDER BY threads.created_at ASC, threads.updated_at DESC, threads.id ASC
LIMIT 1;`,
		sqliteQuote(fmt.Sprintf("pid:%d:%%", pid)),
		minTS,
	)

	rows, err := querySQLiteRows[processThreadRow](ctx, dbPath, query)
	if err != nil {
		return processThreadRow{}, err
	}
	if len(rows) == 0 {
		return processThreadRow{}, nil
	}
	return rows[0], nil
}

func queryThreads(ctx context.Context, dbPath string, threadIDs []string) ([]threadRow, error) {
	query := "SELECT id, title, first_user_message, cwd, updated_at, git_branch, git_origin_url FROM threads WHERE archived = 0"
	if len(threadIDs) > 0 {
		query += " AND id IN (" + sqliteStringList(threadIDs) + ")"
	}
	query += " ORDER BY updated_at DESC, id DESC;"
	return querySQLiteRows[threadRow](ctx, dbPath, query)
}

func queryRecentToolLogs(ctx context.Context, dbPath string, threadIDs []string) (map[string][]threadLogRow, error) {
	if len(threadIDs) == 0 {
		return map[string][]threadLogRow{}, nil
	}

	var where strings.Builder
	where.WriteString("target = ")
	where.WriteString(sqliteQuote(codexToolLogTarget))
	where.WriteString(" AND thread_id IS NOT NULL AND thread_id != ''")
	if len(threadIDs) > 0 {
		where.WriteString(" AND thread_id IN (")
		where.WriteString(sqliteStringList(threadIDs))
		where.WriteString(")")
	}

	query := fmt.Sprintf(`
WITH ranked AS (
	SELECT thread_id, target, message, ts,
		ROW_NUMBER() OVER (PARTITION BY thread_id ORDER BY id DESC) AS rn
	FROM logs
	WHERE %s
)
SELECT thread_id, target, message, ts
FROM ranked
WHERE rn <= %d
ORDER BY thread_id ASC, rn ASC;`,
		where.String(),
		sessionLogPreviewLimit,
	)

	rows, err := querySQLiteRows[threadLogRow](ctx, dbPath, query)
	if err != nil {
		return nil, err
	}

	grouped := make(map[string][]threadLogRow)
	for _, row := range rows {
		grouped[row.ThreadID] = append(grouped[row.ThreadID], row)
	}
	return grouped, nil
}

func querySQLiteRows[T any](ctx context.Context, dbPath, query string) ([]T, error) {
	data, err := runSQLiteJSONVar(ctx, dbPath, query)
	if err != nil {
		return nil, err
	}

	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, nil
	}

	var rows []T
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("failed to parse sqlite json output: %w", err)
	}
	return rows, nil
}

func runSQLiteJSON(ctx context.Context, dbPath, query string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, sqlite3Command, "-json", dbPath, query)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("sqlite query failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func listLiveCodexProcesses(ctx context.Context) ([]liveCodexProcess, error) {
	cmd := exec.CommandContext(ctx, psCommand, "-axo", "pid=,tty=,etime=,comm=")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list processes: %w", err)
	}
	return parsePSOutput(string(output))
}

func parsePSOutput(output string) ([]liveCodexProcess, error) {
	lines := strings.Split(output, "\n")
	processes := make([]liveCodexProcess, 0, len(lines))

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		command := strings.Join(fields[3:], " ")
		if !looksLikeCodexCommand(command) {
			continue
		}

		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, fmt.Errorf("failed to parse codex pid %q: %w", fields[0], err)
		}

		ageSeconds, err := parseElapsedTime(fields[2])
		if err != nil {
			return nil, fmt.Errorf("failed to parse codex process age %q: %w", fields[2], err)
		}

		processes = append(processes, liveCodexProcess{
			PID:        pid,
			TTY:        fields[1],
			AgeSeconds: ageSeconds,
			Command:    command,
		})
	}

	return processes, nil
}

func looksLikeCodexCommand(command string) bool {
	command = strings.TrimSpace(command)
	if command == "" {
		return false
	}
	base := filepath.Base(command)
	return base == "codex" || strings.Contains(command, "/codex/")
}

func parseElapsedTime(raw string) (int64, error) {
	dayParts := strings.SplitN(raw, "-", 2)
	days := 0
	timePart := raw
	if len(dayParts) == 2 {
		parsedDays, err := strconv.Atoi(dayParts[0])
		if err != nil {
			return 0, err
		}
		days = parsedDays
		timePart = dayParts[1]
	}

	parts := strings.Split(timePart, ":")
	switch len(parts) {
	case 3:
		hours, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, err
		}
		minutes, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, err
		}
		seconds, err := strconv.Atoi(parts[2])
		if err != nil {
			return 0, err
		}
		total := int64(days*24*3600 + hours*3600 + minutes*60 + seconds)
		return total, nil
	case 2:
		minutes, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, err
		}
		seconds, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, err
		}
		total := int64(days*24*3600 + minutes*60 + seconds)
		return total, nil
	case 1:
		seconds, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, err
		}
		total := int64(days*24*3600 + seconds)
		return total, nil
	default:
		return 0, fmt.Errorf("unsupported elapsed time format")
	}
}

func inferEffectiveWorkdir(logs []threadLogRow, fallback string) string {
	for _, row := range logs {
		call, ok := parseToolCallMessage(row.Message)
		if !ok {
			continue
		}
		if workdir := extractWorkdir(call); workdir != "" {
			return workdir
		}
	}
	return fallback
}

func summarizeRecentAction(logs []threadLogRow) string {
	for _, row := range logs {
		call, ok := parseToolCallMessage(row.Message)
		if !ok {
			continue
		}

		switch call.Name {
		case "exec_command":
			if cmd := compactWhitespace(stringArg(call.Args, "cmd")); cmd != "" {
				return "exec_command: " + cmd
			}
		case "spawn_agent", "send_input":
			if message := compactWhitespace(stringArg(call.Args, "message")); message != "" {
				return call.Name + ": " + message
			}
			return call.Name
		case "write_stdin":
			return "write_stdin"
		default:
			if message := compactWhitespace(firstNonEmpty(
				stringArg(call.Args, "message"),
				stringArg(call.Args, "question"),
				stringArg(call.Args, "query"),
			)); message != "" {
				return call.Name + ": " + message
			}
			return call.Name
		}
	}
	return ""
}

func parseToolCallMessage(message string) (toolCall, bool) {
	if !strings.HasPrefix(message, defaultRecentActionSuffix) {
		return toolCall{}, false
	}

	rest := strings.TrimSpace(strings.TrimPrefix(message, defaultRecentActionSuffix))
	if rest == "" {
		return toolCall{}, false
	}

	idx := strings.Index(rest, " {")
	if idx == -1 {
		return toolCall{Name: rest}, true
	}

	name := strings.TrimSpace(rest[:idx])
	rawArgs := strings.TrimSpace(rest[idx+1:])
	if name == "" || rawArgs == "" {
		return toolCall{}, false
	}

	args := make(map[string]any)
	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		return toolCall{Name: name}, true
	}

	return toolCall{Name: name, Args: args}, true
}

func extractWorkdir(call toolCall) string {
	if workdir := stringArg(call.Args, "workdir"); workdir != "" {
		return workdir
	}

	cmd := stringArg(call.Args, "cmd")
	if cmd == "" {
		return ""
	}
	return extractShellWorkdir(cmd)
}

func extractShellWorkdir(command string) string {
	matches := changeDirCommandRegex.FindStringSubmatch(command)
	if len(matches) == 0 {
		return ""
	}

	if matches[2] != "" {
		return strings.TrimSpace(matches[2])
	}
	return strings.TrimSpace(matches[3])
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value, ok := args[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func compactWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func sqliteStringList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, sqliteQuote(value))
	}
	return strings.Join(quoted, ", ")
}

func sqliteQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func unixSecondsToTime(seconds int64) time.Time {
	if seconds <= 0 {
		return time.Time{}
	}
	return time.Unix(seconds, 0).UTC()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
