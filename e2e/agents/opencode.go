package agents

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type openCodeAgent struct {
	model   string
	timeout time.Duration
}

func init() {
	if env := os.Getenv("E2E_AGENT"); env != "" && env != "opencode" {
		return
	}
	if _, err := exec.LookPath("opencode"); err != nil {
		return
	}
	model := os.Getenv("E2E_OPENCODE_MODEL")
	if model == "" {
		model = "anthropic/claude-haiku-4-5"
	}
	Register(&openCodeAgent{model: model, timeout: 2 * time.Minute})
}

func (a *openCodeAgent) Name() string               { return "opencode" }
func (a *openCodeAgent) EntireAgent() string        { return "opencode" }
func (a *openCodeAgent) PromptPattern() string      { return `(Ask anything|▣)` }
func (a *openCodeAgent) TimeoutMultiplier() float64 { return 2.0 }

func (a *openCodeAgent) RunPrompt(ctx context.Context, dir string, prompt string, opts ...Option) (Output, error) {
	cfg := &runConfig{}
	for _, o := range opts {
		o(cfg)
	}

	model := a.model
	if cfg.Model != "" {
		model = cfg.Model
	}

	args := []string{"run"}
	if model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, prompt)

	timeout := a.timeout
	if envTimeout := os.Getenv("E2E_TIMEOUT"); envTimeout != "" {
		if parsed, err := time.ParseDuration(envTimeout); err == nil {
			timeout = parsed
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "opencode", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "ENTIRE_TEST_TTY=0")

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	out := Output{
		Command: "opencode " + strings.Join(args, " "),
		Stdout:  stdout.String(),
		Stderr:  stderr.String(),
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			out.ExitCode = exitErr.ExitCode()
		} else {
			out.ExitCode = -1
		}
		return out, err
	}

	return out, nil
}

func (a *openCodeAgent) StartSession(ctx context.Context, dir string) (Session, error) {
	// opencode's TUI occasionally fails to render on CI (empty pane).
	// Retry once if the first attempt produces no output at all.
	var s *TmuxSession
	var lastErr error
	for attempt := range 2 {
		name := fmt.Sprintf("opencode-test-%d", time.Now().UnixNano())
		var err error
		s, err = NewTmuxSession(name, dir, nil, "env", "ENTIRE_TEST_TTY=0", "opencode", "--model", a.model)
		if err != nil {
			return nil, err
		}

		// Wait for TUI to be ready (input area with placeholder text).
		if _, err := s.WaitFor(`Ask anything`, 15*time.Second); err != nil {
			content := s.Capture()
			_ = s.Close()
			if strings.TrimSpace(content) == "" && attempt == 0 {
				lastErr = err
				continue
			}
			return s, fmt.Errorf("waiting for startup: %w", err)
		}
		s.stableAtSend = ""
		return s, nil
	}
	return nil, fmt.Errorf("opencode TUI failed to start after retry: %w", lastErr)
}
