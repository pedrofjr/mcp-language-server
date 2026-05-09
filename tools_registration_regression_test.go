package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

func TestRegisterTools_GetSymbolsOverview_IsRegisteredAndRejectsNonStringQuery(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	listResp := handleTestMCPRequest(t, svc, mcp.MethodToolsList, map[string]any{}, 4)
	var listResult mcp.ListToolsResult
	decodeTestMCPResult(t, listResp.Result, &listResult)

	toolSet := make(map[string]struct{}, len(listResult.Tools))
	for _, tool := range listResult.Tools {
		toolSet[tool.Name] = struct{}{}
	}

	if _, ok := toolSet["get_symbols_overview"]; !ok {
		t.Fatal("expected get_symbols_overview to be explicitly registered in tools/list")
	}

	if _, ok := toolSet["workspace_symbols"]; !ok {
		t.Fatal("expected workspace_symbols to remain registered alongside get_symbols_overview")
	}

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "get_symbols_overview",
			"arguments": map[string]any{
				"query": 123,
			},
		},
		5,
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
		t.Fatal("expected get_symbols_overview with non-string query to return a tool error")
	}

	if !strings.Contains(string(resultBytes), "query must be a string") {
		t.Fatalf("expected get_symbols_overview error message to mention invalid query type, got %s", string(resultBytes))
	}
}

func TestRegisterTools_OnboardingTools_RejectNonStringContextArgument(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	tmpProject := t.TempDir()

	cases := []struct {
		toolName string
		id       int
	}{
		{toolName: "onboarding", id: 25},
		{toolName: "check_onboarding_performed", id: 26},
	}

	for _, tc := range cases {
		callResp := handleTestMCPRequest(
			t,
			svc,
			mcp.MethodToolsCall,
			map[string]any{
				"name": tc.toolName,
				"arguments": map[string]any{
					"projectPath": tmpProject,
					"context":     123,
				},
			},
			tc.id,
		)

		resultBytes, err := json.Marshal(callResp.Result)
		if err != nil {
			t.Fatalf("%s: failed to marshal result: %v", tc.toolName, err)
		}

		var callResult map[string]any
		if err := json.Unmarshal(resultBytes, &callResult); err != nil {
			t.Fatalf("%s: failed to decode result map: %v", tc.toolName, err)
		}

		isError, _ := callResult["isError"].(bool)
		if !isError {
			t.Fatalf("%s: expected tool error for non-string context", tc.toolName)
		}

		if !strings.Contains(string(resultBytes), "context must be a string") {
			t.Fatalf("%s: expected validation message for context type, got %s", tc.toolName, string(resultBytes))
		}
	}
}

func TestRegisterTools_OnboardingTools_AcceptNullAndWhitespaceContextArgument(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	tmpProject := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpProject, "Unit1.pas"), []byte("unit Unit1; interface implementation end."), 0o644)
	if err != nil {
		t.Fatalf("failed to create minimal Delphi fixture: %v", err)
	}

	cases := []struct {
		name    string
		tool    string
		context any
		id      int
	}{
		{name: "onboarding with null context", tool: "onboarding", context: nil, id: 27},
		{name: "onboarding with whitespace context", tool: "onboarding", context: "   ", id: 28},
		{name: "check_onboarding_performed with null context", tool: "check_onboarding_performed", context: nil, id: 29},
		{name: "check_onboarding_performed with whitespace context", tool: "check_onboarding_performed", context: "\t  ", id: 30},
	}

	for _, tc := range cases {
		callResp := handleTestMCPRequest(
			t,
			svc,
			mcp.MethodToolsCall,
			map[string]any{
				"name": tc.tool,
				"arguments": map[string]any{
					"projectPath": tmpProject,
					"context":     tc.context,
				},
			},
			tc.id,
		)

		resultBytes, marshalErr := json.Marshal(callResp.Result)
		if marshalErr != nil {
			t.Fatalf("%s: failed to marshal result: %v", tc.name, marshalErr)
		}

		var callResult map[string]any
		if unmarshalErr := json.Unmarshal(resultBytes, &callResult); unmarshalErr != nil {
			t.Fatalf("%s: failed to decode result map: %v", tc.name, unmarshalErr)
		}

		isError, _ := callResult["isError"].(bool)
		if isError {
			t.Fatalf("%s: expected success for null/whitespace context, got %s", tc.name, string(resultBytes))
		}
	}
}

func TestRegisterTools_CheckOnboardingPerformed_NotReadyActionableMessage(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	tmpProject := t.TempDir()

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "check_onboarding_performed",
			"arguments": map[string]any{
				"projectPath": tmpProject,
			},
		},
		31,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal check_onboarding_performed result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode check_onboarding_performed result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if isError {
		t.Fatalf("expected check_onboarding_performed not-ready path to return non-error MCP result (err=nil and isError=false), got %s", string(resultBytes))
	}

	contentRaw, ok := callResult["content"].([]any)
	if !ok || len(contentRaw) == 0 {
		t.Fatalf("expected check_onboarding_performed success payload to include content array, got %s", string(resultBytes))
	}

	firstContent, ok := contentRaw[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first content entry to be an object, got %s", string(resultBytes))
	}

	message, _ := firstContent["text"].(string)
	if message == "" {
		t.Fatalf("expected check_onboarding_performed payload to include human-readable text, got %s", string(resultBytes))
	}

	legacyPrefix := "Onboarding ainda nao foi executado para este projeto"
	if !strings.HasPrefix(message, legacyPrefix) {
		t.Fatalf("expected message to preserve legacy prefix %q, got %q", legacyPrefix, message)
	}

	lowerMessage := strings.ToLower(message)
	if !strings.Contains(lowerMessage, "onboarding") || !strings.Contains(lowerMessage, "novamente") {
		t.Fatalf("expected actionable guidance with explicit next step (execute onboarding/check novamente), got %q", message)
	}

	if !strings.Contains(lowerMessage, "se nao houver") {
		t.Fatalf("expected readiness hint in cautious language (e.g., 'se nao houver ...'), got %q", message)
	}

	if !strings.Contains(lowerMessage, ".pas") || !strings.Contains(lowerMessage, ".dpr") || !strings.Contains(lowerMessage, ".dpk") {
		t.Fatalf("expected readiness hint to mention Delphi source extensions (.pas/.dpr/.dpk), got %q", message)
	}
}

func TestRegisterTools_CheckOnboardingPerformed_PerformedSuggestsGetSymbolsOverview_DefaultContext(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	projectPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectPath, "Unit1.pas"), []byte("unit Unit1; interface implementation end."), 0o644); err != nil {
		t.Fatalf("failed to create onboarding fixture: %v", err)
	}

	onboardingResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "onboarding",
			"arguments": map[string]any{
				"projectPath": projectPath,
			},
		},
		401,
	)

	onboardingResultBytes, err := json.Marshal(onboardingResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal onboarding result: %v", err)
	}

	var onboardingResult map[string]any
	if err := json.Unmarshal(onboardingResultBytes, &onboardingResult); err != nil {
		t.Fatalf("failed to decode onboarding result map: %v", err)
	}

	onboardingError, _ := onboardingResult["isError"].(bool)
	if onboardingError {
		t.Fatalf("expected onboarding fixture call to succeed, got %s", string(onboardingResultBytes))
	}

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "check_onboarding_performed",
			"arguments": map[string]any{
				"projectPath": projectPath,
			},
		},
		402,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal check_onboarding_performed result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode check_onboarding_performed result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if isError {
		t.Fatalf("expected check_onboarding_performed performed path to return success, got %s", string(resultBytes))
	}

	contentRaw, ok := callResult["content"].([]any)
	if !ok || len(contentRaw) == 0 {
		t.Fatalf("expected check_onboarding_performed payload to include content array, got %s", string(resultBytes))
	}

	firstContent, ok := contentRaw[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first content entry to be an object, got %s", string(resultBytes))
	}

	message, _ := firstContent["text"].(string)
	if message == "" {
		t.Fatalf("expected check_onboarding_performed payload to include human-readable text, got %s", string(resultBytes))
	}

	legacyPrefix := "Onboarding executado em:"
	if !strings.HasPrefix(message, legacyPrefix) {
		t.Fatalf("expected performed message to preserve legacy prefix %q, got %q", legacyPrefix, message)
	}

	if !strings.Contains(strings.ToLower(message), "get_symbols_overview") {
		t.Fatalf("expected performed message to suggest get_symbols_overview as next step, got %q", message)
	}

	if strings.Contains(strings.ToLower(message), "definition") {
		t.Fatalf("expected performed message to avoid definition extrapolation, got %q", message)
	}

	if strings.Contains(strings.ToLower(message), "references") {
		t.Fatalf("expected performed message to avoid references extrapolation, got %q", message)
	}
}

