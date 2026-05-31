package main

import (
	"os"
	"strings"
	"testing"
)

func TestPublicTools_ReadmeAndClaudeAlignWithInventory(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	readmeText := string(readme)
	if !strings.Contains(readmeText, "tools_inventory.go") || !strings.Contains(readmeText, "TestToolInventory_MatchesToolsList") {
		t.Fatal("README must reference tools_inventory.go and TestToolInventory_MatchesToolsList")
	}

	claudeText := ""
	if data, readErr := os.ReadFile("CLAUDE.md"); readErr == nil {
		claudeText = string(data)
	}

	if !strings.Contains(readmeText, "tools/list") {
		t.Fatal("README must document MCP tools/list for full public tool catalog")
	}

	inventory := registeredToolInventory()
	var missingCritical []string
	for _, entry := range inventory {
		if !entry.Critical {
			continue
		}
		if !strings.Contains(readmeText, entry.Name) {
			missingCritical = append(missingCritical, entry.Name)
		}
	}
	if len(missingCritical) > 0 {
		t.Fatalf("README missing critical tools: %v", missingCritical)
	}

	catalog, err := os.ReadFile("docs/MCP_TOOLS_PUBLIC.md")
	if err != nil {
		t.Fatal("docs/MCP_TOOLS_PUBLIC.md required for public tool catalog without opening Go code")
	}
	catalogText := string(catalog)
	var missingCatalog []string
	for _, entry := range inventory {
		if !strings.Contains(catalogText, "| "+entry.Name+" |") && !strings.Contains(catalogText, "`"+entry.Name+"`") {
			missingCatalog = append(missingCatalog, entry.Name)
		}
	}
	if len(missingCatalog) > 0 {
		t.Fatalf("docs/MCP_TOOLS_PUBLIC.md missing tools: %v", missingCatalog)
	}

	if claudeText != "" {
		if strings.Contains(claudeText, "211 passed") || strings.Contains(claudeText, "213 passed") || strings.Contains(claudeText, "217 passed") {
			t.Fatal("CLAUDE.md must not embed stale go test counts")
		}
		if !strings.Contains(strings.ToLower(claudeText), "lsp") {
			t.Fatal("CLAUDE.md must state LSP as authoritative Delphi source")
		}
		if !strings.Contains(claudeText, "tools_inventory") && !strings.Contains(claudeText, "tools/list") {
			t.Fatal("CLAUDE.md must reference tools_inventory or tools/list for public catalog")
		}
	}
}

func TestReadme_ReferencesRegisteredToolInventory(t *testing.T) {
	TestPublicTools_ReadmeAndClaudeAlignWithInventory(t)
}
