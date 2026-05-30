package main

import (
	"testing"
)

func TestRegisteredToolInventory_CoversAllLSPBackedRequestContextCases(t *testing.T) {
	inventory := registeredToolInventory()
	byName := make(map[string]ToolInventoryEntry, len(inventory))
	for _, entry := range inventory {
		byName[entry.Name] = entry
	}

	cases := lspBackedToolRequestContextCases("C:/workspace/Unit1.pas")
	for _, tc := range cases {
		entry, ok := byName[tc.name]
		if !ok {
			t.Fatalf("request-context case %q missing from registeredToolInventory", tc.name)
		}
		if entry.Kind != ToolKindLSPBacked {
			t.Fatalf("expected %q to be lsp-backed in inventory, got %q", tc.name, entry.Kind)
		}
	}
}

func TestCriticalMCPTools_AreRegisteredInventoryEntries(t *testing.T) {
	byName := make(map[string]ToolInventoryEntry)
	for _, entry := range registeredToolInventory() {
		byName[entry.Name] = entry
	}
	for name := range criticalMCPTools {
		entry, ok := byName[name]
		if !ok {
			t.Fatalf("criticalMCPTools %q missing from registeredToolInventory", name)
		}
		if !entry.Critical {
			t.Fatalf("criticalMCPTools %q is not Critical in inventory", name)
		}
	}
}

func TestRegisteredToolInventory_EveryCriticalLSPBackedToolHasRequestContextCase(t *testing.T) {
	cases := lspBackedToolRequestContextCases("C:/workspace/Unit1.pas")
	covered := make(map[string]struct{}, len(cases))
	for _, tc := range cases {
		covered[tc.name] = struct{}{}
	}

	for _, entry := range registeredToolInventory() {
		if entry.Kind != ToolKindLSPBacked || !entry.Critical {
			continue
		}
		if _, ok := covered[entry.Name]; !ok {
			t.Fatalf("critical LSP-backed tool %q missing from lspBackedToolRequestContextCases", entry.Name)
		}
	}
}
