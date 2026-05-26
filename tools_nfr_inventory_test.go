package main

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestToolInventory_MatchesToolsList(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	listed := listedToolNamesFromServer(t, svc)
	inventory := toolInventoryMap()

	listedSet := make(map[string]struct{}, len(listed))
	for _, name := range listed {
		listedSet[name] = struct{}{}
		if _, ok := inventory[name]; !ok {
			t.Fatalf("tool %q appears in tools/list but is missing from registeredToolInventory", name)
		}
	}

	for name := range inventory {
		if _, ok := listedSet[name]; !ok {
			t.Fatalf("tool %q is documented in registeredToolInventory but missing from tools/list", name)
		}
	}
}

func TestToolNFRInventory_EveryCriticalToolHasValidationAndOperationalScenarios(t *testing.T) {
	fixturePath := "C:/workspace/Unit1.pas"
	byTool := toolNFRErrorScenariosByTool(fixturePath)

	for _, entry := range registeredToolInventory() {
		if !entry.Critical {
			continue
		}
		scenarios, ok := byTool[entry.Name]
		if !ok {
			t.Fatalf("critical tool %q (%s) has no NFR error scenarios; add cases to toolNFRErrorScenarios", entry.Name, entry.Kind)
		}
		if _, ok := scenarios[NFRErrorScenarioValidation]; !ok {
			t.Fatalf("critical tool %q missing validation NFR scenario", entry.Name)
		}
		if _, ok := scenarios[NFRErrorScenarioOperational]; !ok {
			if entry.Name == "memory_write" {
				continue
			}
			t.Fatalf("critical tool %q missing operational NFR scenario", entry.Name)
		}
	}
}

func TestToolNFRInventory_NoOrphanScenariosForUnknownTools(t *testing.T) {
	inventory := toolInventoryMap()
	fixturePath := "C:/workspace/Unit1.pas"

	for _, scenario := range toolNFRErrorScenarios(fixturePath) {
		entry, ok := inventory[scenario.Tool]
		if !ok {
			t.Fatalf("NFR scenario references unknown tool %q", scenario.Tool)
		}
		if !entry.Critical && scenario.Kind == NFRErrorScenarioOperational {
			t.Fatalf("operational NFR scenario registered for non-critical tool %q", scenario.Tool)
		}
	}
}

func listedToolNamesFromServer(t *testing.T, svc *mcpServer) []string {
	t.Helper()

	listResp := handleTestMCPRequest(t, svc, mcp.MethodToolsList, map[string]any{}, 2)
	var listResult mcp.ListToolsResult
	decodeTestMCPResult(t, listResp.Result, &listResult)

	names := make([]string, 0, len(listResult.Tools))
	for _, tool := range listResult.Tools {
		names = append(names, tool.Name)
	}
	return names
}
