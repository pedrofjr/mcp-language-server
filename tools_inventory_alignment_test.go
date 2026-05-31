package main

import (
	"os"
	"strings"
	"testing"
)

func TestToolInventory_LSPBackedToolsUseLSPGuard(t *testing.T) {
	lspBacked := map[string]bool{}
	for _, entry := range registeredToolInventory() {
		if entry.Kind == ToolKindLSPBacked {
			lspBacked[entry.Name] = true
		}
	}
	for _, name := range []string{"get_node_at_position", "get_node_types"} {
		if !lspBacked[name] {
			t.Fatalf("tool %q must be ToolKindLSPBacked when registered with withLSPGuard", name)
		}
	}
}

func TestToolInventory_LocalOnlyToolsNotBehindLSPGuard(t *testing.T) {
	src, err := os.ReadFile("tools.go")
	if err != nil {
		t.Fatalf("read tools.go: %v", err)
	}
	text := string(src)
	for _, entry := range registeredToolInventory() {
		if entry.Kind != ToolKindLocalOnly {
			continue
		}
		if entry.Name == "run_query" {
			continue
		}
		pattern := `withToolLogging("` + entry.Name + `"`
		idx := strings.Index(text, pattern)
		if idx < 0 {
			continue
		}
		snippet := text[idx : min(len(text), idx+500)]
		if strings.Contains(snippet, "withLSPGuard") {
			t.Fatalf("local-only tool %q must not be registered behind withLSPGuard", entry.Name)
		}
	}
}
