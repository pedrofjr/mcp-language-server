package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
)

const graphNodeFakeLSPEnv = "MCP_FAKE_LSP_GRAPH_NODE"

func TestHelperProcessGraphNodeFakeLSP(t *testing.T) {
	if os.Getenv(graphNodeFakeLSPEnv) != "1" {
		return
	}

	runGraphNodeFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestGetGraphNode_CallsCustomGraphNodeMethod_AndFormatsIndentedJSON(t *testing.T) {
	client, cleanup := setupGraphNodeFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	result, err := GetGraphNode(ctx, client, "file:///workspace/Foo.pas", "uses_unit")
	if err != nil {
		t.Fatalf("expected GetGraphNode to succeed against fake LSP, got error: %v", err)
	}

	count, err := fakeGraphNodeMethodCallCount(ctx, client, "custom/graph/node")
	if err != nil {
		t.Fatalf("failed to read fake LSP method call count: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected GetGraphNode to call custom/graph/node exactly once, got %d", count)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got unmarshal error: %v\noutput=%s", err, result)
	}

	if payload["nodeId"] != "Foo" {
		t.Fatalf("expected nodeId=Foo, got %#v", payload["nodeId"])
	}
	if payload["relationType"] != "uses_unit" {
		t.Fatalf("expected relationType=uses_unit, got %#v", payload["relationType"])
	}
	if payload["resolved"] != true {
		t.Fatalf("expected resolved=true, got %#v", payload["resolved"])
	}

	relations, ok := payload["relations"].(map[string]any)
	if !ok {
		t.Fatalf("expected relations object, got %#v", payload["relations"])
	}

	imports, ok := relations["imports"].([]any)
	if !ok {
		t.Fatalf("expected relations.imports array, got %#v", relations["imports"])
	}
	if len(imports) != 1 || imports[0] != "Bar" {
		t.Fatalf("expected relations.imports [Bar], got %#v", imports)
	}

	importedBy, ok := relations["importedBy"].([]any)
	if !ok {
		t.Fatalf("expected relations.importedBy array, got %#v", relations["importedBy"])
	}
	if len(importedBy) != 1 || importedBy[0] != "Baz" {
		t.Fatalf("expected relations.importedBy [Baz], got %#v", importedBy)
	}
}

func setupGraphNodeFakeClient(t *testing.T) (*lsp.Client, func()) {
	t.Helper()

	t.Setenv(graphNodeFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessGraphNodeFakeLSP")
	if err != nil {
		t.Fatalf("failed to start fake LSP: %v", err)
	}

	cleanup := func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, t.TempDir()); err != nil {
		cleanup()
		t.Fatalf("failed to initialize fake LSP client: %v", err)
	}

	return client, cleanup
}

func fakeGraphNodeMethodCallCount(ctx context.Context, client *lsp.Client, method string) (int, error) {
	params := map[string]string{"method": method}
	var count int
	if err := client.Call(ctx, "custom/testing/methodCallCount", params, &count); err != nil {
		return 0, err
	}
	return count, nil
}

func runGraphNodeFakeLSP(stdin *os.File, stdout *os.File) {
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
			sendGraphNodeFakeResponse(writer, msg.ID, result, nil)
		case "initialized":
			// no-op
		case "custom/graph/node":
			counts[msg.Method]++
			result := map[string]any{
				"nodeId":       "Foo",
				"relationType": "uses_unit",
				"resolved":     true,
				"relations": map[string]any{
					"imports":    []string{"Bar"},
					"importedBy": []string{"Baz"},
				},
			}
			sendGraphNodeFakeResponse(writer, msg.ID, result, nil)
		case "custom/testing/methodCallCount":
			var params struct {
				Method string `json:"method"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			sendGraphNodeFakeResponse(writer, msg.ID, counts[params.Method], nil)
		case "shutdown":
			sendGraphNodeFakeResponse(writer, msg.ID, nil, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendGraphNodeFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32601, Message: "method not found: " + msg.Method})
			}
		}
	}
}

func sendGraphNodeFakeResponse(w *os.File, id *lsp.MessageID, result any, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
	}

	if rpcErr == nil {
		if result == nil {
			resp.Result = mustMarshalGraphNodeFake(struct{}{})
		} else {
			resp.Result = mustMarshalGraphNodeFake(result)
		}
	}

	_ = lsp.WriteMessage(w, resp)
}

func mustMarshalGraphNodeFake(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal fake LSP json: %v", err))
	}
	return b
}
