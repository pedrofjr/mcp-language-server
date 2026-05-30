package main

import (
	"os"
	"strings"
	"testing"
)

// TestReadme_ReferencesRegisteredToolInventory ensures README indexes the public tool surface.
func TestReadme_ReferencesRegisteredToolInventory(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	text := string(readme)
	if !strings.Contains(text, "tools_inventory.go") || !strings.Contains(text, "TestToolInventory_MatchesToolsList") {
		t.Fatal("README must reference tools_inventory.go and TestToolInventory_MatchesToolsList")
	}
	documentedCritical := []string{
		"definition", "references", "diagnostics", "hover", "rename_symbol",
		"edit_file", "get_symbols_overview", "run_query", "workspace_symbols",
		"code_actions", "dependency_tree", "memory_read", "memory_write",
	}
	for _, name := range documentedCritical {
		if _, ok := criticalMCPTools[name]; !ok {
			t.Fatalf("documentedCritical %q not in criticalMCPTools", name)
		}
		if !strings.Contains(text, "`"+name+"`") {
			t.Fatalf("README missing critical tool %q", name)
		}
	}
}
