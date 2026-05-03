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

const graphQueryFakeLSPEnv = "MCP_FAKE_LSP_GRAPH_QUERY"

func TestHelperProcessGraphQueryFakeLSP(t *testing.T) {
	if os.Getenv(graphQueryFakeLSPEnv) != "1" {
		return
	}

	runGraphQueryFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestGetGraphQuery_CallsCustomGraphQueryMethod_AndFormatsIndentedJSON(t *testing.T) {
	client, cleanup := setupGraphQueryFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	result, err := GetGraphQuery(ctx, client, "file:///workspace/Foo.pas", "uses_unit", "both", 1)
	if err != nil {
		t.Fatalf("expected GetGraphQuery to succeed against fake LSP, got error: %v", err)
	}

	count, err := fakeGraphQueryMethodCallCount(ctx, client, "custom/graph/query")
	if err != nil {
		t.Fatalf("failed to read fake LSP method call count: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected GetGraphQuery to call custom/graph/query exactly once, got %d", count)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got unmarshal error: %v\noutput=%s", err, result)
	}

	if payload["root"] != "Foo" {
		t.Fatalf("expected root=Foo, got %#v", payload["root"])
	}
	if payload["relationType"] != "uses_unit" {
		t.Fatalf("expected relationType=uses_unit, got %#v", payload["relationType"])
	}
	if payload["direction"] != "both" {
		t.Fatalf("expected direction=both, got %#v", payload["direction"])
	}
	if payload["depth"] != float64(1) {
		t.Fatalf("expected depth=1, got %#v", payload["depth"])
	}

	nodes, ok := payload["nodes"].([]any)
	if !ok {
		t.Fatalf("expected nodes array, got %#v", payload["nodes"])
	}
	if len(nodes) < 1 {
		t.Fatalf("expected at least one node, got %#v", nodes)
	}

	edges, ok := payload["edges"].([]any)
	if !ok {
		t.Fatalf("expected edges array, got %#v", payload["edges"])
	}
	if len(edges) < 1 {
		t.Fatalf("expected at least one edge, got %#v", edges)
	}

	stats, ok := payload["stats"].(map[string]any)
	if !ok {
		t.Fatalf("expected stats object, got %#v", payload["stats"])
	}
	if stats["nodeCount"] != float64(2) {
		t.Fatalf("expected stats.nodeCount=2, got %#v", stats["nodeCount"])
	}
	if stats["edgeCount"] != float64(1) {
		t.Fatalf("expected stats.edgeCount=1, got %#v", stats["edgeCount"])
	}
	if stats["truncated"] != false {
		t.Fatalf("expected stats.truncated=false, got %#v", stats["truncated"])
	}
	if stats["rootResolved"] != true {
		t.Fatalf("expected stats.rootResolved=true, got %#v", stats["rootResolved"])
	}
}

func setupGraphQueryFakeClient(t *testing.T) (*lsp.Client, func()) {
	t.Helper()

	t.Setenv(graphQueryFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessGraphQueryFakeLSP")
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

func fakeGraphQueryMethodCallCount(ctx context.Context, client *lsp.Client, method string) (int, error) {
	params := map[string]string{"method": method}
	var count int
	if err := client.Call(ctx, "custom/testing/methodCallCount", params, &count); err != nil {
		return 0, err
	}
	return count, nil
}

func runGraphQueryFakeLSP(stdin *os.File, stdout *os.File) {
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
			sendGraphQueryFakeResponse(writer, msg.ID, result, nil)
		case "initialized":
			// no-op
		case "custom/graph/query":
			counts[msg.Method]++
			result := map[string]any{
				"root":         "Foo",
				"relationType": "uses_unit",
				"direction":    "both",
				"depth":        1,
				"nodes": []map[string]any{
					{"id": "Foo", "kind": "unit", "resolved": true},
					{"id": "Bar", "kind": "unit", "resolved": true},
				},
				"edges": []map[string]any{
					{"source": "Foo", "target": "Bar", "type": "uses_unit", "direction": "imports"},
				},
				"stats": map[string]any{
					"nodeCount":    2,
					"edgeCount":    1,
					"truncated":    false,
					"rootResolved": true,
				},
			}
			sendGraphQueryFakeResponse(writer, msg.ID, result, nil)
		case "custom/testing/methodCallCount":
			var params struct {
				Method string `json:"method"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			sendGraphQueryFakeResponse(writer, msg.ID, counts[params.Method], nil)
		case "shutdown":
			sendGraphQueryFakeResponse(writer, msg.ID, nil, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendGraphQueryFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32601, Message: "method not found: " + msg.Method})
			}
		}
	}
}

func sendGraphQueryFakeResponse(w *os.File, id *lsp.MessageID, result any, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
	}

	if rpcErr == nil {
		if result == nil {
			resp.Result = mustMarshalGraphQueryFake(struct{}{})
		} else {
			resp.Result = mustMarshalGraphQueryFake(result)
		}
	}

	_ = lsp.WriteMessage(w, resp)
}

func mustMarshalGraphQueryFake(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal fake LSP json: %v", err))
	}
	return b
}
