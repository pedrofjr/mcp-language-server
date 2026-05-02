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

const renameCrossFileWorkspaceEditFakeLSPEnv = "MCP_FAKE_LSP_RENAME_CROSS_FILE_WORKSPACE_EDIT"

func TestHelperProcessRenameCrossFileWorkspaceEditFakeLSP(t *testing.T) {
	if os.Getenv(renameCrossFileWorkspaceEditFakeLSPEnv) != "1" {
		return
	}

	runRenameCrossFileWorkspaceEditFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestRenameSymbol_CrossFileConsumers_PathWithSpace_PersistsExactTokenOnDisk(t *testing.T) {
	workspaceDir := filepath.Join(t.TempDir(), "workspace with space")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("failed to create workspace directory with space: %v", err)
	}

	producerPath := filepath.Join(workspaceDir, "producer.pas")
	if err := os.WriteFile(producerPath, []byte("unit Producer;\ninterface\nimplementation\nend.\n"), 0o644); err != nil {
		t.Fatalf("failed to write producer fixture: %v", err)
	}

	consumersDir := filepath.Join(workspaceDir, "consumers with space")
	if err := os.MkdirAll(consumersDir, 0o755); err != nil {
		t.Fatalf("failed to create consumers directory with space: %v", err)
	}

	consumerLine := "  s := GetHighlightersFilter(fHighlighters);"
	consumerPaths := []string{
		filepath.Join(consumersDir, "consumer1.pas"),
		filepath.Join(consumersDir, "consumer2.pas"),
		filepath.Join(workspaceDir, "consumer3.pas"),
	}

	for _, path := range consumerPaths {
		if err := os.WriteFile(path, []byte(consumerLine), 0o644); err != nil {
			t.Fatalf("failed to write consumer fixture %q: %v", path, err)
		}
	}

	t.Setenv(renameCrossFileWorkspaceEditFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessRenameCrossFileWorkspaceEditFakeLSP")
	if err != nil {
		t.Fatalf("failed to start fake LSP: %v", err)
	}
	defer func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		t.Fatalf("failed to initialize fake LSP client: %v", err)
	}

	result, err := RenameSymbol(ctx, client, producerPath, 1, 1, "GetHighlightersFilterStr")
	if err != nil {
		t.Fatalf("expected rename to succeed, got error: %v", err)
	}

	if !strings.Contains(result, "Successfully renamed symbol") {
		t.Fatalf("expected success message from rename, got: %s", result)
	}

	want := "  s := GetHighlightersFilterStr(fHighlighters);"
	for _, path := range consumerPaths {
		gotBytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read updated consumer file %q: %v", path, err)
		}
		got := string(gotBytes)
		if got != want {
			t.Fatalf("unexpected consumer content at %q; got %q, want %q", path, got, want)
		}
		if strings.Contains(got, "GetHighlGetHighlightersFilterStr") {
			t.Fatalf("invalid duplicated token artifact detected at %q: %q", path, got)
		}
	}
}

func runRenameCrossFileWorkspaceEditFakeLSP(stdin *os.File, stdout *os.File) {
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
					"renameProvider": true,
				},
			}
			sendRenameCrossFileWorkspaceEditFakeResponse(writer, msg.ID, result, nil)
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
		case "textDocument/didChange":
			// no-op
		case "textDocument/rename":
			if openedURI == "" {
				sendRenameCrossFileWorkspaceEditFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{
					Code:    -32602,
					Message: "rename requested before didOpen",
				})
				continue
			}

			openedPath, err := protocol.ParseDocumentUri(openedURI)
			if err != nil {
				sendRenameCrossFileWorkspaceEditFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{
					Code:    -32602,
					Message: fmt.Sprintf("failed to parse opened URI: %v", err),
				})
				continue
			}

			workspaceDir := filepath.Dir(openedPath.Path())
			consumerPaths := []string{
				filepath.Join(workspaceDir, "consumers with space", "consumer1.pas"),
				filepath.Join(workspaceDir, "consumers with space", "consumer2.pas"),
				filepath.Join(workspaceDir, "consumer3.pas"),
			}

			changes := map[string]any{}
			for _, path := range consumerPaths {
				changes[string(protocol.URIFromPath(path))] = []map[string]any{{
					"range": map[string]any{
						"start": map[string]any{"line": 0, "character": 7},
						"end":   map[string]any{"line": 0, "character": 28},
					},
					"newText": "GetHighlightersFilterStr",
				}}
			}

			sendRenameCrossFileWorkspaceEditFakeResponse(writer, msg.ID, map[string]any{
				"changes": changes,
			}, nil)
		case "shutdown":
			sendRenameCrossFileWorkspaceEditFakeResponse(writer, msg.ID, nil, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendRenameCrossFileWorkspaceEditFakeResponse(writer, msg.ID, nil, nil)
			}
		}
	}
}

func sendRenameCrossFileWorkspaceEditFakeResponse(w *os.File, id *lsp.MessageID, result any, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
	}

	if rpcErr == nil {
		if result == nil {
			resp.Result = mustMarshalRenameCrossFileWorkspaceEditFake(struct{}{})
		} else {
			resp.Result = mustMarshalRenameCrossFileWorkspaceEditFake(result)
		}
	}

	_ = lsp.WriteMessage(w, resp)
}

func mustMarshalRenameCrossFileWorkspaceEditFake(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal fake LSP json: %v", err))
	}
	return b
}