func TestRegisterTools_CheckOnboardingPerformed_PerformedSuggestsGetSymbolsOverview_ExplicitContext(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	projectPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectPath, "Unit1.pas"), []byte("unit Unit1; interface implementation end."), 0o644); err != nil {
		t.Fatalf("failed to create onboarding fixture: %v", err)
	}

	contextKey := "manual-flow"
	onboardingResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "onboarding",
			"arguments": map[string]any{
				"projectPath": projectPath,
				"context":     contextKey,
			},
		},
		403,
	)

	onboardingResultBytes, err := json.Marshal(onboardingResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal onboarding result: %v", err)
	}

	var onboardingResult map[string]any
	if err := json.Unmarshal(onboardingResultBytes, &onboardingResult); err != nil {
		t.Fatalf("failed to decode onboarding result map: %v", err)
	}

	onboardingError, _ := onboardingResult["isError"].(bool)
	if onboardingError {
		t.Fatalf("expected onboarding fixture call to succeed, got %s", string(onboardingResultBytes))
	}

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "check_onboarding_performed",
			"arguments": map[string]any{
				"projectPath": projectPath,
				"context":     contextKey,
			},
		},
		404,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal check_onboarding_performed result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode check_onboarding_performed result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if isError {
		t.Fatalf("expected check_onboarding_performed performed path to return success, got %s", string(resultBytes))
	}

	contentRaw, ok := callResult["content"].([]any)
	if !ok || len(contentRaw) == 0 {
		t.Fatalf("expected check_onboarding_performed payload to include content array, got %s", string(resultBytes))
	}

	firstContent, ok := contentRaw[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first content entry to be an object, got %s", string(resultBytes))
	}

	message, _ := firstContent["text"].(string)
	if message == "" {
		t.Fatalf("expected check_onboarding_performed payload to include human-readable text, got %s", string(resultBytes))
	}

	legacyPrefix := "Onboarding executado em:"
	if !strings.HasPrefix(message, legacyPrefix) {
		t.Fatalf("expected performed message to preserve legacy prefix %q, got %q", legacyPrefix, message)
	}

	if !strings.Contains(message, "(contexto: "+contextKey+")") {
		t.Fatalf("expected explicit context suffix in performed message, got %q", message)
	}

	if !strings.Contains(strings.ToLower(message), "get_symbols_overview") {
		t.Fatalf("expected performed message to suggest get_symbols_overview as next step, got %q", message)
	}

	if strings.Contains(strings.ToLower(message), "definition") {
		t.Fatalf("expected performed message to avoid definition extrapolation, got %q", message)
	}

	if strings.Contains(strings.ToLower(message), "references") {
		t.Fatalf("expected performed message to avoid references extrapolation, got %q", message)
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

func TestRegisterTools_DependencyTree_RegisteredAndRejectsInvalidDirection(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	listResp := handleTestMCPRequest(t, svc, mcp.MethodToolsList, map[string]any{}, 40)
	var listResult mcp.ListToolsResult
	decodeTestMCPResult(t, listResp.Result, &listResult)

	foundDependencyTree := false
	for _, tool := range listResult.Tools {
		if tool.Name == "dependency_tree" {
			foundDependencyTree = true
			break
		}
	}
	if !foundDependencyTree {
		t.Fatal("expected dependency_tree to be explicitly registered in tools/list")
	}

	cases := []struct {
		name      string
		direction any
	}{
		{name: "invalid enum value", direction: "sideways"},
		{name: "non-string direction", direction: 123},
	}

	for i, tc := range cases {
		request := mcp.JSONRPCRequest{
			JSONRPC: mcp.JSONRPC_VERSION,
			ID:      41 + i,
			Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
			Params: map[string]any{
				"name": "dependency_tree",
				"arguments": map[string]any{
					"uri":       "file:///tmp/Unit1.pas",
					"direction": tc.direction,
				},
			},
		}

		requestBytes, err := json.Marshal(request)
		if err != nil {
			t.Fatalf("%s: failed to marshal request: %v", tc.name, err)
		}

		rawResponse := svc.mcpServer.HandleMessage(context.Background(), requestBytes)

		var payload string
		switch response := rawResponse.(type) {
		case mcp.JSONRPCResponse:
			resultBytes, err := json.Marshal(response.Result)
			if err != nil {
				t.Fatalf("%s: failed to marshal dependency_tree result: %v", tc.name, err)
			}
			payload = string(resultBytes)
		case mcp.JSONRPCError:
			errorBytes, err := json.Marshal(response)
			if err != nil {
				t.Fatalf("%s: failed to marshal dependency_tree JSONRPCError: %v", tc.name, err)
			}
			payload = string(errorBytes)
		default:
			t.Fatalf("%s: HandleMessage() returned %T, want JSONRPCResponse or JSONRPCError", tc.name, rawResponse)
		}

		if !strings.Contains(payload, "direction") {
			t.Fatalf("%s: expected dependency_tree failure payload to mention direction, got %s", tc.name, payload)
		}
		if !strings.Contains(payload, "imports") || !strings.Contains(payload, "importedBy") {
			t.Fatalf("%s: expected dependency_tree failure payload to mention allowed direction values, got %s", tc.name, payload)
		}
	}
}

func TestRegisterTools_GraphNeighbors_RegisteredAndValidatesParams(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	listResp := handleTestMCPRequest(t, svc, mcp.MethodToolsList, map[string]any{}, 50)
	var listResult mcp.ListToolsResult
	decodeTestMCPResult(t, listResp.Result, &listResult)

	foundGraphNeighbors := false
	for _, tool := range listResult.Tools {
		if tool.Name == "graph_neighbors" {
			foundGraphNeighbors = true
			break
		}
	}
	if !foundGraphNeighbors {
		t.Fatal("expected graph_neighbors to be explicitly registered in tools/list")
	}

	cases := []struct {
		name           string
		args           map[string]any
		expectedSubstr string
	}{
		{
			name:           "uri must be string",
			args:           map[string]any{"uri": 123, "relationType": "uses_unit", "direction": "imports"},
			expectedSubstr: "uri must be a non-empty string",
		},
		{
			name:           "direction enum validation",
			args:           map[string]any{"uri": "file:///tmp/Unit1.pas", "relationType": "uses_unit", "direction": "sideways"},
			expectedSubstr: "direction must be 'imports' or 'importedBy'",
		},
		{
			name:           "relationType enum validation",
			args:           map[string]any{"uri": "file:///tmp/Unit1.pas", "relationType": "calls", "direction": "imports"},
			expectedSubstr: "relationType must be 'uses_unit'",
		},
	}

	for i, tc := range cases {
		callResp := handleTestMCPRequest(
			t,
			svc,
			mcp.MethodToolsCall,
			map[string]any{
				"name":      "graph_neighbors",
				"arguments": tc.args,
			},
			51+i,
		)

		resultBytes, err := json.Marshal(callResp.Result)
		if err != nil {
			t.Fatalf("%s: failed to marshal graph_neighbors result: %v", tc.name, err)
		}

		var callResult map[string]any
		if err := json.Unmarshal(resultBytes, &callResult); err != nil {
			t.Fatalf("%s: failed to decode graph_neighbors result map: %v", tc.name, err)
		}

		isError, _ := callResult["isError"].(bool)
		if !isError {
			t.Fatalf("%s: expected graph_neighbors to return tool error for invalid params", tc.name)
		}

		if !strings.Contains(string(resultBytes), tc.expectedSubstr) {
			t.Fatalf("%s: expected graph_neighbors error to contain %q, got %s", tc.name, tc.expectedSubstr, string(resultBytes))
		}
	}
}

func TestRegisterTools_GraphNode_RegisteredAndValidatesParams(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	listResp := handleTestMCPRequest(t, svc, mcp.MethodToolsList, map[string]any{}, 60)
	var listResult mcp.ListToolsResult
	decodeTestMCPResult(t, listResp.Result, &listResult)

	foundGraphNode := false
	for _, tool := range listResult.Tools {
		if tool.Name == "graph_node" {
			foundGraphNode = true
			break
		}
	}
	if !foundGraphNode {
		t.Fatal("expected graph_node to be explicitly registered in tools/list")
	}

	cases := []struct {
		name           string
		args           map[string]any
		expectedSubstr string
	}{
		{
			name:           "uri must be string",
			args:           map[string]any{"uri": 123, "relationType": "uses_unit"},
			expectedSubstr: "uri must be a non-empty string",
		},
		{
			name:           "uri must be non-empty",
			args:           map[string]any{"uri": "   ", "relationType": "uses_unit"},
			expectedSubstr: "uri must be a non-empty string",
		},
		{
			name:           "relationType enum validation",
			args:           map[string]any{"uri": "file:///tmp/Unit1.pas", "relationType": "calls"},
			expectedSubstr: "relationType must be 'uses_unit'",
		},
		{
			name:           "relationType must be string when provided",
			args:           map[string]any{"uri": "file:///tmp/Unit1.pas", "relationType": 42},
			expectedSubstr: "relationType must be 'uses_unit'",
		},
	}

	for i, tc := range cases {
		callResp := handleTestMCPRequest(
			t,
			svc,
			mcp.MethodToolsCall,
			map[string]any{
				"name":      "graph_node",
				"arguments": tc.args,
			},
			61+i,
		)

		resultBytes, err := json.Marshal(callResp.Result)
		if err != nil {
			t.Fatalf("%s: failed to marshal graph_node result: %v", tc.name, err)
		}

		var callResult map[string]any
		if err := json.Unmarshal(resultBytes, &callResult); err != nil {
			t.Fatalf("%s: failed to decode graph_node result map: %v", tc.name, err)
		}

		isError, _ := callResult["isError"].(bool)
		if !isError {
			t.Fatalf("%s: expected graph_node to return tool error for invalid params", tc.name)
		}

		if !strings.Contains(string(resultBytes), tc.expectedSubstr) {
			t.Fatalf("%s: expected graph_node error to contain %q, got %s", tc.name, tc.expectedSubstr, string(resultBytes))
		}
	}
}

func TestRegisterTools_GraphQuery_RegisteredAndValidatesParams(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	listResp := handleTestMCPRequest(t, svc, mcp.MethodToolsList, map[string]any{}, 70)
	var listResult mcp.ListToolsResult
	decodeTestMCPResult(t, listResp.Result, &listResult)

	foundGraphQuery := false
	for _, tool := range listResult.Tools {
		if tool.Name == "graph_query" {
			foundGraphQuery = true
			break
		}
	}
	if !foundGraphQuery {
		t.Fatal("expected graph_query to be explicitly registered in tools/list")
	}

	cases := []struct {
		name           string
		args           map[string]any
		expectedSubstr string
	}{
		{
			name:           "uri must be string",
			args:           map[string]any{"uri": 123, "relationType": "uses_unit", "direction": "both", "depth": 1},
			expectedSubstr: "uri must be a non-empty string",
		},
		{
			name:           "relationType enum validation",
			args:           map[string]any{"uri": "file:///tmp/Unit1.pas", "relationType": "calls", "direction": "both", "depth": 1},
			expectedSubstr: "relationType must be 'uses_unit'",
		},
		{
			name:           "direction enum validation",
			args:           map[string]any{"uri": "file:///tmp/Unit1.pas", "relationType": "uses_unit", "direction": "sideways", "depth": 1},
			expectedSubstr: "direction must be 'imports', 'importedBy' or 'both'",
		},
		{
			name:           "depth must be integer",
			args:           map[string]any{"uri": "file:///tmp/Unit1.pas", "relationType": "uses_unit", "direction": "both", "depth": 1.5},
			expectedSubstr: "depth must be an integer",
		},
		{
			name:           "depth cannot be negative",
			args:           map[string]any{"uri": "file:///tmp/Unit1.pas", "relationType": "uses_unit", "direction": "both", "depth": -1},
			expectedSubstr: "depth must be greater than or equal to 0",
		},
	}

	for i, tc := range cases {
		callResp := handleTestMCPRequest(
			t,
			svc,
			mcp.MethodToolsCall,
			map[string]any{
				"name":      "graph_query",
				"arguments": tc.args,
			},
			71+i,
		)

		resultBytes, err := json.Marshal(callResp.Result)
		if err != nil {
			t.Fatalf("%s: failed to marshal graph_query result: %v", tc.name, err)
		}

		var callResult map[string]any
		if err := json.Unmarshal(resultBytes, &callResult); err != nil {
			t.Fatalf("%s: failed to decode graph_query result map: %v", tc.name, err)
		}

		isError, _ := callResult["isError"].(bool)
		if !isError {
			t.Fatalf("%s: expected graph_query to return tool error for invalid params", tc.name)
		}

		if !strings.Contains(string(resultBytes), tc.expectedSubstr) {
			t.Fatalf("%s: expected graph_query error to contain %q, got %s", tc.name, tc.expectedSubstr, string(resultBytes))
		}
	}
}

func TestRegisterTools_CodeActions_RegisteredAndValidatesParams(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	listResp := handleTestMCPRequest(t, svc, mcp.MethodToolsList, map[string]any{}, 80)
	var listResult mcp.ListToolsResult
	decodeTestMCPResult(t, listResp.Result, &listResult)

	foundCodeActions := false
	for _, tool := range listResult.Tools {
		if tool.Name == "code_actions" {
			foundCodeActions = true
			break
		}
	}
	if !foundCodeActions {
		t.Fatal("expected code_actions to be explicitly registered in tools/list")
	}

	cases := []struct {
		name           string
		args           map[string]any
		expectedSubstr string
	}{
		{
			name:           "filePath must be string",
			args:           map[string]any{"filePath": 123, "line": 7, "column": 13},
			expectedSubstr: "filePath must be a string",
		},
		{
			name:           "line must be number",
			args:           map[string]any{"filePath": "C:/tmp/sample.pas", "line": "7", "column": 13},
			expectedSubstr: "line must be a number",
		},
		{
			name:           "column must be number",
			args:           map[string]any{"filePath": "C:/tmp/sample.pas", "line": 7, "column": "13"},
			expectedSubstr: "column must be a number",
		},
		{
			name:           "only must be array of strings",
			args:           map[string]any{"filePath": "C:/tmp/sample.pas", "line": 7, "column": 13, "only": "quickfix"},
			expectedSubstr: "only must be an array of strings",
		},
		{
			name:           "includeDiagnostics must be bool",
			args:           map[string]any{"filePath": "C:/tmp/sample.pas", "line": 7, "column": 13, "includeDiagnostics": "true"},
			expectedSubstr: "includeDiagnostics must be a boolean",
		},
	}

	for i, tc := range cases {
		callResp := handleTestMCPRequest(
			t,
			svc,
			mcp.MethodToolsCall,
			map[string]any{
				"name":      "code_actions",
				"arguments": tc.args,
			},
			81+i,
		)

		resultBytes, err := json.Marshal(callResp.Result)
		if err != nil {
			t.Fatalf("%s: failed to marshal code_actions result: %v", tc.name, err)
		}

		var callResult map[string]any
		if err := json.Unmarshal(resultBytes, &callResult); err != nil {
			t.Fatalf("%s: failed to decode code_actions result map: %v", tc.name, err)
		}

		isError, _ := callResult["isError"].(bool)
		if !isError {
			t.Fatalf("%s: expected code_actions to return tool error for invalid params", tc.name)
		}

		if !strings.Contains(string(resultBytes), tc.expectedSubstr) {
			t.Fatalf("%s: expected code_actions error to contain %q, got %s", tc.name, tc.expectedSubstr, string(resultBytes))
		}
	}
}

func TestRegisterTools_ActivateProject_InvalidDirReturnsToolError(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "activate_project",
			"arguments": map[string]any{
				"dir": "C:/__invalid__/__missing__/project",
			},
		},
		90,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal activate_project result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode activate_project result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatalf("expected activate_project invalid dir to return isError=true, got payload: %s", string(resultBytes))
	}

	if !strings.Contains(strings.ToLower(string(resultBytes)), "dir") {
		t.Fatalf("expected activate_project invalid dir error to mention 'dir', got %s", string(resultBytes))
	}
}

