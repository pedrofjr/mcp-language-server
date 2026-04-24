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
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

const diagnosticsURIRegressionFakeLSPEnv = "MCP_FAKE_LSP_DIAGNOSTICS_URI_REGRESSION"
const diagnosticsURIRegressionWithDiagnosticsEnv = "MCP_FAKE_LSP_DIAGNOSTICS_URI_REGRESSION_WITH_DIAGNOSTICS"

func TestHelperProcessDiagnosticsURIRegressionFakeLSP(t *testing.T) {
	if os.Getenv(diagnosticsURIRegressionFakeLSPEnv) != "1" {
		return
	}

	runDiagnosticsURIRegressionFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestGetDiagnosticsForFile_AcceptsFileURIInput(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "main.go")
	fixtureContent := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(filePath, []byte(fixtureContent), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	t.Setenv(diagnosticsURIRegressionFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessDiagnosticsURIRegressionFakeLSP")
	if err != nil {
		t.Fatalf("failed to start fake LSP: %v", err)
	}
	defer func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		t.Fatalf("failed to initialize fake LSP client: %v", err)
	}

	fileURI := string(protocol.URIFromPath(filePath))
	result, err := GetDiagnosticsForFile(ctx, client, fileURI, 0, false)
	if err != nil {
		t.Fatalf("expected GetDiagnosticsForFile to accept file URI input without read/open errors, got error: %v", err)
	}

	expectedOutput := "No diagnostics found for " + fileURI
	if !strings.Contains(result, expectedOutput) {
		t.Fatalf("unexpected diagnostics output for URI input; expected it to contain %q, got %q", expectedOutput, result)
	}
}

func TestGetDiagnosticsForFile_AcceptsFileURIInputWithDiagnostics(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "main.go")
	fixtureContent := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(filePath, []byte(fixtureContent), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	t.Setenv(diagnosticsURIRegressionFakeLSPEnv, "1")
	t.Setenv(diagnosticsURIRegressionWithDiagnosticsEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessDiagnosticsURIRegressionFakeLSP")
	if err != nil {
		t.Fatalf("failed to start fake LSP: %v", err)
	}
	defer func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		t.Fatalf("failed to initialize fake LSP client: %v", err)
	}

	fileURI := string(protocol.URIFromPath(filePath))
	result, err := GetDiagnosticsForFile(ctx, client, fileURI, 0, true)
	if err != nil {
		t.Fatalf("expected GetDiagnosticsForFile to accept URI input and format diagnostics output, got error: %v", err)
	}

	if strings.Contains(result, "Error reading file:") {
		t.Fatalf("expected diagnostics output to read file content via normalized path, got read error output: %q", result)
	}

	if !strings.Contains(result, "Diagnostics in File: 1") {
		t.Fatalf("expected diagnostics count in output, got %q", result)
	}

	if !strings.Contains(result, "ERROR at L1:C1: simulated diagnostic") {
		t.Fatalf("expected diagnostic summary in output, got %q", result)
	}

	if !strings.Contains(result, "1|package main") {
		t.Fatalf("expected line-formatted output to include file content, got %q", result)
	}
}

func runDiagnosticsURIRegressionFakeLSP(stdin *os.File, stdout *os.File) {
	reader := bufio.NewReader(stdin)
	writer := stdout

	for {
		msg, err := lsp.ReadMessage(reader)
		if err != nil {
			return
		}

		switch msg.Method {
		case "initialize":
			result := map[string]any{
				"capabilities": map[string]any{
					"textDocumentSync": 1,
					"diagnosticProvider": map[string]any{
						"interFileDependencies": false,
						"workspaceDiagnostics":  false,
					},
				},
			}
			sendDiagnosticsURIRegressionFakeResponse(writer, msg.ID, result, nil)
		case "initialized":
			// No-op.
		case "textDocument/didOpen":
			if os.Getenv(diagnosticsURIRegressionWithDiagnosticsEnv) == "1" {
				var didOpen protocol.DidOpenTextDocumentParams
				if err := json.Unmarshal(msg.Params, &didOpen); err == nil {
					sendDiagnosticsURIRegressionFakeNotification(writer, "textDocument/publishDiagnostics", protocol.PublishDiagnosticsParams{
						URI: didOpen.TextDocument.URI,
						Diagnostics: []protocol.Diagnostic{
							{
								Range: protocol.Range{
									Start: protocol.Position{Line: 0, Character: 0},
									End:   protocol.Position{Line: 0, Character: 7},
								},
								Severity: protocol.SeverityError,
								Source:   "fake-lsp",
								Message:  "simulated diagnostic",
							},
						},
					})
				}
			}
		case "textDocument/diagnostic":
			sendDiagnosticsURIRegressionFakeResponse(writer, msg.ID, map[string]any{
				"kind":  "full",
				"items": []any{},
			}, nil)
		case "shutdown":
			sendDiagnosticsURIRegressionFakeResponse(writer, msg.ID, nil, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendDiagnosticsURIRegressionFakeResponse(writer, msg.ID, nil, nil)
			}
		}
	}
}

func sendDiagnosticsURIRegressionFakeResponse(w *os.File, id *lsp.MessageID, result any, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
	}

	if rpcErr == nil {
		if result == nil {
			resp.Result = mustMarshalDiagnosticsURIRegressionFake(struct{}{})
		} else {
			resp.Result = mustMarshalDiagnosticsURIRegressionFake(result)
		}
	}

	_ = lsp.WriteMessage(w, resp)
}

func sendDiagnosticsURIRegressionFakeNotification(w *os.File, method string, params any) {
	notification := &lsp.Message{
		JSONRPC: "2.0",
		Method:  method,
		Params:  mustMarshalDiagnosticsURIRegressionFake(params),
	}

	_ = lsp.WriteMessage(w, notification)
}

func mustMarshalDiagnosticsURIRegressionFake(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal fake LSP json: %v", err))
	}
	return b
}
