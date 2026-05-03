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

const graphNeighborsFakeLSPEnv = "MCP_FAKE_LSP_GRAPH_NEIGHBORS"
const graphNeighborsFakeLSPReturnNullEnv = "MCP_FAKE_LSP_GRAPH_NEIGHBORS_RETURN_NULL"
const graphNeighborsFakeLSPReturnEmptyEnv = "MCP_FAKE_LSP_GRAPH_NEIGHBORS_RETURN_EMPTY"

func TestHelperProcessGraphNeighborsFakeLSP(t *testing.T) {
	if os.Getenv(graphNeighborsFakeLSPEnv) != "1" {
		return
	}

	runGraphNeighborsFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestGetGraphNeighbors_CallsCustomGraphNeighborsMethod(t *testing.T) {
	client, cleanup := setupGraphNeighborsFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := GetGraphNeighbors(ctx, client, "file:///workspace/Foo.pas", "uses_unit", "imports"); err != nil {
		t.Fatalf("expected GetGraphNeighbors to succeed against fake LSP, got error: %v", err)
	}

	count, err := fakeGraphNeighborsMethodCallCount(ctx, client, "custom/graph/neighbors")
	if err != nil {
		t.Fatalf("failed to read fake LSP method call count: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected GetGraphNeighbors to call custom/graph/neighbors exactly once, got %d", count)
	}
}

func TestGetGraphNeighbors_WhenLSPReturnsObject_FormatsIndentedJSON(t *testing.T) {
	client, cleanup := setupGraphNeighborsFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	result, err := GetGraphNeighbors(ctx, client, "file:///workspace/Foo.pas", "uses_unit", "imports")
	if err != nil {
		t.Fatalf("expected GetGraphNeighbors to format object result, got error: %v", err)
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
	if payload["direction"] != "imports" {
		t.Fatalf("expected direction=imports, got %#v", payload["direction"])
	}
	if payload["resolved"] != true {
		t.Fatalf("expected resolved=true, got %#v", payload["resolved"])
	}

	neighbors, ok := payload["neighbors"].([]any)
	if !ok {
		t.Fatalf("expected neighbors array, got %#v", payload["neighbors"])
	}
	if len(neighbors) != 2 || neighbors[0] != "Bar" || neighbors[1] != "Baz" {
		t.Fatalf("expected neighbors [Bar, Baz], got %#v", neighbors)
	}
}

func TestGetGraphNeighbors_WhenLSPReturnsNullOrEmpty_ReturnsFriendlyMessage(t *testing.T) {
	testCases := []struct {
		name       string
		envVarName string
	}{
		{name: "null payload", envVarName: graphNeighborsFakeLSPReturnNullEnv},
		{name: "empty payload", envVarName: graphNeighborsFakeLSPReturnEmptyEnv},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.envVarName, "1")

			client, cleanup := setupGraphNeighborsFakeClient(t)
			defer cleanup()

			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()

			result, err := GetGraphNeighbors(ctx, client, "file:///workspace/Foo.pas", "uses_unit", "imports")
			if err != nil {
				t.Fatalf("expected GetGraphNeighbors to handle %s without error, got: %v", tc.name, err)
			}

			const expected = "Graph neighbors not available for the requested file or relation."
			if result != expected {
				t.Fatalf("unexpected friendly message for %s; expected %q, got %q", tc.name, expected, result)
			}
		})
	}
}

func setupGraphNeighborsFakeClient(t *testing.T) (*lsp.Client, func()) {
	t.Helper()

	t.Setenv(graphNeighborsFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessGraphNeighborsFakeLSP")
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

func fakeGraphNeighborsMethodCallCount(ctx context.Context, client *lsp.Client, method string) (int, error) {
	params := map[string]string{"method": method}
	var count int
	if err := client.Call(ctx, "custom/testing/methodCallCount", params, &count); err != nil {
		return 0, err
	}
	return count, nil
}

func runGraphNeighborsFakeLSP(stdin *os.File, stdout *os.File) {
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
			sendGraphNeighborsFakeResponse(writer, msg.ID, result, nil)
		case "initialized":
			// no-op
		case "custom/graph/neighbors":
			counts[msg.Method]++
			if os.Getenv(graphNeighborsFakeLSPReturnNullEnv) == "1" {
				sendGraphNeighborsFakeRawResponse(writer, msg.ID, json.RawMessage("null"), nil)
				continue
			}
			if os.Getenv(graphNeighborsFakeLSPReturnEmptyEnv) == "1" {
				sendGraphNeighborsFakeRawResponse(writer, msg.ID, json.RawMessage(""), nil)
				continue
			}
			result := map[string]any{
				"nodeId":       "Foo",
				"relationType": "uses_unit",
				"direction":    "imports",
				"resolved":     true,
				"neighbors":    []string{"Bar", "Baz"},
			}
			sendGraphNeighborsFakeResponse(writer, msg.ID, result, nil)
		case "custom/testing/methodCallCount":
			var params struct {
				Method string `json:"method"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			sendGraphNeighborsFakeResponse(writer, msg.ID, counts[params.Method], nil)
		case "shutdown":
			sendGraphNeighborsFakeResponse(writer, msg.ID, nil, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendGraphNeighborsFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32601, Message: "method not found: " + msg.Method})
			}
		}
	}
}

func sendGraphNeighborsFakeResponse(w *os.File, id *lsp.MessageID, result any, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
	}

	if rpcErr == nil {
		if result == nil {
			resp.Result = mustMarshalGraphNeighborsFake(struct{}{})
		} else {
			resp.Result = mustMarshalGraphNeighborsFake(result)
		}
	}

	_ = lsp.WriteMessage(w, resp)
}

func sendGraphNeighborsFakeRawResponse(w *os.File, id *lsp.MessageID, raw json.RawMessage, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
		Result:  raw,
	}
	_ = lsp.WriteMessage(w, resp)
}

func mustMarshalGraphNeighborsFake(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal fake LSP json: %v", err))
	}
	return b
}