func TestRegisterTools_Diagnostics_ContextLinesBooleanDoesNotTriggerTypeError(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "diagnostics",
			"arguments": map[string]any{
				"filePath":        "C:/tmp/does-not-exist.pas",
				"contextLines":    true,
				"showLineNumbers": true,
			},
		},
		91,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal diagnostics result: %v", err)
	}

	payloadLower := strings.ToLower(string(resultBytes))
	// Validate that contextLines=true does NOT trigger an argument validation error
	// Errors from backend unavailability or file not found are acceptable.
	if strings.Contains(payloadLower, "contextlines must") {
		t.Fatalf("expected diagnostics with contextLines=true to avoid argument validation error, got %s", string(resultBytes))
	}
}

func TestRegisterTools_Diagnostics_UninitialiizedLspClientReturnsErrorFlag(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	// Deliberately skip initializeTestMCPServer to leave lspClient uninitialized

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "diagnostics",
			"arguments": map[string]any{
				"filePath":        "C:/tmp/test.pas",
				"contextLines":    false,
				"showLineNumbers": true,
			},
		},
		91,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal diagnostics result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode diagnostics result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatalf("expected diagnostics with uninitialized lspClient to return isError=true, got payload %s", string(resultBytes))
	}
}

func TestRegisterTools_NodeAtPosition_InvalidLineReturnsUsefulToolError(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "get_node_at_position",
			"arguments": map[string]any{
				"filePath": "C:/tmp/sample.pas",
				"line":     "abc",
				"column":   "10",
			},
		},
		92,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal get_node_at_position invalid-line result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode get_node_at_position invalid-line result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatalf("expected get_node_at_position with non-numeric line to return isError=true, got %s", string(resultBytes))
	}

	payloadLower := strings.ToLower(string(resultBytes))
	if !strings.Contains(payloadLower, "line") {
		t.Fatalf("expected get_node_at_position invalid-line error to mention line, got %s", string(resultBytes))
	}
}

