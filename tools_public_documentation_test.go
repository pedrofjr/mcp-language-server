package main

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestPublicTools_ToolsListDocumentsEveryInventoryEntry validates public contract fields per tool.
func TestPublicTools_ToolsListDocumentsEveryInventoryEntry(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	listResp := handleTestMCPRequest(t, svc, mcp.MethodToolsList, map[string]any{}, 2)
	var listResult mcp.ListToolsResult
	decodeTestMCPResult(t, listResp.Result, &listResult)

	byName := make(map[string]mcp.Tool, len(listResult.Tools))
	for _, tool := range listResult.Tools {
		byName[tool.Name] = tool
	}

	for _, entry := range registeredToolInventory() {
		tool, ok := byName[entry.Name]
		if !ok {
			t.Fatalf("tool %q missing from tools/list", entry.Name)
		}
		if strings.TrimSpace(tool.Description) == "" {
			t.Fatalf("tool %q has empty description in tools/list", entry.Name)
		}
		if entry.Name == "run_query" {
			desc := strings.ToLower(tool.Description)
			if !strings.Contains(desc, "fallback") && !strings.Contains(desc, "degradad") {
				t.Fatalf("run_query tools/list description must document degraded fallback")
			}
		}
		if len(tool.InputSchema.Required) > 0 {
			for _, req := range tool.InputSchema.Required {
				if _, ok := tool.InputSchema.Properties[req]; !ok {
					t.Fatalf("tool %q lists required param %q missing from schema properties", entry.Name, req)
				}
			}
		}
	}
}
