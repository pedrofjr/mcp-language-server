package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestRegisterTools_RegistersCallGraphAlongsideCoreDelphiTools(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	listResp := handleTestMCPRequest(t, svc, mcp.MethodToolsList, map[string]any{}, 2)
	var listResult mcp.ListToolsResult
	decodeTestMCPResult(t, listResp.Result, &listResult)

	toolSet := make(map[string]struct{}, len(listResult.Tools))
	for _, tool := range listResult.Tools {
		toolSet[tool.Name] = struct{}{}
	}

	required := []string{"call_graph", "ast_summary", "dependency_tree", "workspace_symbols"}
	for _, name := range required {
		if _, ok := toolSet[name]; !ok {
			t.Fatalf("expected registerTools() to include %q in tools/list, but it was missing", name)
		}
	}

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "call_graph",
			"arguments": map[string]any{
				"symbolName": "Unit1.DoWork",
				"depth":      1.5,
			},
		},
		5,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal call_graph result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode call_graph result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatal("expected call_graph with fractional depth to return a tool error")
	}

	if !strings.Contains(string(resultBytes), "depth must be an integer") {
		t.Fatalf("expected call_graph error message to mention integer depth, got %s", string(resultBytes))
	}
}

func TestRegisterTools_WorkspaceSymbols_RemainsRegisteredAndCallable(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	listResp := handleTestMCPRequest(t, svc, mcp.MethodToolsList, map[string]any{}, 3)
	var listResult mcp.ListToolsResult
	decodeTestMCPResult(t, listResp.Result, &listResult)

	foundWorkspaceSymbols := false
	for _, tool := range listResult.Tools {
		if tool.Name == "workspace_symbols" {
			foundWorkspaceSymbols = true
			break
		}
	}

	if !foundWorkspaceSymbols {
		t.Fatal("expected workspace_symbols to stay explicitly registered in tools/list")
	}

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "workspace_symbols",
			"arguments": map[string]any{
				"query": 123,
			},
		},
		4,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal call result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode call result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatal("expected workspace_symbols with non-string query to return a tool error")
	}

	if !strings.Contains(string(resultBytes), "query must be a string") {
		t.Fatalf("expected workspace_symbols error message to mention invalid query type, got %s", string(resultBytes))
	}
}

func newRegisteredTestMCPServer(t *testing.T) *mcpServer {
	t.Helper()

	svc := &mcpServer{ctx: context.Background()}
	svc.mcpServer = newMCPServer()

	if err := svc.registerTools(); err != nil {
		t.Fatalf("registerTools() returned error: %v", err)
	}

	return svc
}

func initializeTestMCPServer(t *testing.T, svc *mcpServer) {
	t.Helper()

	_ = handleTestMCPRequest(
		t,
		svc,
		mcp.MethodInitialize,
		map[string]any{
			"protocolVersion": mcp.LATEST_PROTOCOL_VERSION,
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "mcp-language-server-regression-test",
				"version": "0.0.0",
			},
		},
		1,
	)
}

func handleTestMCPRequest(t *testing.T, svc *mcpServer, method mcp.MCPMethod, params map[string]any, id int) mcp.JSONRPCResponse {
	t.Helper()

	request := mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      id,
		Request: mcp.Request{Method: string(method)},
		Params:  params,
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("json.Marshal(%s request) returned error: %v", method, err)
	}

	response := svc.mcpServer.HandleMessage(context.Background(), requestBytes)
	rpcResp, ok := response.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("HandleMessage() for %s returned %T, want mcp.JSONRPCResponse", method, response)
	}

	return rpcResp
}

func decodeTestMCPResult(t *testing.T, result any, out any) {
	t.Helper()

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal(result) returned error: %v", err)
	}

	if err := json.Unmarshal(encoded, out); err != nil {
		t.Fatalf("json.Unmarshal(result) returned error: %v", err)
	}
}
