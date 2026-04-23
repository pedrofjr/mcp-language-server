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
	"unicode"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
)

const renameSequentialSyncFakeLSPEnv = "MCP_FAKE_LSP_RENAME_SEQUENTIAL_SYNC"

func TestHelperProcessRenameSequentialSyncFakeLSP(t *testing.T) {
	if os.Getenv(renameSequentialSyncFakeLSPEnv) != "1" {
		return
	}

	runRenameSequentialSyncFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestRenameSymbol_SequentialRenameRequiresDidChangeSync(t *testing.T) {
	client, filePath, cleanup := setupRenameSequentialSyncFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	firstResult, err := RenameSymbol(ctx, client, filePath, 3, 7, "Round2")
	if err != nil {
		t.Fatalf("first rename should succeed, got error: %v", err)
	}
	if !strings.Contains(firstResult, "Successfully renamed symbol") {
		t.Fatalf("first rename should report success, got: %s", firstResult)
	}

	secondResult, err := RenameSymbol(ctx, client, filePath, 3, 7, "original")
	if err != nil {
		t.Fatalf("second rename should succeed after local edit sync, got error: %v", err)
	}
	if !strings.Contains(secondResult, "Successfully renamed symbol") {
		t.Fatalf("second rename should report success after local edit sync, got: %s", secondResult)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read fixture after sequential rename: %v", err)
	}

	text := string(content)
	if strings.Contains(text, "Round2") {
		t.Fatalf("expected final content to be reverted to original, but found Round2 in: %s", text)
	}
	if strings.Count(text, "original") != 3 {
		t.Fatalf("expected final content to contain exactly 3 occurrences of original, got %d in: %s", strings.Count(text, "original"), text)
	}
}

func setupRenameSequentialSyncFakeClient(t *testing.T) (*lsp.Client, string, func()) {
	t.Helper()

	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "main.go")
	fixture := "package main\n\nconst original = 1\nvar first = original\nvar second = original\n"
	if err := os.WriteFile(filePath, []byte(fixture), 0o644); err != nil {
		t.Fatalf("failed to write test fixture: %v", err)
	}

	t.Setenv(renameSequentialSyncFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessRenameSequentialSyncFakeLSP")
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

func runRenameSequentialSyncFakeLSP(stdin *os.File, stdout *os.File) {
	reader := bufio.NewReader(stdin)
	writer := stdout
	snapshots := make(map[string]string)

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
			sendRenameSequentialSyncFakeResponse(writer, msg.ID, result, nil)
		case "initialized":
			// no-op
		case "textDocument/didOpen":
			var params struct {
				TextDocument struct {
					URI  string `json:"uri"`
					Text string `json:"text"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			snapshots[params.TextDocument.URI] = params.TextDocument.Text
		case "textDocument/didChange":
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
				ContentChanges []struct {
					Text string `json:"text"`
				} `json:"contentChanges"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			if len(params.ContentChanges) > 0 {
				snapshots[params.TextDocument.URI] = params.ContentChanges[0].Text
			}
		case "textDocument/rename":
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
				Position struct {
					Line      int `json:"line"`
					Character int `json:"character"`
				} `json:"position"`
				NewName string `json:"newName"`
			}
			_ = json.Unmarshal(msg.Params, &params)

			snapshot := snapshots[params.TextDocument.URI]
			token, edits := computeRenameEditsFromSnapshot(snapshot, params.Position.Line, params.Position.Character, params.NewName)
			if token == "" || len(edits) == 0 {
				sendRenameSequentialSyncFakeResponse(writer, msg.ID, map[string]any{"changes": map[string]any{}}, nil)
				continue
			}

			sendRenameSequentialSyncFakeResponse(writer, msg.ID, map[string]any{
				"changes": map[string]any{
					params.TextDocument.URI: edits,
				},
			}, nil)
		case "shutdown":
			sendRenameSequentialSyncFakeResponse(writer, msg.ID, nil, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendRenameSequentialSyncFakeResponse(writer, msg.ID, nil, nil)
			}
		}
	}
}

func computeRenameEditsFromSnapshot(snapshot string, line int, character int, newName string) (string, []map[string]any) {
	if snapshot == "" {
		return "", nil
	}

	lines := strings.Split(snapshot, "\n")
	if line < 0 || line >= len(lines) {
		return "", nil
	}

	token := tokenAtPosition(lines[line], character)
	if token == "" || token == newName {
		return token, nil
	}

	edits := make([]map[string]any, 0)
	for i, lineText := range lines {
		for _, span := range findTokenSpans(lineText, token) {
			edits = append(edits, map[string]any{
				"range": map[string]any{
					"start": map[string]any{"line": i, "character": span[0]},
					"end":   map[string]any{"line": i, "character": span[1]},
				},
				"newText": newName,
			})
		}
	}

	return token, edits
}

func tokenAtPosition(line string, character int) string {
	if line == "" {
		return ""
	}
	runes := []rune(line)
	if character < 0 {
		return ""
	}
	if character >= len(runes) {
		character = len(runes) - 1
	}
	if !isIdentifierRune(runes[character]) {
		if character > 0 && isIdentifierRune(runes[character-1]) {
			character--
		} else {
			return ""
		}
	}

	start := character
	for start > 0 && isIdentifierRune(runes[start-1]) {
		start--
	}
	end := character
	for end+1 < len(runes) && isIdentifierRune(runes[end+1]) {
		end++
	}

	return string(runes[start : end+1])
}

func findTokenSpans(line string, token string) [][2]int {
	target := []rune(token)
	if len(target) == 0 {
		return nil
	}

	runes := []rune(line)
	spans := make([][2]int, 0)
	for i := 0; i+len(target) <= len(runes); i++ {
		if !slicesEqual(runes[i:i+len(target)], target) {
			continue
		}
		if i > 0 && isIdentifierRune(runes[i-1]) {
			continue
		}
		after := i + len(target)
		if after < len(runes) && isIdentifierRune(runes[after]) {
			continue
		}
		spans = append(spans, [2]int{i, i + len(target)})
	}

	return spans
}

func slicesEqual(a []rune, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func isIdentifierRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func sendRenameSequentialSyncFakeResponse(w *os.File, id *lsp.MessageID, result any, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
	}

	if rpcErr == nil {
		if result == nil {
			resp.Result = mustMarshalRenameSequentialSyncFake(struct{}{})
		} else {
			resp.Result = mustMarshalRenameSequentialSyncFake(result)
		}
	}

	_ = lsp.WriteMessage(w, resp)
}

func mustMarshalRenameSequentialSyncFake(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal fake LSP json: %v", err))
	}
	return b
}
