package main

import (
	"encoding/json"
	"os"
	"sort"
	"testing"
)

func criticalLspBackedToolNames() []string {
	var names []string
	for _, entry := range registeredToolInventory() {
		if entry.Critical && entry.Kind == ToolKindLSPBacked {
			names = append(names, entry.Name)
		}
	}
	sort.Strings(names)
	return names
}

func TestGovernanceLspBackedCriticalManifest_MatchesInventory(t *testing.T) {
	data, err := os.ReadFile("docs/governance-lsp-backed-critical.json")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest struct {
		Tools []string `json:"tools"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	expected := criticalLspBackedToolNames()
	if len(manifest.Tools) != len(expected) {
		t.Fatalf("manifest tools=%d inventory=%d", len(manifest.Tools), len(expected))
	}
	for i, name := range expected {
		if manifest.Tools[i] != name {
			t.Fatalf("manifest[%d]=%q expected %q", i, manifest.Tools[i], name)
		}
	}
}
