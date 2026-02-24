package agents

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// isolatedConfigDir creates a temp directory that mirrors ~/.claude via
// symlinks but omits CLAUDE.md and skills/ so that test runs don't inherit
// the operator's personal instructions or custom skills.
func isolatedConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	src := filepath.Join(home, ".claude")

	dst, err := os.MkdirTemp("", "claude-config-*")
	if err != nil {
		return "", err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return dst, fmt.Errorf("read %s: %w", src, err)
	}

	skip := map[string]bool{"CLAUDE.md": true, "skills": true}
	for _, e := range entries {
		if skip[e.Name()] {
			continue
		}
		_ = os.Symlink(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()))
	}
	return dst, nil
}

// cleanEnv returns os.Environ() with CLAUDECODE removed so that
// Claude Code doesn't refuse to start inside this test runner.
func cleanEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "CLAUDECODE=") {
			env = append(env, e)
		}
	}
	return env
}

func init() {
	if env := os.Getenv("E2E_AGENT"); env != "" && env != "claude-code" {
		return
	}
	Register(&Claude{})
}

type Claude struct{}

func (c *Claude) Name() string               { return "claude-code" }
func (c *Claude) EntireAgent() string        { return "claude-code" }
func (c *Claude) PromptPattern() string      { return `❯` }
func (c *Claude) TimeoutMultiplier() float64 { return 1.0 }

func (c *Claude) Bootstrap() error {
	// On CI, write a config file so Claude Code uses the API key from the
	// environment instead of trying OAuth/Keychain.
	if os.Getenv("CI") == "" {
		return nil
	}
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	config := fmt.Sprintf(`{"primaryApiKey":%q,"hasCompletedOnboarding":true}`, apiKey)
	path := filepath.Join(dir, ".claude.json")
	return os.WriteFile(path, []byte(config), 0o644)
}

func (c *Claude) RunPrompt(ctx context.Context, dir string, prompt string, opts ...Option) (Output, error) {
	cfg := &runConfig{Model: "haiku"}
	for _, o := range opts {
		o(cfg)
	}

	configDir, err := isolatedConfigDir()
	if err != nil {
		return Output{}, fmt.Errorf("create isolated config dir: %w", err)
	}
	defer os.RemoveAll(configDir)

	args := []string{"-p", prompt, "--model", cfg.Model, "--dangerously-skip-permissions"}
	displayArgs := []string{"-p", fmt.Sprintf("%q", prompt), "--model", cfg.Model, "--dangerously-skip-permissions"}
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = dir
	cmd.Stdin = nil
	cmd.Env = append(cleanEnv(), "ACCESSIBLE=1", "ENTIRE_TEST_TTY=0", "CLAUDE_CONFIG_DIR="+configDir)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 5 * time.Second

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	exitCode := 0
	exitErr := &exec.ExitError{}
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	}

	return Output{
		Command:  "claude " + strings.Join(displayArgs, " "),
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, err
}

func (c *Claude) StartSession(ctx context.Context, dir string) (Session, error) {
	name := fmt.Sprintf("claude-test-%d", time.Now().UnixNano())

	envArgs := []string{"ACCESSIBLE=1", "ENTIRE_TEST_TTY=0"}

	// On CI (no macOS Keychain), use an isolated config dir so Claude Code
	// picks up ANTHROPIC_API_KEY from the environment instead of trying OAuth.
	// Locally, we skip CLAUDE_CONFIG_DIR so the Keychain-based auth works.
	if os.Getenv("CI") != "" {
		configDir, err := isolatedConfigDir()
		if err == nil {
			envArgs = append(envArgs, "CLAUDE_CONFIG_DIR="+configDir)
		}
	}

	args := append([]string{"env"}, envArgs...)
	args = append(args, "claude", "--dangerously-skip-permissions")
	s, err := NewTmuxSession(name, dir, []string{"CLAUDECODE"}, args[0], args[1:]...)
	if err != nil {
		return nil, err
	}

	// Dismiss startup dialogs until we reach the input prompt.
	for range 5 {
		content, err := s.WaitFor(`❯`, 15*time.Second)
		if err != nil {
			_ = s.Close()
			return nil, fmt.Errorf("waiting for startup prompt: %w", err)
		}
		if !strings.Contains(content, "Enter to confirm") {
			break
		}
		// The bypass permissions dialog defaults to "No, exit" —
		// arrow down to "Yes, I accept" before confirming.
		if strings.Contains(content, "Yes, I accept") {
			_ = s.SendKeys("Down")
			time.Sleep(200 * time.Millisecond)
		}
		_ = s.SendKeys("Enter")
		time.Sleep(500 * time.Millisecond)
	}
	s.stableAtSend = ""

	return s, nil
}
