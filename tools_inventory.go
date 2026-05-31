package main

import "path/filepath"

// ToolKind classifies MCP tools for timeout/cancel inventory and NFR coverage.
type ToolKind string

const (
	ToolKindLocalOnly ToolKind = "local-only"
	ToolKindLSPBacked ToolKind = "lsp-backed"
)

// ToolInventoryEntry documents a public MCP tool for operational contracts.
type ToolInventoryEntry struct {
	Name     string
	Kind     ToolKind
	Mutating bool
	Critical bool
}

// registeredToolInventory is the authoritative matrix of public MCP tools.
// Critical LSP-backed tools must honor request ctx via handlerOperationContext.
func registeredToolInventory() []ToolInventoryEntry {
	return []ToolInventoryEntry{
		{Name: "edit_file", Kind: ToolKindLSPBacked, Mutating: true, Critical: true},
		{Name: "definition", Kind: ToolKindLSPBacked, Mutating: false, Critical: true},
		{Name: "references", Kind: ToolKindLSPBacked, Mutating: false, Critical: true},
		{Name: "diagnostics", Kind: ToolKindLSPBacked, Mutating: false, Critical: true},
		{Name: "hover", Kind: ToolKindLSPBacked, Mutating: false, Critical: true},
		{Name: "rename_symbol", Kind: ToolKindLSPBacked, Mutating: true, Critical: true},
		{Name: "workspace_symbols", Kind: ToolKindLSPBacked, Mutating: false, Critical: true},
		{Name: "get_symbols_overview", Kind: ToolKindLSPBacked, Mutating: false, Critical: true},
		{Name: "ast_summary", Kind: ToolKindLSPBacked, Mutating: false, Critical: false},
		{Name: "dependency_tree", Kind: ToolKindLSPBacked, Mutating: false, Critical: true},
		{Name: "graph_neighbors", Kind: ToolKindLSPBacked, Mutating: false, Critical: false},
		{Name: "graph_node", Kind: ToolKindLSPBacked, Mutating: false, Critical: false},
		{Name: "graph_query", Kind: ToolKindLSPBacked, Mutating: false, Critical: true},
		{Name: "call_graph", Kind: ToolKindLSPBacked, Mutating: false, Critical: false},
		{Name: "semantic_search", Kind: ToolKindLSPBacked, Mutating: false, Critical: true},
		{Name: "code_actions", Kind: ToolKindLSPBacked, Mutating: false, Critical: true},
		{Name: "replace_symbol_body", Kind: ToolKindLSPBacked, Mutating: true, Critical: true},
		{Name: "insert_after_symbol", Kind: ToolKindLSPBacked, Mutating: true, Critical: true},
		{Name: "insert_before_symbol", Kind: ToolKindLSPBacked, Mutating: true, Critical: true},
		{Name: "safe_delete_symbol", Kind: ToolKindLSPBacked, Mutating: true, Critical: true},
		{Name: "run_query", Kind: ToolKindLocalOnly, Mutating: false, Critical: true},
		{Name: "get_diagnostics_for_symbol", Kind: ToolKindLSPBacked, Mutating: false, Critical: true},
		{Name: "find_implementations", Kind: ToolKindLSPBacked, Mutating: false, Critical: false},
		{Name: "get_node_at_position", Kind: ToolKindLSPBacked, Mutating: false, Critical: true},
		{Name: "get_node_types", Kind: ToolKindLSPBacked, Mutating: false, Critical: false},
		{Name: "onboarding", Kind: ToolKindLocalOnly, Mutating: false, Critical: true},
		{Name: "check_onboarding_performed", Kind: ToolKindLocalOnly, Mutating: false, Critical: true},
		{Name: "analyze_complexity", Kind: ToolKindLocalOnly, Mutating: false, Critical: false},
		{Name: "find_similar_code", Kind: ToolKindLocalOnly, Mutating: false, Critical: false},
		{Name: "activate_project", Kind: ToolKindLocalOnly, Mutating: false, Critical: false},
		{Name: "build_query", Kind: ToolKindLocalOnly, Mutating: false, Critical: false},
		{Name: "adapt_query", Kind: ToolKindLocalOnly, Mutating: false, Critical: false},
		{Name: "memory_write", Kind: ToolKindLocalOnly, Mutating: true, Critical: true},
		{Name: "write_memory", Kind: ToolKindLocalOnly, Mutating: true, Critical: false},
		{Name: "memory_read", Kind: ToolKindLocalOnly, Mutating: false, Critical: true},
		{Name: "read_memory", Kind: ToolKindLocalOnly, Mutating: false, Critical: false},
		{Name: "memory_list", Kind: ToolKindLocalOnly, Mutating: false, Critical: true},
		{Name: "list_memories", Kind: ToolKindLocalOnly, Mutating: false, Critical: false},
		{Name: "memory_edit", Kind: ToolKindLocalOnly, Mutating: true, Critical: false},
		{Name: "edit_memory", Kind: ToolKindLocalOnly, Mutating: true, Critical: false},
		{Name: "memory_delete", Kind: ToolKindLocalOnly, Mutating: true, Critical: false},
		{Name: "delete_memory", Kind: ToolKindLocalOnly, Mutating: true, Critical: false},
	}
}

