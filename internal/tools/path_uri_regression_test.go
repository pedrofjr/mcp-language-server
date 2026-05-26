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

const pathURIRegressionFakeLSPEnv = "MCP_FAKE_LSP_PATH_URI_REGRESSION"

func TestHelperProcessPathURIRegressionFakeLSP(t *testing.T) {
	if os.Getenv(pathURIRegressionFakeLSPEnv) != "1" {
		return
	}

	runPathURIRegressionFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestGetHoverInfo_ConvertsFilesystemPathToCanonicalFileURI(t *testing.T) {
	client, filePath, cleanup := setupPathURIRegressionFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	result, err := GetHoverInfo(ctx, client, filePath, 1, 1)
	if err != nil {
		t.Fatalf("expected GetHoverInfo to convert filesystem path to canonical URI, got error: %v", err)
	}

	if result != "hover ok" {
		t.Fatalf("unexpected hover content: got %q, want %q", result, "hover ok")
	}
}

func TestRenameSymbol_ConvertsFilesystemPathToCanonicalFileURI(t *testing.T) {
	client, filePath, cleanup := setupPathURIRegressionFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	result, err := RenameSymbol(ctx, client, filePath, 1, 1, "renamedSymbol")
	if err != nil {
		t.Fatalf("expected RenameSymbol to convert filesystem path to canonical URI, got error: %v", err)
	}

	if result != "Failed to rename symbol. 0 occurrences found." {
		t.Fatalf("unexpected rename result: got %q", result)
	}
}

func TestGetCodeLens_ConvertsFilesystemPathToCanonicalFileURI(t *testing.T) {
	client, filePath, cleanup := setupPathURIRegressionFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	result, err := GetCodeLens(ctx, client, filePath)
	if err != nil {
		t.Fatalf("expected GetCodeLens to convert filesystem path to canonical URI, got error: %v", err)
	}

	if !strings.Contains(result, "Found 1 code lens items.") {
		t.Fatalf("unexpected codelens result: got %q", result)
	}
}

func TestExecuteCodeLens_ConvertsFilesystemPathToCanonicalFileURI(t *testing.T) {
	client, filePath, cleanup := setupPathURIRegressionFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	result, err := ExecuteCodeLens(ctx, client, filePath, 1)
	if err != nil {
		t.Fatalf("expected ExecuteCodeLens to convert filesystem path to canonical URI, got error: %v", err)
	}

	if result != "Successfully executed code lens command: Run fake code lens" {
		t.Fatalf("unexpected execute codelens result: got %q", result)
	}
}

func setupPathURIRegressionFakeClient(t *testing.T) (*lsp.Client, string, func()) {
	t.Helper()

	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "main.go")
	fixture := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(filePath, []byte(fixture), 0o644); err != nil {
		t.Fatalf("failed to write test fixture: %v", err)
	}

	t.Setenv(pathURIRegressionFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessPathURIRegressionFakeLSP")
	if err != nil {
		t.Fatalf("failed to start fake LSP: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		_ = client.Close()
		t.Fatalf("failed to initialize fake LSP client: %v", err)
	}

	cleanup := func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	}

	return client, filePath, cleanup
}

func runPathURIRegressionFakeLSP(stdin *os.File, stdout *os.File) {
	reader := bufio.NewReader(stdin)
	writer := stdout
	openedURI := ""

	for {
		msg, err := lsp.ReadMessage(reader)
		if err != nil {
			return
		}

		switch msg.Method {
		case "initialize":
			result := map[string]any{
				"capabilities": map[string]any{
					"hoverProvider":    true,
					"renameProvider":   true,
					"codeLensProvider": map[string]any{"resolveProvider": false},
				},
			}
			sendPathURIRegressionFakeResponse(writer, msg.ID, result, nil)
		case "initialized":
			// no-op
		case "textDocument/didOpen":
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			openedURI = params.TextDocument.URI
		case "textDocument/hover":
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &params)

			expectedURI := openedURI
			if params.TextDocument.URI != expectedURI {
				sendPathURIRegressionFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{
					Code:    -32602,
					Message: fmt.Sprintf("invalid URI for hover: got=%s expected=%s", params.TextDocument.URI, expectedURI),
				})
				continue
			}

			sendPathURIRegressionFakeResponse(writer, msg.ID, map[string]any{
				"contents": map[string]any{
					"kind":  "markdown",
					"value": "hover ok",
				},
			}, nil)
		case "textDocument/rename":
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &params)

			expectedURI := openedURI
			if params.TextDocument.URI != expectedURI {
				sendPathURIRegressionFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{
					Code:    -32602,
					Message: fmt.Sprintf("invalid URI for rename: got=%s expected=%s", params.TextDocument.URI, expectedURI),
				})
				continue
			}

			sendPathURIRegressionFakeResponse(writer, msg.ID, map[string]any{
				"changes": map[string]any{},
			}, nil)
		case "textDocument/codeLens":
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &params)

			expectedURI := openedURI
			if params.TextDocument.URI != expectedURI {
				sendPathURIRegressionFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{
					Code:    -32602,
					Message: fmt.Sprintf("invalid URI for codeLens: got=%s expected=%s", params.TextDocument.URI, expectedURI),
				})
				continue
			}

			sendPathURIRegressionFakeResponse(writer, msg.ID, []map[string]any{
				{
					"range": map[string]any{
						"start": map[string]any{"line": 0, "character": 0},
						"end":   map[string]any{"line": 0, "character": 4},
					},
					"command": map[string]any{
						"title":   "Run fake code lens",
						"command": "fake.codelens.run",
					},
				},
			}, nil)
		case "workspace/executeCommand":
			sendPathURIRegressionFakeResponse(writer, msg.ID, struct{}{}, nil)
		case "shutdown":
			sendPathURIRegressionFakeResponse(writer, msg.ID, nil, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendPathURIRegressionFakeResponse(writer, msg.ID, nil, nil)
			}
		}
	}
}

func sendPathURIRegressionFakeResponse(w *os.File, id *lsp.MessageID, result any, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
	}

	if rpcErr == nil {
		if result == nil {
			resp.Result = mustMarshalPathURIRegressionFake(struct{}{})
		} else {
			resp.Result = mustMarshalPathURIRegressionFake(result)
		}
	}

	_ = lsp.WriteMessage(w, resp)
}

func mustMarshalPathURIRegressionFake(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal fake LSP json: %v", err))
	}
	return b
}
