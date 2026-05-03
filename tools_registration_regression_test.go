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

	required := []string{"call_graph", "ast_summary", "dependency_tree", "workspace_symbols", "semantic_search"}
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

func TestRegisterTools_SemanticSearch_RegisteredAndValidatesParams(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	listResp := handleTestMCPRequest(t, svc, mcp.MethodToolsList, map[string]any{}, 30)
	var listResult mcp.ListToolsResult
	decodeTestMCPResult(t, listResp.Result, &listResult)

	foundSemanticSearch := false
	for _, tool := range listResult.Tools {
		if tool.Name == "semantic_search" {
			foundSemanticSearch = true
			break
		}
	}
	if !foundSemanticSearch {
		t.Fatal("expected semantic_search to be explicitly registered in tools/list")
	}

	cases := []struct {
		name           string
		args           map[string]any
		expectedSubstr string
	}{
		{
			name:           "query must be string",
			args:           map[string]any{"query": 123},
			expectedSubstr: "query must be a non-empty string",
		},
		{
			name:           "query must be non-empty",
			args:           map[string]any{"query": "   "},
			expectedSubstr: "query must be a non-empty string",
		},
		{
			name:           "scope enum validation",
			args:           map[string]any{"query": "payment", "scope": "project"},
			expectedSubstr: "scope must be 'workspace' or 'file'",
		},
		{
			name:           "scope file requires uri",
			args:           map[string]any{"query": "payment", "scope": "file"},
			expectedSubstr: "uri is required when scope='file'",
		},
		{
			name:           "limit must be positive integer",
			args:           map[string]any{"query": "payment", "limit": 0},
			expectedSubstr: "limit must be a positive integer",
		},
	}

	for i, tc := range cases {
		callResp := handleTestMCPRequest(
			t,
			svc,
			mcp.MethodToolsCall,
			map[string]any{
				"name":      "semantic_search",
				"arguments": tc.args,
			},
			31+i,
		)

		resultBytes, err := json.Marshal(callResp.Result)
		if err != nil {
			t.Fatalf("%s: failed to marshal semantic_search result: %v", tc.name, err)
		}

		var callResult map[string]any
		if err := json.Unmarshal(resultBytes, &callResult); err != nil {
			t.Fatalf("%s: failed to decode semantic_search result map: %v", tc.name, err)
		}

		isError, _ := callResult["isError"].(bool)
		if !isError {
			t.Fatalf("%s: expected semantic_search to return tool error for invalid params", tc.name)
		}

		if !strings.Contains(string(resultBytes), tc.expectedSubstr) {
			t.Fatalf("%s: expected semantic_search error to contain %q, got %s", tc.name, tc.expectedSubstr, string(resultBytes))
		}
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
