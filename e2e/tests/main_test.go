//go:build e2e

package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	_ "github.com/entireio/cli/e2e/agents"
	"github.com/entireio/cli/e2e/testutil"
)

func TestMain(m *testing.M) {
	runDir := os.Getenv("E2E_ARTIFACT_DIR")
	if runDir == "" {
		_, file, _, _ := runtime.Caller(0)
		testutil.ArtifactRoot = filepath.Join(filepath.Dir(file), "..", "artifacts")
		runDir = testutil.ArtifactRunDir()
	}
	_ = os.MkdirAll(runDir, 0o755)
	testutil.SetRunDir(runDir)

	version := "unknown"
	if out, err := exec.Command("entire", "version").Output(); err == nil {
		version = string(out)
		_ = os.WriteFile(filepath.Join(runDir, "entire-version.txt"), out, 0o644)
	}

	fmt.Fprintf(os.Stderr, "entire version: %s", version)
	fmt.Fprintf(os.Stderr, "artifact dir:   %s\n", runDir)

	os.Exit(m.Run())
}
