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

const semanticSearchFakeLSPEnv = "MCP_FAKE_LSP_SEMANTIC_SEARCH"

func TestHelperProcessSemanticSearchFakeLSP(t *testing.T) {
        if os.Getenv(semanticSearchFakeLSPEnv) != "1" {
                return
        }

        runSemanticSearchFakeLSP(os.Stdin, os.Stdout)
        os.Exit(0)
}

func TestGetSemanticSearch_CallsCustomSemanticSearchMethod(t *testing.T) {
        client, cleanup := setupSemanticSearchFakeClient(t)
        defer cleanup()

        ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
        defer cancel()

        if _, err := GetSemanticSearch(ctx, client, "payment", "workspace", "", 20); err != nil {
                t.Fatalf("expected GetSemanticSearch to succeed against fake LSP, got error: %v", err)
        }

        count, err := fakeSemanticSearchMethodCallCount(ctx, client, "custom/semanticSearch")
        if err != nil {
                t.Fatalf("failed to read fake LSP method call count: %v", err)
        }

        if count != 1 {
                t.Fatalf("expected GetSemanticSearch to call custom/semanticSearch exactly once, got %d", count)
        }
}

func TestGetSemanticSearch_WhenLSPReturnsObject_FormatsIndentedJSONAndPreservesFields(t *testing.T) {
        client, cleanup := setupSemanticSearchFakeClient(t)
        defer cleanup()

        ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
        defer cancel()

        result, err := GetSemanticSearch(ctx, client, "payment", "workspace", "", 20)
        if err != nil {
                t.Fatalf("expected GetSemanticSearch to format object result, got error: %v", err)
        }

        var payload map[string]any
        if err := json.Unmarshal([]byte(result), &payload); err != nil {
                t.Fatalf("expected valid JSON output, got unmarshal error: %v\noutput=%s", err, result)
        }

        results, ok := payload["results"].([]any)
        if !ok || len(results) != 1 {
                t.Fatalf("expected one semantic search result, got %#v", payload["results"])
        }

        first, ok := results[0].(map[string]any)
        if !ok {
                t.Fatalf("expected first result to be an object, got %#v", results[0])
        }

        if first["symbol"] != "Billing.ProcessPayment" {
                t.Fatalf("expected symbol Billing.ProcessPayment, got %#v", first["symbol"])
        }
        if first["unit"] != "Billing" {
                t.Fatalf("expected unit Billing, got %#v", first["unit"])
        }
        if first["matchReason"] != "name + comment" {
                t.Fatalf("expected matchReason preserved, got %#v", first["matchReason"])
        }

        location, ok := first["location"].(map[string]any)
        if !ok {
                t.Fatalf("expected location object, got %#v", first["location"])
        }
        if location["uri"] != "file:///workspace/billing.pas" {
                t.Fatalf("expected location.uri preserved, got %#v", location["uri"])
        }
}

func setupSemanticSearchFakeClient(t *testing.T) (*lsp.Client, func()) {
        t.Helper()

        t.Setenv(semanticSearchFakeLSPEnv, "1")

        execPath, err := os.Executable()
        if err != nil {
                t.Fatalf("failed to resolve test binary path: %v", err)
        }

        client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessSemanticSearchFakeLSP")
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

func fakeSemanticSearchMethodCallCount(ctx context.Context, client *lsp.Client, method string) (int, error) {
        params := map[string]string{"method": method}
        var count int
        if err := client.Call(ctx, "custom/testing/methodCallCount", params, &count); err != nil {
                return 0, err
        }
        return count, nil
}

func runSemanticSearchFakeLSP(stdin *os.File, stdout *os.File) {
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
                        sendSemanticSearchFakeResponse(writer, msg.ID, result, nil)
                case "initialized":
                        // No-op.
                case "custom/semanticSearch":
                        counts[msg.Method]++
                        result := map[string]any{
                                "results": []map[string]any{
                                        {
                                                "symbol":      "Billing.ProcessPayment",
                                                "unit":        "Billing",
                                                "score":       0.87,
                                                "matchReason": "name + comment",
                                                "location": map[string]any{
                                                        "uri": "file:///workspace/billing.pas",
                                                        "range": map[string]any{
                                                                "start": map[string]any{"line": 10.0, "character": 2.0},
                                                                "end":   map[string]any{"line": 10.0, "character": 20.0},
                                                        },
                                                },
                                        },
                                },
                        }
                        sendSemanticSearchFakeResponse(writer, msg.ID, result, nil)
                case "custom/testing/methodCallCount":
                        var params struct {
                                Method string `json:"method"`
                        }
                        _ = json.Unmarshal(msg.Params, &params)
                        sendSemanticSearchFakeResponse(writer, msg.ID, counts[params.Method], nil)
                case "shutdown":
                        sendSemanticSearchFakeResponse(writer, msg.ID, nil, nil)
                case "exit":
                        return
                default:
                        if msg.ID != nil && msg.ID.Value != nil {
                                sendSemanticSearchFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32601, Message: "method not found: " + msg.Method})
                        }
                }
        }
}

func sendSemanticSearchFakeResponse(w *os.File, id *lsp.MessageID, result any, rpcErr *lsp.ResponseError) {
        resp := &lsp.Message{
                JSONRPC: "2.0",
                ID:      id,
                Error:   rpcErr,
        }

        if rpcErr == nil {
                if result == nil {
                        resp.Result = mustMarshalSemanticSearchFake(struct{}{})
                } else {
                        resp.Result = mustMarshalSemanticSearchFake(result)
                }
        }

        _ = lsp.WriteMessage(w, resp)
}

func mustMarshalSemanticSearchFake(v any) json.RawMessage {
        b, err := json.Marshal(v)
        if err != nil {
                panic(fmt.Sprintf("failed to marshal fake LSP json: %v", err))
        }
        return b
}
