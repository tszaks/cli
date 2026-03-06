package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	codexagent "github.com/entireio/cli/cmd/entire/cli/agent/codex"
	"github.com/entireio/cli/cmd/entire/cli/jsonutil"
	"github.com/entireio/cli/cmd/entire/cli/stringutil"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const codexWatchInterval = 2 * time.Second

const annotationSkipVersionCheckWhenJSON = "entire.io/skip-version-check-when-json"

var collectCodexSessions = codexagent.CollectSessions
var runCodexResume = execCodexResume

func newCodexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codex",
		Short: "Codex utilities",
	}

	cmd.AddCommand(newCodexSessionsCmd())
	cmd.AddCommand(newCodexResumeCmd())
	return cmd
}

func newCodexSessionsCmd() *cobra.Command {
	var includeAll bool
	var jsonOutput bool
	var includeDetails bool
	var watch bool

	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Show live Codex sessions",
		Args:  cobra.NoArgs,
		Annotations: map[string]string{
			annotationSkipVersionCheckWhenJSON: "true",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if watch && jsonOutput {
				return fmt.Errorf("--watch cannot be used with --json")
			}

			opts := codexagent.SessionCollectOptions{
				IncludeAll:     includeAll,
				IncludeDetails: includeDetails,
			}

			if watch {
				return watchCodexSessions(cmd.Context(), cmd.OutOrStdout(), opts)
			}
			return renderCodexSessionsOnce(cmd.Context(), cmd.OutOrStdout(), opts, jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&includeAll, "all", false, "Include inactive Codex threads too")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output JSON instead of a table")
	cmd.Flags().BoolVar(&includeDetails, "details", false, "Show recent action details")
	cmd.Flags().BoolVar(&watch, "watch", false, "Refresh the table every 2 seconds")

	return cmd
}

func newCodexResumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resume <thread-id-or-prefix>",
		Short: "Resume a Codex session by thread id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			session, err := resolveCodexSession(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return runCodexResume(cmd.Context(), session.ThreadID)
		},
	}

	return cmd
}

