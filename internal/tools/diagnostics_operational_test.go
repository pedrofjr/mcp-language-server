package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
)

const diagnosticsOperationalFakeLSPEnv = "MCP_FAKE_LSP_DIAGNOSTICS_OPERATIONAL"
const diagnosticsOperationalFailPullEnv = "MCP_FAKE_LSP_DIAGNOSTICS_FAIL_PULL"

func TestHelperProcessDiagnosticsOperationalFakeLSP(t *testing.T) {
	if os.Getenv(diagnosticsOperationalFakeLSPEnv) != "1" {
		return
	}

	runDiagnosticsOperationalFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestGetDiagnosticsForFile_PullFailureWithoutCacheReturnsOperationalError(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	t.Setenv(diagnosticsOperationalFakeLSPEnv, "1")
	t.Setenv(diagnosticsOperationalFailPullEnv, "1")

	client := startDiagnosticsOperationalFakeLSPClient(t, workspaceDir)
	defer killDiagnosticsOperationalFakeLSPClient(client)

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	_, err := GetDiagnosticsForFile(ctx, client, filePath, 0, false)
	if err == nil {
		t.Fatal("expected operational error when pull fails without cache, got nil")
	}
	if !strings.Contains(err.Error(), "diagnostics unavailable") {
		t.Fatalf("expected diagnostics unavailable error, got %v", err)
	}
	if strings.Contains(err.Error(), "No diagnostics found") {
		t.Fatalf("must not masquerade operational failure as empty diagnostics: %v", err)
	}
}

func TestGetDiagnosticsForFile_PullSuccessWithEmptyItemsReturnsNoDiagnosticsMessage(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	t.Setenv(diagnosticsOperationalFakeLSPEnv, "1")

	client := startDiagnosticsOperationalFakeLSPClient(t, workspaceDir)
	defer killDiagnosticsOperationalFakeLSPClient(client)

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	result, err := GetDiagnosticsForFile(ctx, client, filePath, 0, false)
	if err != nil {
		t.Fatalf("expected success for empty pull report, got error: %v", err)
	}
	if !strings.Contains(result, "No diagnostics found for ") {
		t.Fatalf("expected explicit empty diagnostics message, got %q", result)
	}
}

func startDiagnosticsOperationalFakeLSPClient(t *testing.T, workspaceDir string) *lsp.Client {
	t.Helper()

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessDiagnosticsOperationalFakeLSP")
	if err != nil {
		t.Fatalf("failed to start fake LSP: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		killDiagnosticsOperationalFakeLSPClient(client)
		t.Fatalf("failed to initialize fake LSP: %v", err)
	}

	return client
}

func killDiagnosticsOperationalFakeLSPClient(client *lsp.Client) {
	if client == nil || client.Cmd == nil || client.Cmd.Process == nil {
		return
	}
	_ = client.Cmd.Process.Kill()
	_, _ = client.Cmd.Process.Wait()
}

func runDiagnosticsOperationalFakeLSP(stdin *os.File, stdout *os.File) {
	reader := bufio.NewReader(stdin)
	writer := stdout

	for {
		msg, err := lsp.ReadMessage(reader)
		if err != nil {
			return
		}

		switch msg.Method {
		case "initialize":
			sendDiagnosticsOperationalFakeResponse(writer, msg.ID, map[string]any{
				"capabilities": map[string]any{
					"textDocumentSync": 1,
					"diagnosticProvider": map[string]any{
						"interFileDependencies": false,
						"workspaceDiagnostics":  false,
					},
				},
			}, nil)
		case "initialized":
		case "textDocument/diagnostic":
			if os.Getenv(diagnosticsOperationalFailPullEnv) == "1" {
				sendDiagnosticsOperationalFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{
					Code:    -32603,
					Message: "simulated diagnostic provider failure",
				})
				continue
			}
			sendDiagnosticsOperationalFakeResponse(writer, msg.ID, map[string]any{
				"kind":  "full",
				"items": []any{},
			}, nil)
		case "shutdown":
			sendDiagnosticsOperationalFakeResponse(writer, msg.ID, nil, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendDiagnosticsOperationalFakeResponse(writer, msg.ID, nil, nil)
			}
		}
	}
}

func sendDiagnosticsOperationalFakeResponse(w *os.File, id *lsp.MessageID, result any, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
	}

	if rpcErr == nil {
		if result == nil {
			resp.Result = mustMarshalDiagnosticsOperationalFake(struct{}{})
		} else {
			resp.Result = mustMarshalDiagnosticsOperationalFake(result)
		}
	}

	_ = lsp.WriteMessage(w, resp)
}

func mustMarshalDiagnosticsOperationalFake(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal fake LSP json: %v", err))
	}
	return b
}