func TestRegisterTools_NodeAtPosition_AcceptsNumericLineAndColumn(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "get_node_at_position",
			"arguments": map[string]any{
				"filePath": "C:/tmp/sample.pas",
				"line":     7.0,
				"column":   12.0,
			},
		},
		93,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal get_node_at_position numeric result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode get_node_at_position numeric result map: %v", err)
	}

	payloadLower := strings.ToLower(string(resultBytes))
	// Validate that parsing did not fail due to type mismatch
	if strings.Contains(payloadLower, "line") && strings.Contains(payloadLower, "type") {
		t.Fatalf("expected get_node_at_position to accept numeric line/column without type error, got %s", string(resultBytes))
	}
	// Note: execution error (e.g., file not found) is acceptable here
}

func TestRegisterTools_NodeAtPosition_InternalExecutionErrorReturnsErrorFlag(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "get_node_at_position",
			"arguments": map[string]any{
				"filePath": "C:/__nonexistent__/__missing__/file.pas",
				"line":     7.0,
				"column":   12.0,
			},
		},
		93,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal get_node_at_position nonexistent-file result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode get_node_at_position nonexistent-file result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatalf("expected get_node_at_position with nonexistent file to return isError=true, got payload %s", string(resultBytes))
	}
}

func TestRegisterTools_MemoryAliases_ArePresentInToolsList(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	listResp := handleTestMCPRequest(t, svc, mcp.MethodToolsList, map[string]any{}, 94)
	var listResult mcp.ListToolsResult
	decodeTestMCPResult(t, listResp.Result, &listResult)

	toolSet := make(map[string]struct{}, len(listResult.Tools))
	for _, tool := range listResult.Tools {
		toolSet[tool.Name] = struct{}{}
	}

	requiredAliases := []string{"write_memory", "read_memory", "list_memories", "edit_memory", "delete_memory"}
	for _, alias := range requiredAliases {
		if _, ok := toolSet[alias]; !ok {
			t.Fatalf("expected tools/list to include memory alias %q", alias)
		}
	}
}

func TestRegisterTools_RunQuery_IsRegisteredInToolsList(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	listResp := handleTestMCPRequest(t, svc, mcp.MethodToolsList, map[string]any{}, 95)
	var listResult mcp.ListToolsResult
	decodeTestMCPResult(t, listResp.Result, &listResult)

	foundRunQuery := false
	for _, tool := range listResult.Tools {
		if tool.Name == "run_query" {
			foundRunQuery = true
			break
		}
	}

	if !foundRunQuery {
		t.Fatal("expected run_query to be explicitly registered in tools/list")
	}
}

func TestRegisterTools_RunQuery_RequiresQueryOrNodeType(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name":      "run_query",
			"arguments": map[string]any{},
		},
		96,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal run_query empty-args result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode run_query empty-args result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatalf("expected run_query with no query/node_type to return tool error, got %s", string(resultBytes))
	}

	if !strings.Contains(string(resultBytes), "query or node_type") {
		t.Fatalf("expected run_query validation error to mention required query or node_type, got %s", string(resultBytes))
	}
}

func TestRegisterTools_RunQuery_InvalidLimitReturnsError(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query": "procedure_declaration",
				"limit": 0,
			},
		},
		97,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal run_query invalid-limit result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode run_query invalid-limit result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatalf("expected run_query with invalid limit to return tool error, got %s", string(resultBytes))
	}

	if !strings.Contains(string(resultBytes), "limit") {
		t.Fatalf("expected run_query invalid-limit error to mention limit constraint, got %s", string(resultBytes))
	}
}

func TestRegisterTools_RunQuery_MinimalHappyPathReturnsExpectedShape(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query": "procedure_declaration",
				"limit": 1,
			},
		},
		98,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal run_query happy-path result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode run_query happy-path result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if isError {
		t.Fatalf("expected run_query minimal happy path to succeed, got error payload %s", string(resultBytes))
	}

	contentRaw, ok := callResult["content"].([]any)
	if !ok || len(contentRaw) == 0 {
		t.Fatalf("expected run_query success payload to include content array, got %s", string(resultBytes))
	}

	firstContent, ok := contentRaw[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first content entry to be an object, got %s", string(resultBytes))
	}

	textPayload, _ := firstContent["text"].(string)
	if textPayload == "" {
		t.Fatalf("expected run_query success payload to include text JSON body, got %s", string(resultBytes))
	}

	var shaped map[string]any
	if err := json.Unmarshal([]byte(textPayload), &shaped); err != nil {
		t.Fatalf("expected run_query text payload to be valid JSON, decode failed: %v; payload=%s", err, textPayload)
	}

	if _, ok := shaped["query"]; !ok {
		t.Fatalf("expected run_query response JSON to contain 'query', got %v", shaped)
	}
	if _, ok := shaped["totalMatches"]; !ok {
		t.Fatalf("expected run_query response JSON to contain 'totalMatches', got %v", shaped)
	}
	if _, ok := shaped["matches"]; !ok {
		t.Fatalf("expected run_query response JSON to contain 'matches', got %v", shaped)
	}
}

func TestRegisterTools_RunQuery_FilePathUsesRequestedFileAndPreservesSuccessShape(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "query-target.pas")
	const queryText = "UniqueProcedureDeclarationToken"
	if err := os.WriteFile(targetFile, []byte("procedure "+queryText+";\n"), 0o600); err != nil {
		t.Fatalf("failed to write temp file for run_query test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":    queryText,
				"filePath": targetFile,
				"limit":    5,
			},
		},
		99,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query with filePath to succeed, got %s", string(resultBytes))
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	if got, _ := shaped["query"].(string); got != queryText {
		t.Fatalf("expected run_query response JSON to preserve query %q, got %q", queryText, got)
	}
	if _, ok := shaped["totalMatches"]; !ok {
		t.Fatalf("expected run_query response JSON to contain 'totalMatches', got %v", shaped)
	}
	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected run_query response JSON to contain array field 'matches', got %v", shaped)
	}
	if len(matches) == 0 {
		t.Fatalf("expected run_query with filePath to return at least one match from %q, got %v", targetFile, shaped)
	}
	firstMatch, ok := matches[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first run_query match to be an object, got %T", matches[0])
	}
	if gotFile, _ := firstMatch["file"].(string); gotFile != targetFile {
		t.Fatalf("expected run_query to scan requested file %q, got %q", targetFile, gotFile)
	}
}

func TestRegisterTools_RunQuery_InvalidFilePathReturnsError(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	missingPath := filepath.Join(t.TempDir(), "does-not-exist.pas")
	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":    "procedure",
				"filePath": missingPath,
			},
		},
		100,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal run_query invalid-filePath result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode run_query invalid-filePath result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatalf("expected run_query with invalid filePath to return tool error, got %s", string(resultBytes))
	}

	if !strings.Contains(strings.ToLower(string(resultBytes)), "filepath") && !strings.Contains(strings.ToLower(string(resultBytes)), "read") {
		t.Fatalf("expected run_query invalid filePath error to mention filePath/read failure, got %s", string(resultBytes))
	}
}

