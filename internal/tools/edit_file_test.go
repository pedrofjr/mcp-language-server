package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
)

const editFileFakeLSPEnv = "MCP_FAKE_LSP_EDIT_FILE"

func TestHelperProcessEditFileFakeLSP(t *testing.T) {
	if os.Getenv(editFileFakeLSPEnv) != "1" {
		return
	}

	runEditFileFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestApplyTextEdits_ConvertsFilesystemPathToFileURIBeforeWorkspaceEdit(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "main.go")
	initialContent := "line one\nline two\n"
	if err := os.WriteFile(filePath, []byte(initialContent), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	t.Setenv(editFileFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessEditFileFakeLSP")
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

	_, err = ApplyTextEdits(ctx, client, filePath, []TextEdit{
		{
			StartLine: 1,
			EndLine:   1,
			NewText:   "updated line",
		},
	})
	if err != nil {
		t.Fatalf("expected ApplyTextEdits to accept filesystem path and convert to file URI, got error: %v", err)
	}

	updatedContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}

	expectedContent := "updated line\nline two\n"
	if string(updatedContent) != expectedContent {
		t.Fatalf("unexpected file content after edit; expected %q, got %q", expectedContent, string(updatedContent))
	}
}

func runEditFileFakeLSP(stdin *os.File, stdout *os.File) {
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
				},
			}
			sendEditFileFakeResponse(writer, msg.ID, result, nil)
		case "initialized", "textDocument/didOpen":
			// No-op.
		case "shutdown":
			sendEditFileFakeResponse(writer, msg.ID, nil, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendEditFileFakeResponse(writer, msg.ID, nil, nil)
			}
		}
	}
}

func sendEditFileFakeResponse(w *os.File, id *lsp.MessageID, result any, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
	}

	if rpcErr == nil {
		if result == nil {
			resp.Result = mustMarshalEditFileFake(struct{}{})
		} else {
			resp.Result = mustMarshalEditFileFake(result)
		}
	}

	_ = lsp.WriteMessage(w, resp)
}

func mustMarshalEditFileFake(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal fake LSP json: %v", err))
	}
	return b
}
