package main

import "path/filepath"

// NFRErrorScenarioKind classifies operational error coverage scenarios.
type NFRErrorScenarioKind string

const (
	NFRErrorScenarioValidation  NFRErrorScenarioKind = "validation"
	NFRErrorScenarioOperational NFRErrorScenarioKind = "operational"
)

// ToolNFRErrorScenario documents a deterministic error path for NFR gates.
type ToolNFRErrorScenario struct {
	Tool            string
	Kind            NFRErrorScenarioKind
	Args            map[string]any
	Reason          string
	RequiresFakeLSP bool
}

func toolInventoryMap() map[string]ToolInventoryEntry {
	entries := registeredToolInventory()
	byName := make(map[string]ToolInventoryEntry, len(entries))
	for _, entry := range entries {
		byName[entry.Name] = entry
	}
	return byName
}

func criticalToolNamesFromInventory() []string {
	names := make([]string, 0)
	for _, entry := range registeredToolInventory() {
		if entry.Critical {
			names = append(names, entry.Name)
		}
	}
	return names
}

// toolNFRErrorScenarios is the authoritative NFR error matrix for critical tools.
// Every Critical=true entry in registeredToolInventory must have validation + operational scenarios.
func toolNFRErrorScenarios(fixturePath string) []ToolNFRErrorScenario {
	fileURI := "file:///" + filepath.ToSlash(fixturePath)
	return []ToolNFRErrorScenario{
		// --- validation (invalid args) ---
		{Tool: "edit_file", Kind: NFRErrorScenarioValidation, Args: map[string]any{}, Reason: "missing filePath/edits"},
		{Tool: "definition", Kind: NFRErrorScenarioValidation, Args: map[string]any{}, Reason: "missing symbolName"},
		{Tool: "references", Kind: NFRErrorScenarioValidation, Args: map[string]any{}, Reason: "missing symbolName"},
		{Tool: "diagnostics", Kind: NFRErrorScenarioValidation, Args: map[string]any{"filePath": 1}, Reason: "filePath type mismatch"},
		{Tool: "hover", Kind: NFRErrorScenarioValidation, Args: map[string]any{"filePath": "Unit1.pas", "line": "x", "column": 1}, Reason: "line type mismatch"},
		{Tool: "rename_symbol", Kind: NFRErrorScenarioValidation, Args: map[string]any{}, Reason: "missing required position/name"},
		{Tool: "workspace_symbols", Kind: NFRErrorScenarioValidation, Args: map[string]any{"query": 123}, Reason: "query type mismatch"},
		{Tool: "get_symbols_overview", Kind: NFRErrorScenarioValidation, Args: map[string]any{"query": 123}, Reason: "query type mismatch"},
		{Tool: "semantic_search", Kind: NFRErrorScenarioValidation, Args: map[string]any{"query": 123}, Reason: "query type mismatch"},
		{Tool: "code_actions", Kind: NFRErrorScenarioValidation, Args: map[string]any{"filePath": 1, "line": 1, "column": 1}, Reason: "filePath type mismatch"},
		{Tool: "replace_symbol_body", Kind: NFRErrorScenarioValidation, Args: map[string]any{}, Reason: "missing symbol payload"},
		{Tool: "insert_after_symbol", Kind: NFRErrorScenarioValidation, Args: map[string]any{}, Reason: "missing symbol payload"},
		{Tool: "insert_before_symbol", Kind: NFRErrorScenarioValidation, Args: map[string]any{}, Reason: "missing symbol payload"},
		{Tool: "safe_delete_symbol", Kind: NFRErrorScenarioValidation, Args: map[string]any{}, Reason: "missing filePath/symbolName"},
		{Tool: "run_query", Kind: NFRErrorScenarioValidation, Args: map[string]any{}, Reason: "missing query/node_type"},
		{Tool: "get_diagnostics_for_symbol", Kind: NFRErrorScenarioValidation, Args: map[string]any{"filePath": 1}, Reason: "filePath type mismatch"},
		{Tool: "dependency_tree", Kind: NFRErrorScenarioValidation, Args: map[string]any{"uri": "file:///u.pas", "direction": "sideways"}, Reason: "invalid direction enum"},
		{Tool: "graph_query", Kind: NFRErrorScenarioValidation, Args: map[string]any{"uri": fileURI, "direction": "sideways"}, Reason: "invalid direction enum"},
		{Tool: "memory_write", Kind: NFRErrorScenarioValidation, Args: map[string]any{"title": "", "content": ""}, Reason: "empty required memory fields"},
		{Tool: "memory_read", Kind: NFRErrorScenarioValidation, Args: map[string]any{}, Reason: "missing memory id/title"},
		{Tool: "memory_list", Kind: NFRErrorScenarioValidation, Args: map[string]any{"tag": 123}, Reason: "tag type mismatch"},
		{Tool: "onboarding", Kind: NFRErrorScenarioValidation, Args: map[string]any{"projectPath": ""}, Reason: "empty projectPath"},
		{Tool: "check_onboarding_performed", Kind: NFRErrorScenarioValidation, Args: map[string]any{"projectPath": ""}, Reason: "empty projectPath"},
		{Tool: "get_node_at_position", Kind: NFRErrorScenarioValidation, Args: map[string]any{"filePath": 1, "line": 1, "column": 1}, Reason: "filePath type mismatch"},

		// --- operational (LSP unavailable or execution failure) ---
		{Tool: "edit_file", Kind: NFRErrorScenarioOperational, Args: map[string]any{
			"filePath": fixturePath,
			"edits":    []map[string]any{{"startLine": 1, "endLine": 1, "newText": "x"}},
		}, Reason: "LSP guard when client nil"},
		{Tool: "definition", Kind: NFRErrorScenarioOperational, Args: map[string]any{"symbolName": "TargetSymbol"}, Reason: "LSP guard when client nil"},
		{Tool: "references", Kind: NFRErrorScenarioOperational, Args: map[string]any{"symbolName": "TargetSymbol"}, Reason: "LSP guard when client nil"},
		{Tool: "diagnostics", Kind: NFRErrorScenarioOperational, Args: map[string]any{"filePath": fixturePath}, Reason: "LSP guard when client nil"},
		{Tool: "hover", Kind: NFRErrorScenarioOperational, Args: map[string]any{"filePath": fixturePath, "line": 2.0, "column": 1.0}, Reason: "LSP guard when client nil"},
		{Tool: "rename_symbol", Kind: NFRErrorScenarioOperational, Args: map[string]any{
			"filePath": fixturePath, "line": 2.0, "column": 1.0, "newName": "Renamed",
		}, Reason: "LSP guard when client nil"},
		{Tool: "workspace_symbols", Kind: NFRErrorScenarioOperational, Args: map[string]any{"query": "TargetSymbol"}, Reason: "LSP guard when client nil"},
		{Tool: "get_symbols_overview", Kind: NFRErrorScenarioOperational, Args: map[string]any{"query": "TargetSymbol"}, Reason: "LSP guard when client nil"},
		{Tool: "semantic_search", Kind: NFRErrorScenarioOperational, Args: map[string]any{"query": "TargetSymbol"}, Reason: "LSP guard when client nil"},
		{Tool: "code_actions", Kind: NFRErrorScenarioOperational, Args: map[string]any{
			"filePath": fixturePath, "line": 2.0, "column": 1.0,
		}, Reason: "LSP guard when client nil"},
		{Tool: "replace_symbol_body", Kind: NFRErrorScenarioOperational, Args: map[string]any{
			"filePath": fixturePath, "symbolName": "TargetSymbolImpl", "newBody": "begin\nend;",
		}, Reason: "LSP guard when client nil"},
		{Tool: "insert_after_symbol", Kind: NFRErrorScenarioOperational, Args: map[string]any{
			"filePath": fixturePath, "symbolName": "TargetSymbolImpl", "text": "// x",
		}, Reason: "LSP guard when client nil"},
		{Tool: "insert_before_symbol", Kind: NFRErrorScenarioOperational, Args: map[string]any{
			"filePath": fixturePath, "symbolName": "TargetSymbolImpl", "text": "// x",
		}, Reason: "LSP guard when client nil"},
		{Tool: "safe_delete_symbol", Kind: NFRErrorScenarioOperational, Args: map[string]any{
			"filePath": fixturePath, "symbolName": "ZZZ_DEL_Only", "force": false,
		}, Reason: "LSP guard when client nil"},
		{Tool: "get_diagnostics_for_symbol", Kind: NFRErrorScenarioOperational, Args: map[string]any{
			"filePath": fixturePath, "symbolName": "TargetSymbol",
		}, Reason: "LSP guard when client nil"},
		{Tool: "run_query", Kind: NFRErrorScenarioOperational, Args: map[string]any{
			"node_type": "not_a_valid_node_type_xyz",
		}, Reason: "tree-sitter query execution failure"},
		{Tool: "dependency_tree", Kind: NFRErrorScenarioOperational, Args: map[string]any{"uri": "file:///nfr-force-lsp-error.pas"}, Reason: "LSP custom method failure", RequiresFakeLSP: true},
		{Tool: "graph_query", Kind: NFRErrorScenarioOperational, Args: map[string]any{"uri": "file:///nfr-force-lsp-error.pas"}, Reason: "LSP custom method failure", RequiresFakeLSP: true},
		{Tool: "memory_read", Kind: NFRErrorScenarioOperational, Args: map[string]any{"id": "does-not-exist-nfr"}, Reason: "memory not found domain failure"},
		{Tool: "memory_list", Kind: NFRErrorScenarioOperational, Args: map[string]any{"tag": "nonexistent-nfr-tag"}, Reason: "memory list domain failure"},
		{Tool: "onboarding", Kind: NFRErrorScenarioOperational, Args: map[string]any{
			"projectPath": filepath.Join(filepath.Dir(fixturePath), "nonexistent-onboarding-project-nfr"),
		}, Reason: "onboarding project path failure"},
		{Tool: "check_onboarding_performed", Kind: NFRErrorScenarioOperational, Args: map[string]any{
			"projectPath": filepath.Join(filepath.Dir(fixturePath), "nonexistent-check-onboarding-nfr"),
		}, Reason: "check onboarding path failure"},
		{Tool: "get_node_at_position", Kind: NFRErrorScenarioOperational, Args: map[string]any{
			"filePath": fixturePath, "line": 2.0, "column": 1.0,
		}, Reason: "LSP guard when client nil"},
	}
}

func toolNFRErrorScenariosByTool(fixturePath string) map[string]map[NFRErrorScenarioKind]ToolNFRErrorScenario {
	grouped := make(map[string]map[NFRErrorScenarioKind]ToolNFRErrorScenario)
	for _, scenario := range toolNFRErrorScenarios(fixturePath) {
		if grouped[scenario.Tool] == nil {
			grouped[scenario.Tool] = make(map[NFRErrorScenarioKind]ToolNFRErrorScenario)
		}
		grouped[scenario.Tool][scenario.Kind] = scenario
	}
	return grouped
}