func TestRegisterTools_RunQuery_ReturnsErrorWhenNoFilesCanBeRead(t *testing.T) {
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to capture working directory: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change working directory for run_query test: %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(originalWD); chdirErr != nil {
			t.Fatalf("failed to restore working directory: %v", chdirErr)
		}
	})

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query": "procedure_declaration",
			},
		},
		101,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal run_query no-readable-files result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode run_query no-readable-files result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatalf("expected run_query with no readable files to return tool error, got %s", string(resultBytes))
	}

	if !strings.Contains(strings.ToLower(string(resultBytes)), "read") {
		t.Fatalf("expected run_query no-readable-files error to mention read failure, got %s", string(resultBytes))
	}
}

func TestRegisterTools_RunQuery_ScansWorkspaceDelphiFilesAndFindsTempToken(t *testing.T) {
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to capture working directory: %v", err)
	}

	tempDir := t.TempDir()
	targetPas := filepath.Join(tempDir, "workspace_target.pas")
	const queryText = "UniqueWorkspaceDelphiToken_20260505"

	if err := os.WriteFile(targetPas, []byte("unit WorkspaceTarget;\ninterface\nprocedure "+queryText+";\nimplementation\nend.\n"), 0o600); err != nil {
		t.Fatalf("failed to write Delphi temp file for run_query workspace scan test: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tempDir, "workspace_target.dpr"), []byte("program WorkspaceTarget;\nbegin\nend.\n"), 0o600); err != nil {
		t.Fatalf("failed to write Delphi .dpr temp file: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tempDir, "workspace_target.dpk"), []byte("package WorkspaceTarget;\nend.\n"), 0o600); err != nil {
		t.Fatalf("failed to write Delphi .dpk temp file: %v", err)
	}

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change working directory for run_query workspace scan test: %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(originalWD); chdirErr != nil {
			t.Fatalf("failed to restore working directory: %v", chdirErr)
		}
	})

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query": queryText,
				"limit": 5,
			},
		},
		102,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query workspace scan to succeed when Delphi files exist, got %s", string(resultBytes))
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected run_query workspace scan to return matches array, got %v", shaped)
	}
	if len(matches) == 0 {
		t.Fatalf("expected run_query workspace scan to find token %q inside Delphi workspace files, got %v", queryText, shaped)
	}
}

func TestRegisterTools_RunQuery_ReturnsStructuredMatchesWithRequiredFields(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "structured_target.pas")
	const queryText = "UniqueStructuredToken_20260505"

	if err := os.WriteFile(targetFile, []byte("unit StructuredTarget;\ninterface\nprocedure "+queryText+";\nimplementation\nend.\n"), 0o600); err != nil {
		t.Fatalf("failed to write temp file for structured run_query test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":    queryText,
				"filePath": targetFile,
				"limit":    5,
			},
		},
		103,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query structured shape scenario to succeed, got %s", string(resultBytes))
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	if _, ok := shaped["query"]; !ok {
		t.Fatalf("expected structured run_query payload to keep top-level query, got %v", shaped)
	}
	if _, ok := shaped["totalMatches"]; !ok {
		t.Fatalf("expected structured run_query payload to keep top-level totalMatches, got %v", shaped)
	}
	matches, ok := shaped["matches"].([]any)
	if !ok || len(matches) == 0 {
		t.Fatalf("expected structured run_query payload to include non-empty matches array, got %v", shaped)
	}

	firstMatch, ok := matches[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first match to be an object, got %T", matches[0])
	}

	requiredMatchFields := []string{"filePath", "startLine", "startColumn", "endLine", "endColumn", "nodeType", "preview"}
	for _, field := range requiredMatchFields {
		if _, ok := firstMatch[field]; !ok {
			t.Fatalf("expected run_query match to include required field %q, got %v", field, firstMatch)
		}
	}
}

func TestRegisterTools_RunQuery_StrictFilePathTrue_DoesNotFallbackToOtherFiles(t *testing.T) {
	tempDir := t.TempDir()
	strictFile := filepath.Join(tempDir, "strict_target.pas")
	if err := os.WriteFile(strictFile, []byte("unit StrictTarget;\ninterface\nimplementation\nend.\n"), 0o600); err != nil {
		t.Fatalf("failed to write strict target file: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":          "runQueryTextScan",
				"filePath":       strictFile,
				"strictFilePath": true,
				"limit":          10,
			},
		},
		104,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected strict filePath run_query scenario to succeed with zero matches, got %s", string(resultBytes))
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	totalMatches, ok := shaped["totalMatches"].(float64)
	if !ok {
		t.Fatalf("expected strict run_query payload to include numeric totalMatches, got %v", shaped)
	}
	if int(totalMatches) != 0 {
		t.Fatalf("expected strictFilePath=true to avoid fallback and return 0 matches for %q, got totalMatches=%v payload=%v", strictFile, totalMatches, shaped)
	}
	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected strict run_query payload to include matches array, got %v", shaped)
	}
	if len(matches) != 0 {
		t.Fatalf("expected strictFilePath=true to return empty matches when target file has no query term, got %v", matches)
	}
}

func TestRegisterTools_RunQuery_StrictFilePathTrue_WithoutFilePathReturnsValidationError(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":          "procedure",
				"strictFilePath": true,
				"limit":          5,
			},
		},
		105,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal strict-without-filePath run_query result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode strict-without-filePath run_query result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatalf("expected run_query with strictFilePath=true and missing filePath to return tool error, got %s", string(resultBytes))
	}

	lowerPayload := strings.ToLower(string(resultBytes))
	if !strings.Contains(lowerPayload, "strictfilepath") || !strings.Contains(lowerPayload, "filepath") {
		t.Fatalf("expected strictFilePath validation error to mention strictFilePath and filePath requirement, got %s", string(resultBytes))
	}
}

func TestRegisterTools_RunQuery_DpkPackageIncludesSymbolName(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "MyPackage.dpk")
	dpkContent := "package MyPackage;\nrequires rtl;\ncontains Unit1 in 'Unit1.pas';\nend.\n"
	if err := os.WriteFile(targetFile, []byte(dpkContent), 0o600); err != nil {
		t.Fatalf("failed to write temp .dpk for run_query test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":          "MyPackage",
				"filePath":       targetFile,
				"strictFilePath": true,
			},
		},
		111,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query for .dpk package query to succeed, got tool error payload=%s", string(resultBytes))
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	totalMatches, ok := shaped["totalMatches"].(float64)
	if !ok {
		t.Fatalf("expected run_query .dpk payload to include numeric totalMatches, got %v", shaped)
	}
	if int(totalMatches) < 1 {
		shapedBytes, _ := json.Marshal(shaped)
		t.Fatalf("expected run_query .dpk query to return at least 1 match, got totalMatches=%v payload=%s", totalMatches, string(shapedBytes))
	}

	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected run_query .dpk payload to include matches array, got %v", shaped)
	}
	if len(matches) == 0 {
		shapedBytes, _ := json.Marshal(shaped)
		t.Fatalf("expected run_query .dpk payload to include at least one match entry, got payload=%s", string(shapedBytes))
	}

	firstMatch, ok := matches[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first run_query .dpk match to be object, got %T", matches[0])
	}

	symbolName, _ := firstMatch["symbolName"].(string)
	if strings.TrimSpace(symbolName) == "" {
		firstBytes, _ := json.Marshal(firstMatch)
		t.Fatalf("expected first run_query .dpk match to include non-empty symbolName, got %s", string(firstBytes))
	}
	if !strings.Contains(symbolName, "MyPackage") {
		firstBytes, _ := json.Marshal(firstMatch)
		t.Fatalf("expected first run_query .dpk match symbolName to contain MyPackage, got symbolName=%q match=%s", symbolName, string(firstBytes))
	}

	nodeType, _ := firstMatch["nodeType"].(string)
	if strings.EqualFold(nodeType, "ERROR") {
		// Diagnostic only: current parser/query behavior may classify .dpk package declaration as ERROR node type.
		t.Logf("diagnostic: run_query .dpk first match returned nodeType=%q while symbolName=%q", nodeType, symbolName)
	}
}