func watchCodexSessions(ctx context.Context, w io.Writer, opts codexagent.SessionCollectOptions) error {
	ticker := time.NewTicker(codexWatchInterval)
	defer ticker.Stop()

	for {
		if supportsScreenClear(w) {
			fmt.Fprint(w, "\033[H\033[2J")
		}
		if err := renderCodexSessionsOnce(ctx, w, opts, false); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func renderCodexSessionsOnce(ctx context.Context, w io.Writer, opts codexagent.SessionCollectOptions, jsonOutput bool) error {
	snapshot, err := collectCodexSessions(ctx, opts)
	if err != nil {
		return err
	}

	if jsonOutput {
		data, err := jsonutil.MarshalIndentWithNewline(snapshot, "", "  ")
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}

	renderCodexSessionsTable(w, snapshot, opts)
	return nil
}

func renderCodexSessionsTable(w io.Writer, snapshot *codexagent.SessionSnapshot, opts codexagent.SessionCollectOptions) {
	sty := newStatusStyles(w)
	activeCount, inactiveCount := countCodexSessions(snapshot.Sessions)

	fmt.Fprintln(w)
	if opts.IncludeAll {
		fmt.Fprintf(w, "%s\n", sty.render(sty.bold, fmt.Sprintf("%d active, %d inactive Codex sessions", activeCount, inactiveCount)))
	} else {
		fmt.Fprintf(w, "%s\n", sty.render(sty.bold, fmt.Sprintf("%d active Codex sessions", activeCount)))
	}
	fmt.Fprintf(w, "%s %s\n", sty.render(sty.dim, "updated"), sty.render(sty.dim, snapshot.GeneratedAt.Local().Format(time.Kitchen)))
	fmt.Fprintln(w)

	if len(snapshot.Sessions) == 0 {
		if opts.IncludeAll {
			fmt.Fprintln(w, "No Codex sessions found.")
		} else {
			fmt.Fprintln(w, "No active Codex sessions found.")
		}
		fmt.Fprintln(w)
		return
	}

	projectWidth, topicWidth := codexColumnWidths(sty.width, opts.IncludeAll)
	printCodexHeader(w, sty, opts.IncludeAll, projectWidth, topicWidth)

	for _, session := range snapshot.Sessions {
		printCodexRow(w, sty, session, opts.IncludeAll, projectWidth, topicWidth)
		if opts.IncludeDetails && session.RecentAction != "" {
			details := stringutil.TruncateRunes(stringutil.CollapseWhitespace(session.RecentAction), maxInt(20, sty.width-10), "...")
			fmt.Fprintf(w, "%s %s\n", sty.render(sty.dim, "recent:"), sty.render(sty.dim, details))
		}
	}

	fmt.Fprintln(w)
}

func printCodexHeader(w io.Writer, sty statusStyles, includeAll bool, projectWidth, topicWidth int) {
	headerFmt := "%-5s %-7s %-7s "
	if includeAll {
		headerFmt += "%-8s "
	}
	headerFmt += "%-8s %-*s %-*s %-11s\n"

	args := []any{"PID", "TTY", "AGE"}
	if includeAll {
		args = append(args, "STATUS")
	}
	args = append(args, "THREAD", projectWidth, "PROJECT", topicWidth, "TOPIC", "LAST ACTIVE")
	fmt.Fprintf(w, sty.render(sty.bold, headerFmt), args...)
}

func printCodexRow(w io.Writer, sty statusStyles, session codexagent.SessionSummary, includeAll bool, projectWidth, topicWidth int) {
	rowFmt := "%-5s %-7s %-7s "
	if includeAll {
		rowFmt += "%-8s "
	}
	rowFmt += "%-8s %-*s %-*s %-11s\n"

	pid := "-"
	if session.PID > 0 {
		pid = fmt.Sprintf("%d", session.PID)
	}

	tty := session.TTY
	if tty == "" {
		tty = "-"
	}
	age := "-"
	if session.AgeSeconds > 0 {
		age = formatShortDuration(session.AgeSeconds)
	}

	threadID := "-"
	if session.ThreadID != "" {
		threadID = shortThreadID(session.ThreadID)
	}

	project := stringutil.TruncateRunes(displayPath(session.EffectiveWorkdir), projectWidth, "...")
	topic := stringutil.TruncateRunes(stringutil.CollapseWhitespace(session.Title), topicWidth, "...")
	lastActive := "-"
	if !session.LastActiveAt.IsZero() {
		lastActive = timeAgo(session.LastActiveAt.Local())
	}

	args := []any{pid, tty, age}
	if includeAll {
		args = append(args, session.Status)
	}
	args = append(args, threadID, projectWidth, project, topicWidth, topic, lastActive)
	fmt.Fprintf(w, rowFmt, args...)
}

func countCodexSessions(sessions []codexagent.SessionSummary) (active int, inactive int) {
	for _, session := range sessions {
		if session.Status == "active" {
			active++
			continue
		}
		inactive++
	}
	return active, inactive
}

func codexColumnWidths(width int, includeAll bool) (projectWidth int, topicWidth int) {
	// The repo caps terminal width at 80, so keep these compact and predictable.
	baseWidth := 5 + 7 + 7 + 8 + 11
	spaces := 6
	if includeAll {
		baseWidth += 8
		spaces++
	}

	available := width - baseWidth - spaces
	if available < 20 {
		available = 20
	}

	projectWidth = available / 2
	topicWidth = available - projectWidth
	if projectWidth < 10 {
		projectWidth = 10
	}
	if topicWidth < 10 {
		topicWidth = 10
	}
	return projectWidth, topicWidth
}

func formatShortDuration(seconds int64) string {
	switch {
	case seconds < 60:
		return fmt.Sprintf("%ds", seconds)
	case seconds < 3600:
		return fmt.Sprintf("%dm", seconds/60)
	case seconds < 86400:
		hours := seconds / 3600
		minutes := (seconds % 3600) / 60
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%dm", hours, minutes)
	default:
		days := seconds / 86400
		hours := (seconds % 86400) / 3600
		if hours == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd%dh", days, hours)
	}
}

func shortThreadID(threadID string) string {
	if len(threadID) <= 8 {
		return threadID
	}
	return threadID[:8]
}

func displayPath(path string) string {
	if path == "" {
		return "-"
	}

	homeDir, err := os.UserHomeDir()
	if err == nil && path == homeDir {
		return "~"
	}
	if err == nil && homeDir != "" {
		prefix := homeDir + string(os.PathSeparator)
		if len(path) > len(prefix) && path[:len(prefix)] == prefix {
			return "~/" + path[len(prefix):]
		}
	}

	return path
}

func supportsScreenClear(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func resolveCodexSession(ctx context.Context, query string) (codexagent.SessionSummary, error) {
	snapshot, err := collectCodexSessions(ctx, codexagent.SessionCollectOptions{IncludeAll: true})
	if err != nil {
		return codexagent.SessionSummary{}, err
	}

	var matches []codexagent.SessionSummary
	for _, session := range snapshot.Sessions {
		if session.ThreadID == query || strings.HasPrefix(session.ThreadID, query) {
			matches = append(matches, session)
		}
	}

	switch len(matches) {
	case 0:
		return codexagent.SessionSummary{}, fmt.Errorf("no Codex session found matching %q", query)
	case 1:
		return matches[0], nil
	default:
		ids := make([]string, 0, len(matches))
		for _, match := range matches {
			ids = append(ids, shortThreadID(match.ThreadID))
		}
		return codexagent.SessionSummary{}, fmt.Errorf("multiple Codex sessions match %q: %s", query, strings.Join(ids, ", "))
	}
}

func execCodexResume(ctx context.Context, threadID string) error {
	if strings.TrimSpace(threadID) == "" {
		return errors.New("thread id is required")
	}

	cmd := exec.CommandContext(ctx, "codex", "resume", threadID)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
