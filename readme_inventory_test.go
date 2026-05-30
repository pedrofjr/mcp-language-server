package main

import (
	"os"
	"strings"
	"testing"
)

// TestReadme_ListsCriticalToolsFromInventory ensures README documents critical MCP tools.
func TestReadme_ListsCriticalToolsFromInventory(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	text := string(readme)

	if strings.Contains(text, "6 ferramentas") || strings.Contains(text, "six tools") {
		t.Fatal("README still cites obsolete fixed tool count")
	}

	// Tools documented in README feature list (subset of criticalMCPTools).
	documentedCritical := []string{
		"definition", "references", "diagnostics", "hover", "rename_symbol",
		"edit_file", "get_symbols_overview", "run_query",
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

// TestReadme_MentionsCoreLSPBackedTools documents primary LSP-backed tools in README.
func TestReadme_MentionsCoreLSPBackedTools(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	text := string(readme)
	core := []string{
		"definition", "references", "hover", "rename_symbol", "diagnostics",
		"edit_file", "workspace_symbols", "get_symbols_overview", "run_query",
	}
	for _, name := range core {
		if !strings.Contains(text, "`"+name+"`") {
			t.Fatalf("README missing core tool %q", name)
		}
	}
}
