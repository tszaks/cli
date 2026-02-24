package agents

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func init() {
	if env := os.Getenv("E2E_AGENT"); env != "" && env != "gemini-cli" {
		return
	}
	Register(&Gemini{})
	RegisterGate("gemini-cli", 3)
}

type Gemini struct{}

func (g *Gemini) Name() string               { return "gemini-cli" }
func (g *Gemini) EntireAgent() string        { return "gemini" }
func (g *Gemini) PromptPattern() string      { return `Type your message` }
func (g *Gemini) TimeoutMultiplier() float64 { return 2.5 }

func (g *Gemini) RunPrompt(ctx context.Context, dir string, prompt string, opts ...Option) (Output, error) {
	cfg := &runConfig{Model: "gemini-2.5-flash"}
	for _, o := range opts {
		o(cfg)
	}

	args := []string{"-p", prompt, "--model", cfg.Model, "-y"}
	displayArgs := []string{"-p", fmt.Sprintf("%q", prompt), "--model", cfg.Model, "-y"}
	cmd := exec.CommandContext(ctx, "gemini", args...)
	cmd.Dir = dir
	cmd.Stdin = nil
	cmd.Env = append(os.Environ(), "ACCESSIBLE=1", "ENTIRE_TEST_TTY=0")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 5 * time.Second

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	exitErr := &exec.ExitError{}
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	}

	return Output{
		Command:  "gemini " + strings.Join(displayArgs, " "),
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, err
}

func (g *Gemini) StartSession(ctx context.Context, dir string) (Session, error) {
	name := fmt.Sprintf("gemini-test-%d", time.Now().UnixNano())
	s, err := NewTmuxSession(name, dir, nil, "env", "ACCESSIBLE=1", "ENTIRE_TEST_TTY=0", "gemini", "-y")
	if err != nil {
		return nil, err
	}

	// Dismiss startup dialogs (workspace trust, etc.)
	for range 5 {
		content, err := s.WaitFor(`(Type your message|trust)`, 15*time.Second)
		if err != nil {
			_ = s.Close()
			return nil, fmt.Errorf("waiting for startup prompt: %w", err)
		}
		if !strings.Contains(content, "trust") {
			break
		}
		_ = s.SendKeys("Enter")
		time.Sleep(500 * time.Millisecond)
	}
	s.stableAtSend = ""

	return s, nil
}
