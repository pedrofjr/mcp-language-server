package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestMcpToolsPublic_RequiredKeysFromToolsListSchema(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	listResp := handleTestMCPRequest(t, svc, mcp.MethodToolsList, map[string]any{}, 90)
	var listResult mcp.ListToolsResult
	decodeTestMCPResult(t, listResp.Result, &listResult)

	byName := make(map[string]mcp.Tool, len(listResult.Tools))
	for _, tool := range listResult.Tools {
		byName[tool.Name] = tool
	}

	catalog, err := os.ReadFile("docs/MCP_TOOLS_PUBLIC.md")
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	text := string(catalog)

	for _, entry := range registeredToolInventory() {
		if !entry.Critical {
			continue
		}
		tool, ok := byName[entry.Name]
		if !ok {
			t.Fatalf("critical tool %q missing from tools/list", entry.Name)
		}
		row := criticalCatalogRow(text, entry.Name)
		if row == "" {
			t.Fatalf("critical tool %q missing from MCP_TOOLS_PUBLIC.md", entry.Name)
		}
		example := extractExampleJSONFromCatalogRow(row)
		if example == nil {
			t.Fatalf("critical tool %q: could not parse example JSON from catalog", entry.Name)
		}
		if len(tool.InputSchema.Required) == 0 {
			if example == nil {
				t.Fatalf("critical tool %q: catalog example required even when schema has no required keys", entry.Name)
			}
			continue
		}
		for _, req := range tool.InputSchema.Required {
			if _, ok := tool.InputSchema.Properties[req]; !ok {
				t.Fatalf("tools/list %q: required %q missing from schema properties", entry.Name, req)
			}
			if _, ok := example[req]; !ok {
				t.Fatalf("catalog example for %q missing required key %q from tools/list schema", entry.Name, req)
			}
		}
		raw, err := json.Marshal(example)
		if err != nil {
			t.Fatalf("marshal example for %q: %v", entry.Name, err)
		}
		if len(raw) < 3 {
			t.Fatalf("catalog example for %q is empty", entry.Name)
		}
	}
}
