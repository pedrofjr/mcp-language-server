package lsp

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

const closeForcedKillHelperEnv = "MCP_FAKE_LSP_CLOSE_FORCED_KILL"

func TestHelperProcessCloseForcedKillSleeper(t *testing.T) {
	if os.Getenv(closeForcedKillHelperEnv) != "1" {
		return
	}

	// Keep process alive long enough to guarantee the forced-kill timeout path.
	time.Sleep(5 * time.Second)
	os.Exit(0)
}

func TestClose_DoesNotPanicWhenForceKillTriggers(t *testing.T) {
	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	cmd := exec.Command(execPath, "-test.run=TestHelperProcessCloseForcedKillSleeper")
	cmd.Env = append(os.Environ(), closeForcedKillHelperEnv+"=1")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("failed to create stdin pipe for helper process: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper process: %v", err)
	}

	defer func() {
		if cmd.Process == nil {
			return
		}

		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	client := &Client{
		Cmd:       cmd,
		stdin:     stdin,
		openFiles: map[string]*OpenFileInfo{},
	}

	var panicValue any
	func() {
		defer func() {
			panicValue = recover()
		}()

		_ = client.Close()
	}()

	if panicValue != nil {
		t.Fatalf("expected Close to avoid panic when forced kill path runs, got panic: %v", panicValue)
	}
}