// lspBackedToolRequestContextCase drives cancel/timeout regression tests.
type lspBackedToolRequestContextCase struct {
	name string
	args map[string]any
	id   int
}

func lspBackedToolRequestContextCases(fixturePath string) []lspBackedToolRequestContextCase {
	fileURI := "file:///" + filepath.ToSlash(fixturePath)
	return []lspBackedToolRequestContextCase{
		{name: "definition", args: map[string]any{"symbolName": "TargetSymbol"}, id: 600},
		{name: "references", args: map[string]any{"symbolName": "TargetSymbol"}, id: 601},
		{name: "diagnostics", args: map[string]any{"filePath": fixturePath}, id: 640},
		{name: "hover", args: map[string]any{"filePath": fixturePath, "line": 2, "column": 11}, id: 641},
		{name: "semantic_search", args: map[string]any{"query": "TargetSymbol"}, id: 642},
		{name: "code_actions", args: map[string]any{"filePath": fixturePath, "line": 2, "column": 11}, id: 643},
		{name: "get_symbols_overview", args: map[string]any{"query": "TargetSymbol"}, id: 644},
		{name: "safe_delete_symbol", args: map[string]any{"filePath": fixturePath, "symbolName": "ZZZ_DEL_Only", "force": false}, id: 645},
		{name: "edit_file", args: map[string]any{
			"filePath": fixturePath,
			"edits": []map[string]any{
				{"startLine": 2, "endLine": 2, "newText": "interface"},
			},
		}, id: 646},
		{name: "rename_symbol", args: map[string]any{"filePath": fixturePath, "line": 2, "column": 11, "newName": "RenamedSymbol"}, id: 647},
		{name: "workspace_symbols", args: map[string]any{"query": "TargetSymbol"}, id: 648},
		{name: "ast_summary", args: map[string]any{"uri": fileURI}, id: 649},
		{name: "dependency_tree", args: map[string]any{"uri": fileURI}, id: 650},
		{name: "graph_neighbors", args: map[string]any{"uri": fileURI}, id: 651},
		{name: "graph_node", args: map[string]any{"uri": fileURI}, id: 652},
		{name: "graph_query", args: map[string]any{"uri": fileURI}, id: 653},
		{name: "call_graph", args: map[string]any{"symbolName": "TargetSymbol"}, id: 654},
		{name: "replace_symbol_body", args: map[string]any{
			"filePath": fixturePath, "symbolName": "TargetSymbolImpl", "newBody": "begin\nend;",
		}, id: 655},
		{name: "insert_after_symbol", args: map[string]any{
			"filePath": fixturePath, "symbolName": "TargetSymbolImpl", "text": "// inserted",
		}, id: 656},
		{name: "insert_before_symbol", args: map[string]any{
			"filePath": fixturePath, "symbolName": "TargetSymbolImpl", "text": "// inserted",
		}, id: 657},
		{name: "get_diagnostics_for_symbol", args: map[string]any{
			"filePath": fixturePath, "symbolName": "TargetSymbol",
		}, id: 658},
		{name: "find_implementations", args: map[string]any{
			"filePath": fixturePath, "symbolName": "IFoo",
		}, id: 659},
		{name: "get_node_at_position", args: map[string]any{
			"filePath": fixturePath, "line": 2, "column": 11,
		}, id: 660},
	}
}

// localPrimaryLSPMutators are LSP-backed tools whose critical path is local file I/O;
// they still honor request ctx at entry but are excluded from slow-LSP timeout tests.
var localPrimaryLSPMutators = map[string]struct{}{
	"edit_file":            {},
	"replace_symbol_body":  {},
	"insert_after_symbol":  {},
	"insert_before_symbol": {},
	"get_node_at_position": {},
	"get_node_types":       {},
}

func lspBackedToolSlowLSPTimeoutCases(fixturePath string) []lspBackedToolRequestContextCase {
	all := lspBackedToolRequestContextCases(fixturePath)
	filtered := make([]lspBackedToolRequestContextCase, 0, len(all))
	for _, tc := range all {
		if _, skip := localPrimaryLSPMutators[tc.name]; skip {
			continue
		}
		filtered = append(filtered, tc)
	}
	return filtered
}