func TestRegisterTools_RunQuery_PackageLikeErrorOutsideDpkDoesNotPopulateSymbolName(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "MyPackage.pas")
	pasContent := "package MyPackage;\n"
	if err := os.WriteFile(targetFile, []byte(pasContent), 0o600); err != nil {
		t.Fatalf("failed to write temp .pas package-like file for run_query test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":          "MyPackage",
				"filePath":       targetFile,
				"strictFilePath": true,
			},
		},
		112,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		return
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected run_query package-like .pas response to include matches array, got %v", shaped)
	}

	for i, m := range matches {
		match, ok := m.(map[string]any)
		if !ok {
			t.Fatalf("expected run_query package-like .pas match to be object at index %d, got %T", i, m)
		}
		symbolName, _ := match["symbolName"].(string)
		if strings.TrimSpace(symbolName) != "" {
			matchBytes, _ := json.Marshal(match)
			t.Fatalf("expected run_query package-like .pas to keep symbolName empty/absent, but matches[%d] has symbolName=%q match=%s", i, symbolName, string(matchBytes))
		}
	}
}

func TestRegisterTools_RunQuery_MalformedDpkPackageDoesNotPopulateSymbolName(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "MyPackage.dpk")
	dpkContent := "package MyPackage\nrequires rtl;\ncontains Unit1 in 'Unit1.pas';\nend.\n"
	if err := os.WriteFile(targetFile, []byte(dpkContent), 0o600); err != nil {
		t.Fatalf("failed to write malformed temp .dpk for run_query test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":          "MyPackage",
				"filePath":       targetFile,
				"strictFilePath": true,
			},
		},
		113,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		return
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected run_query malformed .dpk response to include matches array, got %v", shaped)
	}

	for i, m := range matches {
		match, ok := m.(map[string]any)
		if !ok {
			t.Fatalf("expected run_query malformed .dpk match to be object at index %d, got %T", i, m)
		}
		symbolName, _ := match["symbolName"].(string)
		if strings.TrimSpace(symbolName) != "" {
			matchBytes, _ := json.Marshal(match)
			t.Fatalf("expected run_query malformed .dpk to keep symbolName empty/absent, but matches[%d] has symbolName=%q match=%s", i, symbolName, string(matchBytes))
		}
	}
}

func decodeRunQueryCallResult(t *testing.T, result any, dest *map[string]any) {
	t.Helper()

	resultBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal run_query result: %v", err)
	}

	if err := json.Unmarshal(resultBytes, dest); err != nil {
		t.Fatalf("failed to decode run_query result map: %v", err)
	}
}

func decodeRunQuerySuccessPayload(t *testing.T, callResult map[string]any) map[string]any {
	t.Helper()

	contentRaw, ok := callResult["content"].([]any)
	if !ok || len(contentRaw) == 0 {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query success payload to include content array, got %s", string(resultBytes))
	}

	firstContent, ok := contentRaw[0].(map[string]any)
	if !ok {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected first content entry to be an object, got %s", string(resultBytes))
	}

	textPayload, _ := firstContent["text"].(string)
	if textPayload == "" {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query success payload to include text JSON body, got %s", string(resultBytes))
	}

	var shaped map[string]any
	if err := json.Unmarshal([]byte(textPayload), &shaped); err != nil {
		t.Fatalf("expected run_query text payload to be valid JSON, decode failed: %v; payload=%s", err, textPayload)
	}

	return shaped
}

func TestRegisterTools_RunQuery_NodeTypeFiltersByStructuralNode(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "filter_target.pas")
	// File has: a procedure decl, a function decl, and a comment that contains "AlphaProc".
	// A structural node_type filter must return only the procedure_declaration node,
	// excluding the comment match entirely.
	pasContent := "procedure AlphaProc;\nfunction BetaFunc: Integer;\n// AlphaProc is a helper routine\n"
	if err := os.WriteFile(targetFile, []byte(pasContent), 0o600); err != nil {
		t.Fatalf("failed to write temp .pas for node_type filter test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":          "AlphaProc",
				"node_type":      "procedure_declaration",
				"filePath":       targetFile,
				"strictFilePath": true,
			},
		},
		106,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query node_type filter to succeed without error, got %s", string(resultBytes))
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected run_query node_type filter response to include matches array, got %v", shaped)
	}
	if len(matches) == 0 {
		t.Fatalf("expected run_query with node_type=procedure_declaration and query=AlphaProc to return at least one match for the procedure declaration, got empty matches")
	}

	for i, m := range matches {
		match, ok := m.(map[string]any)
		if !ok {
			t.Fatalf("matches[%d] is not an object: %T", i, m)
		}
		nodeType, _ := match["nodeType"].(string)
		if nodeType != "procedure_declaration" {
			t.Fatalf("expected all run_query matches to have nodeType='procedure_declaration' when node_type filter is set, but matches[%d] has nodeType=%q; full match=%v", i, nodeType, match)
		}
		preview, _ := match["preview"].(string)
		trimmed := strings.TrimSpace(preview)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "(*") {
			t.Fatalf("expected no match to originate from a comment line when node_type=procedure_declaration, but matches[%d] preview=%q looks like a comment", i, preview)
		}
	}
}

func TestRegisterTools_RunQuery_NodeTypeWithoutQueryUsesStructuralMatch(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "node_type_without_query_target.pas")
	pasContent := "procedure AlphaProc;\nfunction BetaFunc: Integer;\n"
	if err := os.WriteFile(targetFile, []byte(pasContent), 0o600); err != nil {
		t.Fatalf("failed to write temp .pas for node_type without query test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"node_type":      "procedure_declaration",
				"filePath":       targetFile,
				"strictFilePath": true,
			},
		},
		107,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query with node_type and no query to succeed via structural match, got %s", string(resultBytes))
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected run_query node_type without query response to include matches array, got %v", shaped)
	}
	if len(matches) == 0 {
		t.Fatalf("expected run_query with node_type=procedure_declaration and no query to return at least one structural match, got %v", shaped)
	}

	firstMatch, ok := matches[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first run_query structural match to be an object, got %T", matches[0])
	}

	nodeType, _ := firstMatch["nodeType"].(string)
	if nodeType != "procedure_declaration" {
		t.Fatalf("expected first structural match nodeType=%q, got %q (match=%v)", "procedure_declaration", nodeType, firstMatch)
	}

	requiredFields := []string{"filePath", "startLine", "startColumn", "endLine", "endColumn", "nodeType", "preview", "file", "line", "text"}
	for _, field := range requiredFields {
		value, exists := firstMatch[field]
		if !exists {
			t.Fatalf("expected first structural match to include required field %q, got %v", field, firstMatch)
		}
		if textValue, ok := value.(string); ok && strings.TrimSpace(textValue) == "" {
			t.Fatalf("expected first structural match field %q to be non-empty, got %v", field, firstMatch)
		}
	}
}

func TestRegisterTools_RunQuery_QueryOnlyNodeTypeNameUsesImplicitStructuralMatch(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "query_only_node_type_target.pas")
	pasContent := "procedure AlphaProc;\nfunction BetaFunc: Integer;\n"
	if err := os.WriteFile(targetFile, []byte(pasContent), 0o600); err != nil {
		t.Fatalf("failed to write temp .pas for implicit structural node_type test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":          "procedure_declaration",
				"filePath":       targetFile,
				"strictFilePath": true,
			},
		},
		108,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query with query-only node_type name to succeed via implicit structural match, got %s", string(resultBytes))
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected run_query implicit structural match response to include matches array, got %v", shaped)
	}
	if len(matches) == 0 {
		t.Fatalf("expected run_query with query=procedure_declaration and no node_type to return at least one implicit structural match, got %v", shaped)
	}

	firstMatch, ok := matches[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first implicit structural run_query match to be an object, got %T", matches[0])
	}

	requiredFields := []string{"filePath", "startLine", "startColumn", "endLine", "endColumn", "nodeType", "preview", "file", "line", "text"}
	for _, field := range requiredFields {
		value, exists := firstMatch[field]
		if !exists {
			t.Fatalf("expected first implicit structural match to include required field %q, got %v", field, firstMatch)
		}
		if textValue, ok := value.(string); ok && strings.TrimSpace(textValue) == "" {
			t.Fatalf("expected first implicit structural match field %q to be non-empty, got %v", field, firstMatch)
		}
	}

	for i, m := range matches {
		match, ok := m.(map[string]any)
		if !ok {
			t.Fatalf("matches[%d] is not an object: %T", i, m)
		}
		nodeType, _ := match["nodeType"].(string)
		if nodeType != "procedure_declaration" {
			t.Fatalf("expected all implicit structural run_query matches to have nodeType='procedure_declaration', but matches[%d] has nodeType=%q; full match=%v", i, nodeType, match)
		}
	}
}

