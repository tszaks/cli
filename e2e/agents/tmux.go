package agents

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// TmuxSession implements Session using tmux for PTY-based interactive agents.
type TmuxSession struct {
	name         string
	stableAtSend string // stable content snapshot when Send was last called
}

// NewTmuxSession creates a new tmux session running the given command in dir.
// unsetEnv lists environment variable names to strip from the session.
func NewTmuxSession(name string, dir string, unsetEnv []string, command string, args ...string) (*TmuxSession, error) {
	s := &TmuxSession{name: name}

	tmuxArgs := []string{"new-session", "-d", "-s", name, "-c", dir}
	// Build the shell command, prefixed with env -u for each var to strip.
	shellCmd := ""
	for _, v := range unsetEnv {
		shellCmd += "env -u " + v + " "
	}
	shellCmd += command
	for _, a := range args {
		shellCmd += " " + a
	}
	tmuxArgs = append(tmuxArgs, shellCmd)

	cmd := exec.Command("tmux", tmuxArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("tmux new-session: %w\n%s", err, out)
	}
	// Keep the pane around after the command exits so we can capture error output.
	setCmd := exec.Command("tmux", "set-option", "-t", name, "remain-on-exit", "on")
	_ = setCmd.Run()
	return s, nil
}

func (s *TmuxSession) Send(input string) error {
	s.stableAtSend = stableContent(s.Capture())
	// Send text and Enter separately — Claude's TUI can swallow Enter
	// if it arrives before the input handler finishes processing the text.
	if err := s.SendKeys(input); err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	return s.SendKeys("Enter")
}

// SendKeys sends raw tmux key names without appending Enter.
func (s *TmuxSession) SendKeys(keys ...string) error {
	args := append([]string{"send-keys", "-t", s.name}, keys...)
	cmd := exec.Command("tmux", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux send-keys: %w\n%s", err, out)
	}
	return nil
}

const settleTime = 2 * time.Second

// stableContent returns the content with the last few lines stripped,
// so that TUI status bar updates don't prevent the settle timer.
func stableContent(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) > 3 {
		lines = lines[:len(lines)-3]
	}
	return strings.Join(lines, "\n")
}

func (s *TmuxSession) WaitFor(pattern string, timeout time.Duration) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid pattern: %w", err)
	}

	deadline := time.Now().Add(timeout)
	var matchedAt time.Time
	var lastStable string
	contentChanged := s.stableAtSend == "" // skip change requirement for initial waits

	for time.Now().Before(deadline) {
		content := s.Capture()
		stable := stableContent(content)

		if !re.MatchString(content) {
			// Pattern lost — reset
			matchedAt = time.Time{}
			lastStable = ""
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Detect content change since Send was called
		if !contentChanged && stable != s.stableAtSend {
			contentChanged = true
		}

		if stable != lastStable {
			// Pattern matches but content is still changing — reset settle timer
			matchedAt = time.Now()
			lastStable = stable
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Pattern matches and content hasn't changed since matchedAt.
		// Only settle if content changed at least once after Send
		// (prevents false settle on echoed input before agent starts).
		if contentChanged && time.Since(matchedAt) >= settleTime {
			return content, nil
		}

		time.Sleep(500 * time.Millisecond)
	}
	content := s.Capture()
	return content, fmt.Errorf("timed out waiting for %q after %s\n--- pane content ---\n%s\n--- end pane content ---", pattern, timeout, content)
}

func (s *TmuxSession) Capture() string {
	cmd := exec.Command("tmux", "capture-pane", "-t", s.name, "-p")
	out, _ := cmd.Output()
	return strings.TrimRight(string(out), "\n")
}

func (s *TmuxSession) Close() error {
	cmd := exec.Command("tmux", "kill-session", "-t", s.name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux kill-session: %w\n%s", err, out)
	}
	return nil
}
