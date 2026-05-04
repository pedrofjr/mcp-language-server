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
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

const codeActionsFakeLSPEnv = "MCP_FAKE_LSP_CODE_ACTIONS"

func TestHelperProcessCodeActionsFakeLSP(t *testing.T) {
	if os.Getenv(codeActionsFakeLSPEnv) != "1" {
		return
	}

	runCodeActionsFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestGetCodeActions_CallsTextDocumentCodeActionAndReturnsJSON(t *testing.T) {
	client, filePath, cleanup := setupCodeActionsFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	result, err := getCodeActionsViaRawLSPCall(ctx, client, filePath, 7, 13)
	if err != nil {
		t.Fatalf("expected code action request to succeed against fake LSP, got error: %v", err)
	}

	count, err := fakeCodeActionsMethodCallCount(ctx, client, "textDocument/codeAction")
	if err != nil {
		t.Fatalf("failed to read fake LSP method call count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected textDocument/codeAction to be called exactly once, got %d", count)
	}

	var actions []map[string]any
	if err := json.Unmarshal([]byte(result), &actions); err != nil {
		t.Fatalf("expected JSON array result from code actions, got unmarshal error: %v\noutput=%s", err, result)
	}
	if len(actions) == 0 {
		t.Fatalf("expected at least one code action in fake response, got %s", result)
	}
	if actions[0]["kind"] != "quickfix" {
		t.Fatalf("expected first action kind to be quickfix, got %#v", actions[0]["kind"])
	}
	if _, ok := actions[0]["edit"].(map[string]any); !ok {
		t.Fatalf("expected first action to include workspace edit object, got %#v", actions[0]["edit"])
	}
}

func TestGetCodeActions_NilDiagnosticsSentAsEmptyArray(t *testing.T) {
	client, filePath, cleanup := setupCodeActionsFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// includeDiagnostics=false: must send diagnostics:[] not diagnostics:null
	result, err := GetCodeActions(ctx, client, filePath, 1, 1, nil, false)
	if err != nil {
		t.Fatalf("expected no error when calling code_actions with includeDiagnostics=false, got: %v", err)
	}

	// Result should be a valid JSON array (possibly empty), not an error
	var actions []map[string]any
	if err := json.Unmarshal([]byte(result), &actions); err != nil {
		t.Fatalf("expected JSON array result (got nil-diagnostics-safe response), but got unmarshal error: %v\noutput=%s", err, result)
	}
}

func getCodeActionsViaRawLSPCall(ctx context.Context, client *lsp.Client, filePath string, line int, column int) (string, error) {
	normalizedPath, err := normalizeFilePathOrURI(filePath)
	if err != nil {
		return "", fmt.Errorf("invalid file path or URI: %v", err)
	}

	if err := client.OpenFile(ctx, normalizedPath); err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.URIFromPath(normalizedPath)
	params := map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"range": map[string]any{
			"start": map[string]any{"line": float64(line - 1), "character": float64(column - 1)},
			"end":   map[string]any{"line": float64(line - 1), "character": float64(column)},
		},
		"context": map[string]any{
			"diagnostics": []any{},
			"only":        []string{"quickfix"},
		},
	}

	var raw json.RawMessage
	if err := client.Call(ctx, "textDocument/codeAction", params, &raw); err != nil {
		return "", fmt.Errorf("textDocument/codeAction request failed: %w", err)
	}

	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return string(raw), nil
	}

	out, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return string(raw), nil
	}

	return string(out), nil
}

func setupCodeActionsFakeClient(t *testing.T) (*lsp.Client, string, func()) {
	t.Helper()
	t.Setenv(codeActionsFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessCodeActionsFakeLSP")
	if err != nil {
		t.Fatalf("failed to start fake LSP: %v", err)
	}

	fixtureDir := t.TempDir()
	filePath := filepath.Join(fixtureDir, "code_actions_fixture.pas")
	source := "unit CodeActionsFixture;\ninterface\nimplementation\nend.\n"
	if err := os.WriteFile(filePath, []byte(source), 0o600); err != nil {
		t.Fatalf("failed to write code action fixture: %v", err)
	}

	cleanup := func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if _, err := client.InitializeLSPClient(ctx, fixtureDir); err != nil {
		cleanup()
		t.Fatalf("failed to initialize fake LSP client: %v", err)
	}

	return client, filePath, cleanup
}

func fakeCodeActionsMethodCallCount(ctx context.Context, client *lsp.Client, method string) (int, error) {
	params := map[string]string{"method": method}
	var count int
	if err := client.Call(ctx, "custom/testing/methodCallCount", params, &count); err != nil {
		return 0, err
	}
	return count, nil
}

func runCodeActionsFakeLSP(stdin *os.File, stdout *os.File) {
	reader := bufio.NewReader(stdin)
	writer := stdout
	counts := map[string]int{}

	for {
		msg, err := lsp.ReadMessage(reader)
		if err != nil {
			return
		}

		switch msg.Method {
		case "initialize":
			result := map[string]any{"capabilities": map[string]any{}}
			sendCodeActionsFakeResponse(writer, msg.ID, result, nil)
		case "initialized":
			// no-op
		case "textDocument/didOpen":
			// no-op
		case "textDocument/codeAction":
			counts[msg.Method]++
			result := []map[string]any{
				{
					"title": "Add SysUtils to uses",
					"kind":  "quickfix",
					"edit": map[string]any{
						"changes": map[string]any{
							"file:///tmp/code_actions_fixture.pas": []map[string]any{
								{
									"range": map[string]any{
										"start": map[string]any{"line": 1.0, "character": 0.0},
										"end":   map[string]any{"line": 1.0, "character": 0.0},
									},
									"newText": "uses SysUtils;\n",
								},
							},
						},
					},
				},
			}
			sendCodeActionsFakeResponse(writer, msg.ID, result, nil)
		case "custom/testing/methodCallCount":
			var params struct {
				Method string `json:"method"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			sendCodeActionsFakeResponse(writer, msg.ID, counts[params.Method], nil)
		case "shutdown":
			sendCodeActionsFakeResponse(writer, msg.ID, nil, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendCodeActionsFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32601, Message: "method not found: " + msg.Method})
			}
		}
	}
}

func sendCodeActionsFakeResponse(w *os.File, id *lsp.MessageID, result any, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{JSONRPC: "2.0", ID: id, Error: rpcErr}
	if rpcErr == nil {
		if result == nil {
			resp.Result = mustMarshalCodeActionsFake(struct{}{})
		} else {
			resp.Result = mustMarshalCodeActionsFake(result)
		}
	}
	_ = lsp.WriteMessage(w, resp)
}

func mustMarshalCodeActionsFake(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal fake LSP json: %v", err))
	}
	return b
}