func TestRegisterTools_RunQuery_QueryOnlyFreeTextStillUsesLegacyFallback(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "query_only_free_text_target.pas")
	pasContent := "procedure AlphaProc;\nfunction BetaFunc: Integer;\n"
	if err := os.WriteFile(targetFile, []byte(pasContent), 0o600); err != nil {
		t.Fatalf("failed to write temp .pas for query-only free-text fallback test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":          "AlphaProc",
				"filePath":       targetFile,
				"strictFilePath": true,
			},
		},
		109,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query with free-text query and no node_type to keep legacy fallback behavior, got %s", string(resultBytes))
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected run_query free-text fallback response to include matches array, got %v", shaped)
	}
	if len(matches) == 0 {
		t.Fatalf("expected run_query with query=AlphaProc and no node_type to keep returning textual matches, got %v", shaped)
	}
}

func TestRegisterTools_RunQuery_LimitTruncatesReturnedMatches(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "limit_truncation_target.pas")
	pasContent := "procedure AlphaOne;\nprocedure AlphaTwo;\nprocedure AlphaThree;\n"
	if err := os.WriteFile(targetFile, []byte(pasContent), 0o600); err != nil {
		t.Fatalf("failed to write temp .pas for run_query limit truncation test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":          "procedure_declaration",
				"filePath":       targetFile,
				"strictFilePath": true,
				"limit":          1,
			},
		},
		110,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query limit truncation scenario to succeed, got %s", string(resultBytes))
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	totalMatches, ok := shaped["totalMatches"].(float64)
	if !ok {
		t.Fatalf("expected run_query limit truncation response to include numeric totalMatches, got %v", shaped)
	}
	if int(totalMatches) != 1 {
		t.Fatalf("expected run_query limit=1 to truncate totalMatches to 1, got totalMatches=%v payload=%v", totalMatches, shaped)
	}

	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected run_query limit truncation response to include matches array, got %v", shaped)
	}
	if len(matches) != 1 {
		t.Fatalf("expected run_query limit=1 to return exactly one match, got len(matches)=%d payload=%v", len(matches), shaped)
	}
}

func TestRegisterTools_RunQuery_FreeTextFallbackPreservesStructuredAndLegacyShape(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "free_text_shape_target.pas")
	pasContent := "procedure AlphaProc;\n"
	if err := os.WriteFile(targetFile, []byte(pasContent), 0o600); err != nil {
		t.Fatalf("failed to write temp .pas for run_query free-text shape test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":          "AlphaProc",
				"filePath":       targetFile,
				"strictFilePath": true,
			},
		},
		111,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query free-text fallback shape scenario to succeed, got %s", string(resultBytes))
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected run_query free-text fallback response to include matches array, got %v", shaped)
	}
	if len(matches) == 0 {
		t.Fatalf("expected run_query free-text fallback to return at least one match, got %v", shaped)
	}

	firstMatch, ok := matches[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first free-text fallback run_query match to be an object, got %T", matches[0])
	}

	requiredFields := []string{"filePath", "startLine", "startColumn", "endLine", "endColumn", "nodeType", "preview", "file", "line", "text"}
	for _, field := range requiredFields {
		value, exists := firstMatch[field]
		if !exists {
			t.Fatalf("expected first free-text fallback match to include required field %q, got %v", field, firstMatch)
		}
		if textValue, ok := value.(string); ok && strings.TrimSpace(textValue) == "" {
			t.Fatalf("expected first free-text fallback match field %q to be non-empty, got %v", field, firstMatch)
		}
	}
}

func TestRegisterTools_RunQuery_NodeTypeKeepsQueryAsAdditionalFilter(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "node_type_query_filter_target.pas")
	pasContent := "procedure AlphaProc;\nfunction BetaFunc: Integer;\n"
	if err := os.WriteFile(targetFile, []byte(pasContent), 0o600); err != nil {
		t.Fatalf("failed to write temp .pas for node_type additional filter test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":          "texto-inexistente",
				"node_type":      "procedure_declaration",
				"filePath":       targetFile,
				"strictFilePath": true,
			},
		},
		108,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query with node_type plus textual query filter to succeed, got %s", string(resultBytes))
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	totalMatches, ok := shaped["totalMatches"].(float64)
	if !ok {
		t.Fatalf("expected run_query node_type additional filter response to include numeric totalMatches, got %v", shaped)
	}
	if totalMatches != 0 {
		t.Fatalf("expected textual query to remain an additional filter when node_type is set, want totalMatches=0 got %v (payload=%v)", totalMatches, shaped)
	}

	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected run_query node_type additional filter response to include matches array, got %v", shaped)
	}
	if len(matches) != 0 {
		t.Fatalf("expected run_query with node_type=procedure_declaration and query=texto-inexistente to return no matches, got %v", matches)
	}
}

func TestRegisterTools_RunQuery_InvalidNodeTypeReturnsTreeSitterQueryError(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "invalid_nodetype_target.pas")
	if err := os.WriteFile(targetFile, []byte("unit Minimal;\ninterface\nimplementation\nend.\n"), 0o600); err != nil {
		t.Fatalf("failed to write temp .pas for invalid node_type test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":          "unit",
				"node_type":      "__invalid_node_type__",
				"filePath":       targetFile,
				"strictFilePath": true,
			},
		},
		107,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal run_query invalid-node_type result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode run_query invalid-node_type result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatalf("expected run_query with node_type='__invalid_node_type__' to return isError=true (invalid tree-sitter query), got success payload %s", string(resultBytes))
	}

	lowerPayload := strings.ToLower(string(resultBytes))
	if !strings.Contains(lowerPayload, "node_type") && !strings.Contains(lowerPayload, "nodetype") &&
		!strings.Contains(lowerPayload, "invalid") && !strings.Contains(lowerPayload, "tree-sitter") {
		t.Fatalf("expected run_query invalid node_type error message to mention node_type or tree-sitter query validation, got %s", string(resultBytes))
	}
}

func TestRegisterTools_RunQuery_NodeTypeIncludesCaptureName(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "capture_name_target.pas")
	if err := os.WriteFile(targetFile, []byte("procedure AlphaProc;\n"), 0o600); err != nil {
		t.Fatalf("failed to write temp .pas for captureName metadata test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":          "AlphaProc",
				"node_type":      "procedure_declaration",
				"strictFilePath": true,
				"filePath":       targetFile,
			},
		},
		108,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query node_type captureName scenario to succeed, got %s", string(resultBytes))
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected run_query node_type captureName payload to include matches array, got %v", shaped)
	}
	if len(matches) == 0 {
		t.Fatalf("expected run_query node_type captureName scenario to return at least one match, got %v", shaped)
	}

	firstMatch, ok := matches[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first match to be an object, got %T", matches[0])
	}

	captureName, _ := firstMatch["captureName"].(string)
	if strings.TrimSpace(captureName) == "" {
		t.Fatalf("expected first run_query match to include non-empty captureName metadata, got %v", firstMatch)
	}
	if captureName != "match" {
		t.Fatalf("expected first run_query match captureName to be at least %q for structural node_type query, got %q (match=%v)", "match", captureName, firstMatch)
	}
}

func TestRegisterTools_RunQuery_FreeTextFallbackDoesNotPopulateCaptureName(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "capture_name_freetext_target.pas")
	if err := os.WriteFile(targetFile, []byte("procedure AlphaProc;\n"), 0o600); err != nil {
		t.Fatalf("failed to write temp .pas for free-text captureName metadata test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":          "AlphaProc",
				"strictFilePath": true,
				"filePath":       targetFile,
			},
		},
		1081,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query free-text fallback captureName scenario to succeed, got %s", string(resultBytes))
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected run_query free-text fallback payload to include matches array, got %v", shaped)
	}
	if len(matches) == 0 {
		t.Fatalf("expected run_query free-text fallback scenario to return at least one match, got %v", shaped)
	}

	firstMatch, ok := matches[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first match to be an object, got %T", matches[0])
	}

	if _, exists := firstMatch["captureName"]; exists {
		captureName, _ := firstMatch["captureName"].(string)
		if strings.TrimSpace(captureName) != "" {
			t.Fatalf("expected first run_query free-text fallback match to omit captureName (or keep it empty), got %v", firstMatch)
		}
	}
}

