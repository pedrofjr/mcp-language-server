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

const callGraphFakeLSPEnv = "MCP_FAKE_LSP_CALL_GRAPH"
const callGraphFakeLSPReturnNullEnv = "MCP_FAKE_LSP_CALL_GRAPH_RETURN_NULL"

func TestHelperProcessCallGraphFakeLSP(t *testing.T) {
	if os.Getenv(callGraphFakeLSPEnv) != "1" {
		return
	}

	runCallGraphFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestGetCallGraph_CallsCustomCallGraphMethod(t *testing.T) {
	client, cleanup := setupCallGraphFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := GetCallGraph(ctx, client, "Unit1.DoWork", 2); err != nil {
		t.Fatalf("expected GetCallGraph to succeed against fake LSP, got error: %v", err)
	}

	count, err := fakeCallGraphMethodCallCount(ctx, client, "custom/callGraph")
	if err != nil {
		t.Fatalf("failed to read fake LSP method call count: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected GetCallGraph to call custom/callGraph exactly once, got %d", count)
	}
}

func TestGetCallGraph_WhenLSPReturnsNull_ReturnsFriendlyMessage(t *testing.T) {
	t.Setenv(callGraphFakeLSPReturnNullEnv, "1")

	client, cleanup := setupCallGraphFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	result, err := GetCallGraph(ctx, client, "Unit1.DoWork", 1)
	if err != nil {
		t.Fatalf("expected GetCallGraph to handle null result without error, got: %v", err)
	}

	const expected = "Call graph not available for the requested file or symbol."
	if result != expected {
		t.Fatalf("unexpected friendly message for null call graph; expected %q, got %q", expected, result)
	}
}

func TestGetCallGraph_WhenLSPReturnsObject_FormatsIndentedJSON(t *testing.T) {
	client, cleanup := setupCallGraphFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	result, err := GetCallGraph(ctx, client, "Unit1.DoWork", 1)
	if err != nil {
		t.Fatalf("expected GetCallGraph to format object result, got error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got unmarshal error: %v\noutput=%s", err, result)
	}

	symbol, ok := payload["symbol"].(string)
	if !ok || symbol != "DoWork" {
		t.Fatalf("expected symbol=DoWork, got %#v", payload["symbol"])
	}

	calls, ok := payload["calls"].([]any)
	if !ok || len(calls) != 2 {
		t.Fatalf("expected exactly two call entries, got %#v", payload["calls"])
	}

	edgesBySymbol := map[string]map[string]any{}
	for _, entry := range calls {
		edge, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("expected call entry to be object, got %#v", entry)
		}
		symbol, _ := edge["symbol"].(string)
		edgesBySymbol[symbol] = edge
	}

	trimEdge, ok := edgesBySymbol["Trim"]
	if !ok {
		t.Fatalf("expected calls to contain Trim edge, got %#v", calls)
	}
	if trimEdge["unit"] != "SysUtils" {
		t.Fatalf("expected Trim edge unit SysUtils, got %#v", trimEdge["unit"])
	}
	if trimEdge["isStub"] != true {
		t.Fatalf("expected Trim edge isStub=true, got %#v", trimEdge["isStub"])
	}

	helperEdge, ok := edgesBySymbol["HelperProc"]
	if !ok {
		t.Fatalf("expected calls to contain HelperProc edge, got %#v", calls)
	}
	if helperEdge["unit"] != "UnitX" {
		t.Fatalf("expected HelperProc edge unit UnitX, got %#v", helperEdge["unit"])
	}
	if helperEdge["isStub"] != false {
		t.Fatalf("expected HelperProc edge isStub=false, got %#v", helperEdge["isStub"])
	}
}

func TestGetCallGraph_RejectsEmptySymbolName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	_, err := GetCallGraph(ctx, nil, "   ", 1)
	if err == nil {
		t.Fatal("expected error when symbolName is empty after trimming")
	}

	if err.Error() != "symbolName must be a non-empty string" {
		t.Fatalf("unexpected error for empty symbolName: %v", err)
	}
}

func setupCallGraphFakeClient(t *testing.T) (*lsp.Client, func()) {
	t.Helper()

	t.Setenv(callGraphFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessCallGraphFakeLSP")
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

func fakeCallGraphMethodCallCount(ctx context.Context, client *lsp.Client, method string) (int, error) {
	params := map[string]string{"method": method}
	var count int
	if err := client.Call(ctx, "custom/testing/methodCallCount", params, &count); err != nil {
		return 0, err
	}
	return count, nil
}

func runCallGraphFakeLSP(stdin *os.File, stdout *os.File) {
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
			result := map[string]any{
				"capabilities": map[string]any{},
			}
			sendCallGraphFakeResponse(writer, msg.ID, result, nil)
		case "initialized":
			// No-op.
		case "custom/callGraph":
			counts[msg.Method]++
			if os.Getenv(callGraphFakeLSPReturnNullEnv) == "1" {
				sendCallGraphFakeRawResponse(writer, msg.ID, json.RawMessage("null"), nil)
				continue
			}
			result := map[string]any{
				"symbol": "DoWork",
				"calls": []map[string]any{
					{"symbol": "Trim", "unit": "SysUtils", "isStub": true},
					{"symbol": "HelperProc", "unit": "UnitX", "isStub": false},
				},
			}
			sendCallGraphFakeResponse(writer, msg.ID, result, nil)
		case "custom/testing/methodCallCount":
			var params struct {
				Method string `json:"method"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			sendCallGraphFakeResponse(writer, msg.ID, counts[params.Method], nil)
		case "shutdown":
			sendCallGraphFakeResponse(writer, msg.ID, nil, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendCallGraphFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32601, Message: "method not found: " + msg.Method})
			}
		}
	}
}

func sendCallGraphFakeResponse(w *os.File, id *lsp.MessageID, result any, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
	}

	if rpcErr == nil {
		if result == nil {
			resp.Result = mustMarshalCallGraphFake(struct{}{})
		} else {
			resp.Result = mustMarshalCallGraphFake(result)
		}
	}

	_ = lsp.WriteMessage(w, resp)
}

func sendCallGraphFakeRawResponse(w *os.File, id *lsp.MessageID, raw json.RawMessage, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
		Result:  raw,
	}
	_ = lsp.WriteMessage(w, resp)
}

func mustMarshalCallGraphFake(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal fake LSP json: %v", err))
	}
	return b
}
