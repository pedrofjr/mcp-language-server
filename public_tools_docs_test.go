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

	inventory := registeredToolInventory()
	var missingCritical []string
	for _, entry := range inventory {
		if !entry.Critical {
			continue
		}
		if !strings.Contains(readmeText, "`"+entry.Name+"`") && !strings.Contains(readmeText, entry.Name) {
			missingCritical = append(missingCritical, entry.Name)
		}
	}
	if len(missingCritical) > 0 {
		t.Fatalf("README missing critical tools: %v", missingCritical)
	}

	if claudeText != "" {
		if strings.Contains(claudeText, "211 passed") || strings.Contains(claudeText, "213 passed") || strings.Contains(claudeText, "217 passed") {
			t.Fatal("CLAUDE.md must not embed stale go test counts")
		}
		if !strings.Contains(strings.ToLower(claudeText), "lsp") {
			t.Fatal("CLAUDE.md must state LSP as authoritative Delphi source")
		}
	}
}

func TestReadme_ReferencesRegisteredToolInventory(t *testing.T) {
	TestPublicTools_ReadmeAndClaudeAlignWithInventory(t)
}
