package main

import (
	"os"
	"strings"
	"testing"
)

func TestMcpToolsPublic_CriticalityBidirectional(t *testing.T) {
	catalog, err := os.ReadFile("docs/MCP_TOOLS_PUBLIC.md")
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	text := string(catalog)

	for _, entry := range registeredToolInventory() {
		row := criticalCatalogRow(text, entry.Name)
		if row == "" {
			continue
		}
		docCritical := strings.Contains(row, "| yes |")
		if entry.Critical && !docCritical {
			t.Fatalf("tool %q Critical in inventory but not marked critical in MCP_TOOLS_PUBLIC.md", entry.Name)
		}
		if !entry.Critical && docCritical {
			t.Fatalf("tool %q not critical in inventory but marked critical in MCP_TOOLS_PUBLIC.md", entry.Name)
		}
	}
}