func TestRegisterTools_RunQuery_DeclarationIncludesSymbolName(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "symbol_name_target.pas")
	pasContent := "procedure AlphaProc;\nfunction BetaFunc: Integer;\n"
	if err := os.WriteFile(targetFile, []byte(pasContent), 0o600); err != nil {
		t.Fatalf("failed to write temp .pas for symbolName metadata test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":          "AlphaProc",
				"node_type":      "procedure_declaration",
				"strictFilePath": true,
				"filePath":       targetFile,
			},
		},
		109,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query declaration symbolName scenario to succeed, got %s", string(resultBytes))
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected run_query declaration symbolName payload to include matches array, got %v", shaped)
	}
	if len(matches) == 0 {
		t.Fatalf("expected run_query declaration symbolName scenario to return at least one match, got %v", shaped)
	}

	firstMatch, ok := matches[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first match to be an object, got %T", matches[0])
	}

	symbolName, _ := firstMatch["symbolName"].(string)
	if strings.TrimSpace(symbolName) == "" {
		t.Fatalf("expected first run_query declaration match to include non-empty symbolName metadata, got %v", firstMatch)
	}
	if !strings.Contains(symbolName, "AlphaProc") {
		t.Fatalf("expected first run_query declaration match symbolName to contain %q, got %q (match=%v)", "AlphaProc", symbolName, firstMatch)
	}
}

func TestRegisterTools_RunQuery_UnitDeclarationIncludesSymbolName(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "unit_declaration_symbol_name_target.pas")
	pasContent := "unit MyUnit;\ninterface\nimplementation\nend.\n"
	if err := os.WriteFile(targetFile, []byte(pasContent), 0o600); err != nil {
		t.Fatalf("failed to write temp .pas for unit declaration symbolName metadata test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":          "MyUnit",
				"node_type":      "unit_declaration",
				"strictFilePath": true,
				"filePath":       targetFile,
			},
		},
		111,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query unit declaration symbolName scenario to succeed, got %s", string(resultBytes))
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected run_query unit declaration symbolName payload to include matches array, got %v", shaped)
	}
	if len(matches) == 0 {
		t.Fatalf("expected run_query unit declaration symbolName scenario to return at least one match, got %v", shaped)
	}

	firstMatch, ok := matches[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first match to be an object, got %T", matches[0])
	}

	symbolName, _ := firstMatch["symbolName"].(string)
	if strings.TrimSpace(symbolName) == "" {
		t.Fatalf("expected first run_query unit declaration match to include non-empty symbolName metadata, got %v", firstMatch)
	}
	if !strings.Contains(symbolName, "MyUnit") {
		t.Fatalf("expected first run_query unit declaration match symbolName to contain %q, got %q (match=%v)", "MyUnit", symbolName, firstMatch)
	}
}

func TestRegisterTools_RunQuery_ProgramDeclarationIncludesSymbolName(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "program_declaration_symbol_name_target.dpr")
	programContent := "program MyProgram;\nbegin\nend.\n"
	if err := os.WriteFile(targetFile, []byte(programContent), 0o600); err != nil {
		t.Fatalf("failed to write temp .dpr for program declaration symbolName metadata test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":          "MyProgram",
				"node_type":      "program_declaration",
				"strictFilePath": true,
				"filePath":       targetFile,
			},
		},
		112,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query program declaration symbolName scenario to succeed, got %s", string(resultBytes))
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected run_query program declaration symbolName payload to include matches array, got %v", shaped)
	}
	if len(matches) == 0 {
		t.Fatalf("expected run_query program declaration symbolName scenario to return at least one match, got %v", shaped)
	}

	firstMatch, ok := matches[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first match to be an object, got %T", matches[0])
	}

	symbolName, _ := firstMatch["symbolName"].(string)
	if strings.TrimSpace(symbolName) == "" {
		t.Fatalf("expected first run_query program declaration match to include non-empty symbolName metadata, got %v", firstMatch)
	}
	if !strings.Contains(symbolName, "MyProgram") {
		t.Fatalf("expected first run_query program declaration match symbolName to contain %q, got %q (match=%v)", "MyProgram", symbolName, firstMatch)
	}
}

func TestRegisterTools_RunQuery_LibraryDeclarationIncludesSymbolName(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "library_declaration_symbol_name_target.dpr")
	libraryContent := "library MyLibrary;\nbegin\nend.\n"
	if err := os.WriteFile(targetFile, []byte(libraryContent), 0o600); err != nil {
		t.Fatalf("failed to write temp .dpr for library declaration symbolName metadata test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":          "MyLibrary",
				"node_type":      "library_declaration",
				"strictFilePath": true,
				"filePath":       targetFile,
			},
		},
		113,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query library declaration symbolName scenario to succeed, got %s", string(resultBytes))
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected run_query library declaration symbolName payload to include matches array, got %v", shaped)
	}
	if len(matches) == 0 {
		t.Fatalf("expected run_query library declaration symbolName scenario to return at least one match, got %v", shaped)
	}

	firstMatch, ok := matches[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first match to be an object, got %T", matches[0])
	}

	symbolName, _ := firstMatch["symbolName"].(string)
	if strings.TrimSpace(symbolName) == "" {
		t.Fatalf("expected first run_query library declaration match to include non-empty symbolName metadata, got %v", firstMatch)
	}
	if !strings.Contains(symbolName, "MyLibrary") {
		t.Fatalf("expected first run_query library declaration match symbolName to contain %q, got %q (match=%v)", "MyLibrary", symbolName, firstMatch)
	}
}

func TestRegisterTools_RunQuery_NonRoutineDeclarationDoesNotPopulateSymbolName(t *testing.T) {
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "const_section_target.pas")
	pasContent := "unit ConstScope;\ninterface\nconst\n  Foo = 1;\nimplementation\nend.\n"
	if err := os.WriteFile(targetFile, []byte(pasContent), 0o600); err != nil {
		t.Fatalf("failed to write temp .pas for non-routine symbolName metadata test: %v", err)
	}

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query":          "Foo",
				"node_type":      "const_section",
				"strictFilePath": true,
				"filePath":       targetFile,
			},
		},
		110,
	)

	var callResult map[string]any
	decodeRunQueryCallResult(t, callResp.Result, &callResult)

	isError, _ := callResult["isError"].(bool)
	if isError {
		resultBytes, _ := json.Marshal(callResult)
		t.Fatalf("expected run_query non-routine declaration symbolName scenario to succeed, got %s", string(resultBytes))
	}

	shaped := decodeRunQuerySuccessPayload(t, callResult)
	matches, ok := shaped["matches"].([]any)
	if !ok {
		t.Fatalf("expected run_query non-routine declaration symbolName payload to include matches array, got %v", shaped)
	}
	if len(matches) == 0 {
		t.Fatalf("expected run_query non-routine declaration symbolName scenario to return at least one match, got %v", shaped)
	}

	firstMatch, ok := matches[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first match to be an object, got %T", matches[0])
	}

	if _, exists := firstMatch["symbolName"]; exists {
		symbolName, _ := firstMatch["symbolName"].(string)
		if strings.TrimSpace(symbolName) != "" {
			t.Fatalf("expected first run_query non-routine declaration match to omit symbolName (or keep it empty), got %v", firstMatch)
		}
	}
}

func TestRegisterTools_MemoryEdit_RejectsTitleOnlyArguments(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "memory_edit",
			"arguments": map[string]any{
				"title":   "Nota sem id",
				"content": "novo conteudo qualquer",
			},
		},
		101,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal memory_edit result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode memory_edit result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatalf("expected memory_edit with title-only args (no id) to return a tool error, got %s", string(resultBytes))
	}

	if !strings.Contains(string(resultBytes), "id") {
		t.Fatalf("expected memory_edit error to mention 'id', got %s", string(resultBytes))
	}
}

func TestRegisterTools_MemoryRead_RejectsTitleOnlyArguments(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "memory_read",
			"arguments": map[string]any{
				"title": "Nota qualquer sem id",
			},
		},
		102,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal memory_read result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode memory_read result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatalf("expected memory_read with title-only args (no id) to return a tool error, got %s", string(resultBytes))
	}

	if !strings.Contains(string(resultBytes), "id") {
		t.Fatalf("expected memory_read error to mention 'id', got %s", string(resultBytes))
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
