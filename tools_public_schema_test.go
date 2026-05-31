package main

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

var exampleJSONRe = regexp.MustCompile("`(\\{[^`]*\\})`")

func extractExampleJSONFromCatalogRow(row string) map[string]any {
	m := exampleJSONRe.FindStringSubmatch(row)
	if len(m) < 2 {
		return nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(m[1]), &parsed); err != nil {
		return nil
	}
	return parsed
}

func requiredKeysForCriticalTool(name string, fixturePath string) []string {
	switch name {
	case "get_symbols_overview", "ast_summary", "dependency_tree":
		return []string{"uri"}
	case "graph_query":
		return []string{"uri"}
	case "call_graph", "graph_neighbors", "graph_node":
		return []string{"symbolName"}
	case "replace_symbol_body":
		return []string{"filePath", "symbolName", "newBody"}
	case "insert_after_symbol", "insert_before_symbol":
		return []string{"filePath", "symbolName", "text"}
	case "safe_delete_symbol":
		return []string{"filePath", "symbolName"}
	case "get_diagnostics_for_symbol":
		return []string{"filePath", "symbolName"}
	case "find_implementations":
		return []string{"symbolName"}
	case "rename_symbol":
		return []string{"filePath", "line", "column", "newName"}
	case "edit_file":
		return []string{"filePath", "edits"}
	case "onboarding", "memory_list", "list_memories":
		return nil
	case "check_onboarding_performed":
		return []string{"projectPath"}
	case "memory_write", "write_memory":
		return []string{"title", "content"}
	case "memory_edit", "edit_memory":
		return []string{"id", "content"}
	case "memory_read", "read_memory", "memory_delete", "delete_memory":
		return []string{"id"}
	case "run_query":
		return []string{"query"}
	default:
		break
	}
	for _, tc := range lspBackedToolRequestContextCases(fixturePath) {
		if tc.name == name {
			keys := make([]string, 0, len(tc.args))
			for k := range tc.args {
				keys = append(keys, k)
			}
			return keys
		}
	}
	return nil
}

func TestMcpToolsPublic_ExamplesMatchRequestContextSchema(t *testing.T) {
	catalog, err := os.ReadFile("docs/MCP_TOOLS_PUBLIC.md")
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	text := string(catalog)
	fixture := "test-fixtures/harness-cli/smoke.pas"

	for _, entry := range registeredToolInventory() {
		if !entry.Critical {
			continue
		}
		row := criticalCatalogRow(text, entry.Name)
		if row == "" {
			continue
		}
		example := extractExampleJSONFromCatalogRow(row)
		if example == nil {
			t.Fatalf("critical tool %q: could not parse example JSON from catalog row", entry.Name)
		}
		required := requiredKeysForCriticalTool(entry.Name, fixture)
		for _, key := range required {
			if _, ok := example[key]; !ok {
				t.Fatalf("tool %q example missing required key %q (contract from request-context cases)", entry.Name, key)
			}
		}
		if strings.Contains(strings.ToLower(row), "ver tools/list") {
			t.Fatalf("tool %q must not defer parameters to tools/list", entry.Name)
		}
	}
}
