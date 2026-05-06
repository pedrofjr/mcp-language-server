package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/isaacphi/mcp-language-server/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
)

func parseContextLinesArgument(raw any, defaultValue int) (int, error) {
	if raw == nil {
		return defaultValue, nil
	}

	switch value := raw.(type) {
	case bool:
		if value {
			return defaultValue, nil
		}
		return 0, nil
	case float64:
		if value != math.Trunc(value) {
			return 0, fmt.Errorf("contextLines must be a boolean or integer")
		}
		return int(value), nil
	case int:
		return value, nil
	default:
		return 0, fmt.Errorf("contextLines must be a boolean or integer")
	}
}

func parsePositiveIntegerArgument(raw any, argName string) (int, error) {
	if raw == nil {
		return 0, fmt.Errorf("%s must be an integer >= 1", argName)
	}

	switch value := raw.(type) {
	case float64:
		if value != math.Trunc(value) {
			return 0, fmt.Errorf("%s must be an integer >= 1", argName)
		}
		parsed := int(value)
		if parsed < 1 {
			return 0, fmt.Errorf("%s must be an integer >= 1", argName)
		}
		return parsed, nil
	case int:
		if value < 1 {
			return 0, fmt.Errorf("%s must be an integer >= 1", argName)
		}
		return value, nil
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return 0, fmt.Errorf("%s must be an integer >= 1", argName)
		}
		parsed, err := strconv.Atoi(trimmed)
		if err != nil || parsed < 1 {
			return 0, fmt.Errorf("%s must be an integer >= 1", argName)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("%s must be an integer >= 1", argName)
	}
}

func parseOptionalPositiveIntegerArgument(raw any, defaultValue int, argName string) (int, error) {
	if raw == nil {
		return defaultValue, nil
	}

	value, err := parsePositiveIntegerArgument(raw, argName)
	if err != nil {
		return 0, err
	}

	return value, nil
}

func collectWorkspaceDelphiCandidates(root string) ([]string, error) {
	collected := make([]string, 0)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		if d.IsDir() {
			name := strings.ToLower(d.Name())
			switch name {
			case ".git", "node_modules", "target", ".idea", ".vscode":
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		switch ext {
		case ".pas", ".dpr", ".dpk":
			collected = append(collected, filepath.Clean(path))
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(collected)
	return collected, nil
}

func runQueryCandidates(filePath string, strictFilePath bool) ([]string, error) {
	baseCandidates := []string{"internal/tools/query_builder.go", "tools.go"}
	seen := make(map[string]struct{}, len(baseCandidates)+1)
	candidates := make([]string, 0, len(baseCandidates)+1)
	trimmed := strings.TrimSpace(filePath)

	if strictFilePath && trimmed == "" {
		return nil, fmt.Errorf("strictFilePath=true requires filePath")
	}

	if trimmed != "" {
		cleaned := filepath.Clean(trimmed)
		info, err := os.Stat(cleaned)
		if err != nil {
			return nil, fmt.Errorf("filePath could not be read: %w", err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("filePath must point to a file: %s", cleaned)
		}
		seen[cleaned] = struct{}{}
		candidates = append(candidates, cleaned)

		if strictFilePath {
			return candidates, nil
		}
	}

	workspaceCandidates, err := collectWorkspaceDelphiCandidates(".")
	if err == nil {
		for _, candidate := range workspaceCandidates {
			if _, exists := seen[candidate]; exists {
				continue
			}
			seen[candidate] = struct{}{}
			candidates = append(candidates, candidate)
		}
	}

	for _, candidate := range baseCandidates {
		cleaned := filepath.Clean(candidate)
		if _, exists := seen[cleaned]; exists {
			continue
		}
		seen[cleaned] = struct{}{}
		candidates = append(candidates, cleaned)
	}

	return candidates, nil
}

func inferRunQueryNodeType(line string) string {
	lower := strings.ToLower(strings.TrimSpace(line))

	switch {
	case strings.HasPrefix(lower, "procedure "):
		return "procedure"
	case strings.HasPrefix(lower, "function "):
		return "function"
	case strings.HasPrefix(lower, "unit "):
		return "unit"
	case strings.HasPrefix(lower, "uses "):
		return "uses"
	case strings.HasPrefix(lower, "var ") || lower == "var":
		return "var"
	case strings.HasPrefix(lower, "const ") || lower == "const":
		return "const"
	case strings.HasPrefix(lower, "type ") || lower == "type":
		return "type"
	case strings.Contains(lower, "class"):
		return "class"
	default:
		return "unknown"
	}
}

func runQueryTextScan(query string, nodeType string, filePath string, strictFilePath bool, limit int) (string, error) {
	type runQueryMatch struct {
		FilePath    string `json:"filePath"`
		StartLine   int    `json:"startLine"`
		StartColumn int    `json:"startColumn"`
		EndLine     int    `json:"endLine"`
		EndColumn   int    `json:"endColumn"`
		NodeType    string `json:"nodeType"`
		Preview     string `json:"preview"`
		File        string `json:"file"`
		Line        int    `json:"line"`
		Text        string `json:"text"`
	}

	type runQueryResponse struct {
		Query        string          `json:"query"`
		TotalMatches int             `json:"totalMatches"`
		Matches      []runQueryMatch `json:"matches"`
	}

	needle := strings.TrimSpace(query)
	if needle == "" {
		needle = strings.TrimSpace(nodeType)
	}

	fileCandidates, err := runQueryCandidates(filePath, strictFilePath)
	if err != nil {
		return "", err
	}

	matches := make([]runQueryMatch, 0, limit)
	needleLower := strings.ToLower(needle)
	readableFiles := 0

	for _, candidate := range fileCandidates {
		if len(matches) >= limit {
			break
		}

		content, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		readableFiles++

		lines := strings.Split(string(content), "\n")
		for index, rawLine := range lines {
			if len(matches) >= limit {
				break
			}

			line := strings.TrimSpace(rawLine)
			if line == "" {
				continue
			}

			lowerRaw := strings.ToLower(rawLine)
			if strings.Contains(lowerRaw, needleLower) {
				byteOffset := strings.Index(lowerRaw, needleLower)
				if byteOffset < 0 {
					continue
				}

				startColumn := utf8.RuneCountInString(rawLine[:byteOffset]) + 1
				matchWidth := utf8.RuneCountInString(needle)
				if byteOffset+len(needle) <= len(rawLine) {
					matchedSlice := rawLine[byteOffset : byteOffset+len(needle)]
					matchWidth = utf8.RuneCountInString(matchedSlice)
				}
				if matchWidth < 1 {
					matchWidth = 1
				}
				endColumn := startColumn + matchWidth - 1

				matches = append(matches, runQueryMatch{
					FilePath:    candidate,
					StartLine:   index + 1,
					StartColumn: startColumn,
					EndLine:     index + 1,
					EndColumn:   endColumn,
					NodeType:    inferRunQueryNodeType(rawLine),
					Preview:     line,
					File:        candidate,
					Line:        index + 1,
					Text:        line,
				})
			}
		}
	}

	if readableFiles == 0 {
		return "", fmt.Errorf("no readable files available for run_query")
	}

	response := runQueryResponse{
		Query:        needle,
		TotalMatches: len(matches),
		Matches:      matches,
	}

	payload, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal run_query response: %w", err)
	}

	return string(payload), nil
}

func (s *mcpServer) registerTools() error {
	coreLogger.Debug("Registering MCP tools")

	applyTextEditTool := mcp.NewTool("edit_file",
		mcp.WithDescription("Apply multiple text edits to a file."),
		mcp.WithArray("edits",
			mcp.Required(),
			mcp.Description("List of edits to apply"),
			mcp.Items(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"startLine": map[string]any{
						"type":        "number",
						"description": "Start line to replace, inclusive, one-indexed",
					},
					"endLine": map[string]any{
						"type":        "number",
						"description": "End line to replace, inclusive, one-indexed",
					},
					"newText": map[string]any{
						"type":        "string",
						"description": "Replacement text. Replace with the new text. Leave blank to remove lines.",
					},
				},
				"required": []string{"startLine", "endLine"},
			}),
		),
		mcp.WithString("filePath",
			mcp.Required(),
			mcp.Description("Path to the file to edit"),
		),
	)

	s.mcpServer.AddTool(applyTextEditTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return mcp.NewToolResultError("filePath must be a string"), nil
		}

		// Extract edits array
		editsArg, ok := request.Params.Arguments["edits"]
		if !ok {
			return mcp.NewToolResultError("edits is required"), nil
		}

		// Type assert and convert the edits
		editsArray, ok := editsArg.([]any)
		if !ok {
			return mcp.NewToolResultError("edits must be an array"), nil
		}

		var edits []tools.TextEdit
		for _, editItem := range editsArray {
			editMap, ok := editItem.(map[string]any)
			if !ok {
				return mcp.NewToolResultError("each edit must be an object"), nil
			}

			startLine, ok := editMap["startLine"].(float64)
			if !ok {
				return mcp.NewToolResultError("startLine must be a number"), nil
			}

			endLine, ok := editMap["endLine"].(float64)
			if !ok {
				return mcp.NewToolResultError("endLine must be a number"), nil
			}

			newText, _ := editMap["newText"].(string) // newText can be empty

			edits = append(edits, tools.TextEdit{
				StartLine: int(startLine),
				EndLine:   int(endLine),
				NewText:   newText,
			})
		}

		coreLogger.Debug("Executing edit_file for file: %s", filePath)
		response, err := tools.ApplyTextEdits(s.ctx, s.lspClient, filePath, edits)
		if err != nil {
			coreLogger.Error("Failed to apply edits: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to apply edits: %v", err)), nil
		}
		return mcp.NewToolResultText(response), nil
	})

	readDefinitionTool := mcp.NewTool("definition",
		mcp.WithDescription("Read the source code definition of a symbol (function, type, constant, etc.) from the codebase. symbolName can be unqualified, but package/type/unit-qualified names may be required or resolve more precisely depending on the language server."),
		mcp.WithString("symbolName",
			mcp.Required(),
			mcp.Description("The symbol to resolve. Unqualified names often work, but package/type/unit-qualified names can be required or more precise depending on the language server (e.g. 'MyFunction', 'mypackage.MyFunction', 'MyType.MyMethod', 'UnitName.Symbol')."),
		),
	)

	s.mcpServer.AddTool(readDefinitionTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		symbolName, ok := request.Params.Arguments["symbolName"].(string)
		if !ok {
			return mcp.NewToolResultError("symbolName must be a string"), nil
		}

		coreLogger.Debug("Executing definition for symbol: %s", symbolName)
		text, err := tools.ReadDefinition(s.ctx, s.lspClient, symbolName)
		if err != nil {
			coreLogger.Error("Failed to get definition: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to get definition: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	findReferencesTool := mcp.NewTool("references",
		mcp.WithDescription("Find all usages and references of a symbol throughout the codebase. symbolName can be unqualified, but package/type/unit-qualified names may be required or resolve more precisely depending on the language server."),
		mcp.WithString("symbolName",
			mcp.Required(),
			mcp.Description("The symbol to search for. Unqualified names often work, but package/type/unit-qualified names can be required or more precise depending on the language server (e.g. 'MyFunction', 'mypackage.MyFunction', 'MyType.MyMethod', 'UnitName.Symbol')."),
		),
	)

	s.mcpServer.AddTool(findReferencesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		symbolName, ok := request.Params.Arguments["symbolName"].(string)
		if !ok {
			return mcp.NewToolResultError("symbolName must be a string"), nil
		}

		coreLogger.Debug("Executing references for symbol: %s", symbolName)
		text, err := tools.FindReferences(s.ctx, s.lspClient, symbolName)
		if err != nil {
			coreLogger.Error("Failed to find references: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to find references: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	getDiagnosticsTool := mcp.NewTool("diagnostics",
		mcp.WithDescription("Get diagnostic information for a specific file from the language server."),
		mcp.WithString("filePath",
			mcp.Required(),
			mcp.Description("The path to the file to get diagnostics for"),
		),
		mcp.WithBoolean("contextLines",
			mcp.Description("Lines to include around each diagnostic."),
			mcp.DefaultBool(false),
		),
		mcp.WithBoolean("showLineNumbers",
			mcp.Description("If true, adds line numbers to the output"),
			mcp.DefaultBool(true),
		),
	)

	s.mcpServer.AddTool(getDiagnosticsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return mcp.NewToolResultError("filePath must be a string"), nil
		}

		contextLines, err := parseContextLinesArgument(request.Params.Arguments["contextLines"], 5)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		showLineNumbers := true // default value
		if showLineNumbersArg, ok := request.Params.Arguments["showLineNumbers"].(bool); ok {
			showLineNumbers = showLineNumbersArg
		}

		if s.lspClient == nil {
			return mcp.NewToolResultError("lspClient not initialized"), nil
		}

		coreLogger.Debug("Executing diagnostics for file: %s", filePath)
		text, err := tools.GetDiagnosticsForFile(s.ctx, s.lspClient, filePath, contextLines, showLineNumbers)
		if err != nil {
			coreLogger.Error("Failed to get diagnostics: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to get diagnostics: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	// Uncomment to add codelens tools
	//
	// getCodeLensTool := mcp.NewTool("get_codelens",
	// 	mcp.WithDescription("Get code lens hints for a given file from the language server."),
	// 	mcp.WithString("filePath",
	// 		mcp.Required(),
	// 		mcp.Description("The path to the file to get code lens information for"),
	// 	),
	// )
	//
	// s.mcpServer.AddTool(getCodeLensTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 	// Extract arguments
	// 	filePath, ok := request.Params.Arguments["filePath"].(string)
	// 	if !ok {
	// 		return mcp.NewToolResultError("filePath must be a string"), nil
	// 	}
	//
	// 	coreLogger.Debug("Executing get_codelens for file: %s", filePath)
	// 	text, err := tools.GetCodeLens(s.ctx, s.lspClient, filePath)
	// 	if err != nil {
	// 		coreLogger.Error("Failed to get code lens: %v", err)
	// 		return mcp.NewToolResultError(fmt.Sprintf("failed to get code lens: %v", err)), nil
	// 	}
	// 	return mcp.NewToolResultText(text), nil
	// })
	//
	// executeCodeLensTool := mcp.NewTool("execute_codelens",
	// 	mcp.WithDescription("Execute a code lens command for a given file and lens index."),
	// 	mcp.WithString("filePath",
	// 		mcp.Required(),
	// 		mcp.Description("The path to the file containing the code lens to execute"),
	// 	),
	// 	mcp.WithNumber("index",
	// 		mcp.Required(),
	// 		mcp.Description("The index of the code lens to execute (from get_codelens output), 1 indexed"),
	// 	),
	// )
	//
	// s.mcpServer.AddTool(executeCodeLensTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 	// Extract arguments
	// 	filePath, ok := request.Params.Arguments["filePath"].(string)
	// 	if !ok {
	// 		return mcp.NewToolResultError("filePath must be a string"), nil
	// 	}
	//
	// 	// Handle both float64 and int for index due to JSON parsing
	// 	var index int
	// 	switch v := request.Params.Arguments["index"].(type) {
	// 	case float64:
	// 		index = int(v)
	// 	case int:
	// 		index = v
	// 	default:
	// 		return mcp.NewToolResultError("index must be a number"), nil
	// 	}
	//
	// 	coreLogger.Debug("Executing execute_codelens for file: %s index: %d", filePath, index)
	// 	text, err := tools.ExecuteCodeLens(s.ctx, s.lspClient, filePath, index)
	// 	if err != nil {
	// 		coreLogger.Error("Failed to execute code lens: %v", err)
	// 		return mcp.NewToolResultError(fmt.Sprintf("failed to execute code lens: %v", err)), nil
	// 	}
	// 	return mcp.NewToolResultText(text), nil
	// })

	hoverTool := mcp.NewTool("hover",
		mcp.WithDescription("Get hover information (type, documentation) for a symbol at the specified position."),
		mcp.WithString("filePath",
			mcp.Required(),
			mcp.Description("The path to the file to get hover information for"),
		),
		mcp.WithNumber("line",
			mcp.Required(),
			mcp.Description("The line number where the hover is requested (1-indexed)"),
		),
		mcp.WithNumber("column",
			mcp.Required(),
			mcp.Description("The column number where the hover is requested (1-indexed)"),
		),
	)

	s.mcpServer.AddTool(hoverTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return mcp.NewToolResultError("filePath must be a string"), nil
		}

		// Handle both float64 and int for line and column due to JSON parsing
		var line, column int
		switch v := request.Params.Arguments["line"].(type) {
		case float64:
			line = int(v)
		case int:
			line = v
		default:
			return mcp.NewToolResultError("line must be a number"), nil
		}

		switch v := request.Params.Arguments["column"].(type) {
		case float64:
			column = int(v)
		case int:
			column = v
		default:
			return mcp.NewToolResultError("column must be a number"), nil
		}

		coreLogger.Debug("Executing hover for file: %s line: %d column: %d", filePath, line, column)
		text, err := tools.GetHoverInfo(s.ctx, s.lspClient, filePath, line, column)
		if err != nil {
			coreLogger.Error("Failed to get hover information: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to get hover information: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	renameSymbolTool := mcp.NewTool("rename_symbol",
		mcp.WithDescription("Rename a symbol (variable, function, class, etc.) at the specified position and update all references throughout the codebase."),
		mcp.WithString("filePath",
			mcp.Required(),
			mcp.Description("The path to the file containing the symbol to rename"),
		),
		mcp.WithNumber("line",
			mcp.Required(),
			mcp.Description("The line number where the symbol is located (1-indexed)"),
		),
		mcp.WithNumber("column",
			mcp.Required(),
			mcp.Description("The column number where the symbol is located (1-indexed)"),
		),
		mcp.WithString("newName",
			mcp.Required(),
			mcp.Description("The new name for the symbol"),
		),
	)

	s.mcpServer.AddTool(renameSymbolTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return mcp.NewToolResultError("filePath must be a string"), nil
		}

		newName, ok := request.Params.Arguments["newName"].(string)
		if !ok {
			return mcp.NewToolResultError("newName must be a string"), nil
		}

		// Handle both float64 and int for line and column due to JSON parsing
		var line, column int
		switch v := request.Params.Arguments["line"].(type) {
		case float64:
			line = int(v)
		case int:
			line = v
		default:
			return mcp.NewToolResultError("line must be a number"), nil
		}

		switch v := request.Params.Arguments["column"].(type) {
		case float64:
			column = int(v)
		case int:
			column = v
		default:
			return mcp.NewToolResultError("column must be a number"), nil
		}

		coreLogger.Debug("Executing rename_symbol for file: %s line: %d column: %d newName: %s", filePath, line, column, newName)
		text, err := tools.RenameSymbol(s.ctx, s.lspClient, filePath, line, column, newName)
		if err != nil {
			coreLogger.Error("Failed to rename symbol: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to rename symbol: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	// workspace_symbols
	s.mcpServer.AddTool(
		mcp.NewTool("workspace_symbols",
			mcp.WithDescription("Search for symbols (types, functions, constants, variables) across the entire Delphi workspace by name or substring. Returns name, kind, and file location for each match."),
			mcp.WithString("query",
				mcp.Description("Substring to filter symbols (case-insensitive). Leave empty to list all exported symbols."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			queryRaw := req.Params.Arguments["query"]
			if queryRaw != nil {
				if _, ok := queryRaw.(string); !ok {
					return mcp.NewToolResultError("query must be a string"), nil
				}
			}
			query, _ := queryRaw.(string)
			result, err := tools.GetWorkspaceSymbols(s.ctx, s.lspClient, query)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed: %v", err)), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	// ast_summary
	s.mcpServer.AddTool(
		mcp.NewTool("ast_summary",
			mcp.WithDescription("Returns the interface structure of a Delphi unit (types, routines, constants, variables) without routine bodies. Useful for understanding unit API without reading the full source."),
			mcp.WithString("uri",
				mcp.Required(),
				mcp.Description("File URI of the Delphi .pas file (e.g. file:///path/to/Unit1.pas)"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, ok := req.Params.Arguments["uri"].(string)
			if !ok || uri == "" {
				return mcp.NewToolResultError("uri must be a non-empty string"), nil
			}
			result, err := tools.GetAstSummary(s.ctx, s.lspClient, uri)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed: %v", err)), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	// dependency_tree
	s.mcpServer.AddTool(
		mcp.NewTool("dependency_tree",
			mcp.WithDescription("Returns the dependency graph for a Delphi unit. Use direction='imports' to see what the unit depends on, or direction='importedBy' to see what depends on it. The response includes a 'tree' flat map and a 'treeBySection' map that splits each unit's dependencies into 'interface' and 'implementation' sections (only populated for direction='imports')."),
			mcp.WithString("uri",
				mcp.Required(),
				mcp.Description("File URI of the Delphi .pas file (e.g. file:///path/to/Unit1.pas)"),
			),
			mcp.WithString("direction",
				mcp.Description("'imports' (default) to show dependencies, or 'importedBy' for reverse dependencies"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, ok := req.Params.Arguments["uri"].(string)
			if !ok || uri == "" {
				return mcp.NewToolResultError("uri must be a non-empty string"), nil
			}
			direction := ""
			if directionRaw, exists := req.Params.Arguments["direction"]; exists && directionRaw != nil {
				directionValue, ok := directionRaw.(string)
				if !ok {
					return mcp.NewToolResultError("direction must be a string: 'imports' or 'importedBy'"), nil
				}
				direction = directionValue
			}
			if direction != "" && direction != "imports" && direction != "importedBy" {
				return mcp.NewToolResultError("direction must be 'imports' or 'importedBy'"), nil
			}
			result, err := tools.GetDependencyTree(s.ctx, s.lspClient, uri, direction)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed: %v", err)), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	// graph_neighbors
	s.mcpServer.AddTool(
		mcp.NewTool("graph_neighbors",
			mcp.WithDescription("Returns immediate graph neighbors for a Delphi unit node using relationType='uses_unit' and direction='imports'|'importedBy'."),
			mcp.WithString("uri",
				mcp.Required(),
				mcp.Description("File URI of the Delphi .pas file (e.g. file:///path/to/Unit1.pas)"),
			),
			mcp.WithString("relationType",
				mcp.Description("Relation kind. Only 'uses_unit' is currently supported."),
			),
			mcp.WithString("direction",
				mcp.Description("Neighborhood direction: 'imports' (default) or 'importedBy'."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, ok := req.Params.Arguments["uri"].(string)
			if !ok || strings.TrimSpace(uri) == "" {
				return mcp.NewToolResultError("uri must be a non-empty string"), nil
			}

			relationType := ""
			if relationTypeRaw, exists := req.Params.Arguments["relationType"]; exists && relationTypeRaw != nil {
				relationTypeValue, ok := relationTypeRaw.(string)
				if !ok {
					return mcp.NewToolResultError("relationType must be 'uses_unit'"), nil
				}
				relationType = strings.TrimSpace(relationTypeValue)
			}
			if relationType != "" && relationType != "uses_unit" {
				return mcp.NewToolResultError("relationType must be 'uses_unit'"), nil
			}

			direction := ""
			if directionRaw, exists := req.Params.Arguments["direction"]; exists && directionRaw != nil {
				directionValue, ok := directionRaw.(string)
				if !ok {
					return mcp.NewToolResultError("direction must be 'imports' or 'importedBy'"), nil
				}
				direction = strings.TrimSpace(directionValue)
			}
			if direction != "" && direction != "imports" && direction != "importedBy" {
				return mcp.NewToolResultError("direction must be 'imports' or 'importedBy'"), nil
			}

			result, err := tools.GetGraphNeighbors(s.ctx, s.lspClient, uri, relationType, direction)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed: %v", err)), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	// graph_node
	s.mcpServer.AddTool(
		mcp.NewTool("graph_node",
			mcp.WithDescription("Returns graph node relations for a Delphi unit node using relationType='uses_unit'."),
			mcp.WithString("uri",
				mcp.Required(),
				mcp.Description("File URI of the Delphi .pas file (e.g. file:///path/to/Unit1.pas)"),
			),
			mcp.WithString("relationType",
				mcp.Description("Relation kind. Only 'uses_unit' is currently supported."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, ok := req.Params.Arguments["uri"].(string)
			if !ok || strings.TrimSpace(uri) == "" {
				return mcp.NewToolResultError("uri must be a non-empty string"), nil
			}

			relationType := ""
			if relationTypeRaw, exists := req.Params.Arguments["relationType"]; exists && relationTypeRaw != nil {
				relationTypeValue, ok := relationTypeRaw.(string)
				if !ok {
					return mcp.NewToolResultError("relationType must be 'uses_unit'"), nil
				}
				relationType = strings.TrimSpace(relationTypeValue)
			}
			if relationType != "" && relationType != "uses_unit" {
				return mcp.NewToolResultError("relationType must be 'uses_unit'"), nil
			}

			result, err := tools.GetGraphNode(s.ctx, s.lspClient, uri, relationType)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed: %v", err)), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	// graph_query
	s.mcpServer.AddTool(
		mcp.NewTool("graph_query",
			mcp.WithDescription("Returns a bounded graph neighborhood for a Delphi unit using relationType='uses_unit', direction='imports'|'importedBy'|'both' and traversal depth."),
			mcp.WithString("uri",
				mcp.Required(),
				mcp.Description("File URI of the Delphi .pas file (e.g. file:///path/to/Unit1.pas)"),
			),
			mcp.WithString("relationType",
				mcp.Description("Relation kind. Only 'uses_unit' is currently supported."),
			),
			mcp.WithString("direction",
				mcp.Description("Traversal direction: 'imports', 'importedBy' or 'both' (default)."),
			),
			mcp.WithNumber("depth",
				mcp.Description("Traversal depth as an integer >= 0. Defaults to 1."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, ok := req.Params.Arguments["uri"].(string)
			if !ok || strings.TrimSpace(uri) == "" {
				return mcp.NewToolResultError("uri must be a non-empty string"), nil
			}

			relationType := ""
			if relationTypeRaw, exists := req.Params.Arguments["relationType"]; exists && relationTypeRaw != nil {
				relationTypeValue, ok := relationTypeRaw.(string)
				if !ok {
					return mcp.NewToolResultError("relationType must be 'uses_unit'"), nil
				}
				relationType = strings.TrimSpace(relationTypeValue)
			}
			if relationType != "" && relationType != "uses_unit" {
				return mcp.NewToolResultError("relationType must be 'uses_unit'"), nil
			}

			direction := ""
			if directionRaw, exists := req.Params.Arguments["direction"]; exists && directionRaw != nil {
				directionValue, ok := directionRaw.(string)
				if !ok {
					return mcp.NewToolResultError("direction must be 'imports', 'importedBy' or 'both'"), nil
				}
				direction = strings.TrimSpace(directionValue)
			}
			if direction != "" && direction != "imports" && direction != "importedBy" && direction != "both" {
				return mcp.NewToolResultError("direction must be 'imports', 'importedBy' or 'both'"), nil
			}

			depth := 1
			if depthRaw, exists := req.Params.Arguments["depth"]; exists && depthRaw != nil {
				var depthNumber float64
				switch v := depthRaw.(type) {
				case float64:
					depthNumber = v
				case int:
					depthNumber = float64(v)
				default:
					return mcp.NewToolResultError("depth must be an integer"), nil
				}

				if depthNumber != math.Trunc(depthNumber) {
					return mcp.NewToolResultError("depth must be an integer"), nil
				}
				if depthNumber < 0 {
					return mcp.NewToolResultError("depth must be greater than or equal to 0"), nil
				}

				depth = int(depthNumber)
			}

			result, err := tools.GetGraphQuery(s.ctx, s.lspClient, uri, relationType, direction, depth)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed: %v", err)), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	// call_graph
	s.mcpServer.AddTool(
		mcp.NewTool("call_graph",
			mcp.WithDescription("Returns the call graph for a symbol, including called routines and reverse callers, with optional traversal depth. Each edge in 'calls' and 'calledBy' arrays includes an 'isStub' boolean field that is true when the referenced unit is a built-in RTL/VCL stub (e.g. SysUtils, Classes, Graphics)."),
			mcp.WithString("symbolName",
				mcp.Required(),
				mcp.Description("Symbol to analyze in the call graph (e.g. Unit1.DoWork or TWorker.Execute)."),
			),
			mcp.WithNumber("depth",
				mcp.Description("Optional positive integer depth for traversal. Defaults to 1 when omitted."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			symbolName, ok := req.Params.Arguments["symbolName"].(string)
			if !ok {
				return mcp.NewToolResultError("symbolName must be a string"), nil
			}

			symbolName = strings.TrimSpace(symbolName)
			if symbolName == "" {
				return mcp.NewToolResultError("symbolName must be a non-empty string"), nil
			}

			depth := 1
			if depthRaw, exists := req.Params.Arguments["depth"]; exists && depthRaw != nil {
				var depthNumber float64
				switch v := depthRaw.(type) {
				case float64:
					depthNumber = v
				case int:
					depthNumber = float64(v)
				default:
					return mcp.NewToolResultError("depth must be a number"), nil
				}

				if depthNumber <= 0 {
					return mcp.NewToolResultError("depth must be a positive integer"), nil
				}
				if depthNumber != math.Trunc(depthNumber) {
					return mcp.NewToolResultError("depth must be an integer"), nil
				}

				depth = int(depthNumber)
			}

			result, err := tools.GetCallGraph(s.ctx, s.lspClient, symbolName, depth)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed: %v", err)), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	// semantic_search
	s.mcpServer.AddTool(
		mcp.NewTool("semantic_search",
			mcp.WithDescription("Search semantically relevant symbols and snippets using the language server semantic index."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query string. Must be non-empty after trim."),
			),
			mcp.WithString("scope",
				mcp.Description("Optional scope: 'workspace' (default) or 'file'."),
			),
			mcp.WithString("uri",
				mcp.Description("Required when scope='file'. File URI to restrict the semantic search."),
			),
			mcp.WithNumber("limit",
				mcp.Description("Optional positive integer result limit. Defaults to 20."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query, ok := req.Params.Arguments["query"].(string)
			if !ok || strings.TrimSpace(query) == "" {
				return mcp.NewToolResultError("query must be a non-empty string"), nil
			}

			scope := "workspace"
			if scopeRaw, exists := req.Params.Arguments["scope"]; exists && scopeRaw != nil {
				scopeText, ok := scopeRaw.(string)
				if !ok {
					return mcp.NewToolResultError("scope must be 'workspace' or 'file'"), nil
				}
				scopeText = strings.TrimSpace(scopeText)
				if scopeText != "" {
					scope = scopeText
				}
			}

			if scope != "workspace" && scope != "file" {
				return mcp.NewToolResultError("scope must be 'workspace' or 'file'"), nil
			}

			uri := ""
			if uriRaw, exists := req.Params.Arguments["uri"]; exists && uriRaw != nil {
				uriText, ok := uriRaw.(string)
				if !ok {
					return mcp.NewToolResultError("uri must be a string"), nil
				}
				uri = strings.TrimSpace(uriText)
			}

			if scope == "file" && uri == "" {
				return mcp.NewToolResultError("uri is required when scope='file'"), nil
			}

			limit := 20
			if limitRaw, exists := req.Params.Arguments["limit"]; exists && limitRaw != nil {
				var limitNumber float64
				switch v := limitRaw.(type) {
				case float64:
					limitNumber = v
				case int:
					limitNumber = float64(v)
				default:
					return mcp.NewToolResultError("limit must be a positive integer"), nil
				}

				if limitNumber <= 0 || limitNumber != math.Trunc(limitNumber) {
					return mcp.NewToolResultError("limit must be a positive integer"), nil
				}

				limit = int(limitNumber)
			}

			result, err := tools.GetSemanticSearch(s.ctx, s.lspClient, query, scope, uri, limit)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed: %v", err)), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	// code_actions
	s.mcpServer.AddTool(
		mcp.NewTool("code_actions",
			mcp.WithDescription("Get the available code actions (quick fixes, refactors, etc.) for a given position in a file."),
			mcp.WithString("filePath",
				mcp.Required(),
				mcp.Description("The path to the file to get code actions for"),
			),
			mcp.WithNumber("line",
				mcp.Required(),
				mcp.Description("The line number where the code actions are requested (1-indexed)"),
			),
			mcp.WithNumber("column",
				mcp.Required(),
				mcp.Description("The column number where the code actions are requested (1-indexed)"),
			),
			mcp.WithArray("only",
				mcp.Description("Optional filter for action kinds (e.g. [\"quickfix\"])"),
				mcp.Items(map[string]any{"type": "string"}),
			),
			mcp.WithBoolean("includeDiagnostics",
				mcp.Description("When true, attaches current file diagnostics to the code-action context"),
				mcp.DefaultBool(false),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filePath, ok := req.Params.Arguments["filePath"].(string)
			if !ok {
				return mcp.NewToolResultError("filePath must be a string"), nil
			}

			var line int
			switch v := req.Params.Arguments["line"].(type) {
			case float64:
				line = int(v)
			case int:
				line = v
			default:
				return mcp.NewToolResultError("line must be a number"), nil
			}

			var column int
			switch v := req.Params.Arguments["column"].(type) {
			case float64:
				column = int(v)
			case int:
				column = v
			default:
				return mcp.NewToolResultError("column must be a number"), nil
			}

			var only []string
			if onlyRaw, exists := req.Params.Arguments["only"]; exists && onlyRaw != nil {
				onlyArr, ok := onlyRaw.([]any)
				if !ok {
					return mcp.NewToolResultError("only must be an array of strings"), nil
				}
				for _, item := range onlyArr {
					s, ok := item.(string)
					if !ok {
						return mcp.NewToolResultError("only must be an array of strings"), nil
					}
					only = append(only, s)
				}
			}

			includeDiagnostics := false
			if inclRaw, exists := req.Params.Arguments["includeDiagnostics"]; exists && inclRaw != nil {
				inclBool, ok := inclRaw.(bool)
				if !ok {
					return mcp.NewToolResultError("includeDiagnostics must be a boolean"), nil
				}
				includeDiagnostics = inclBool
			}

			text, err := tools.GetCodeActions(s.ctx, s.lspClient, filePath, line, column, only, includeDiagnostics)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get code actions: %v", err)), nil
			}
			return mcp.NewToolResultText(text), nil
		},
	)

	// replace_symbol_body
	s.mcpServer.AddTool(
		mcp.NewTool("replace_symbol_body",
			mcp.WithDescription("Replace the begin..end body of a named Delphi symbol with new code."),
			mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute path to the .pas file")),
			mcp.WithString("symbolName", mcp.Required(), mcp.Description("Symbol name, e.g. 'TFoo.Bar' or 'Bar'")),
			mcp.WithString("newBody", mcp.Required(), mcp.Description("Replacement text for the begin..end block (include begin and end lines)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filePath, ok := req.Params.Arguments["filePath"].(string)
			if !ok {
				return mcp.NewToolResultError("filePath must be a string"), nil
			}
			symbolName, ok := req.Params.Arguments["symbolName"].(string)
			if !ok {
				return mcp.NewToolResultError("symbolName must be a string"), nil
			}
			newBody, ok := req.Params.Arguments["newBody"].(string)
			if !ok {
				return mcp.NewToolResultError("newBody must be a string"), nil
			}
			result, err := tools.ReplaceSymbolBody(s.ctx, s.lspClient, filePath, symbolName, newBody)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("replace_symbol_body failed: %v", err)), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	// insert_after_symbol
	s.mcpServer.AddTool(
		mcp.NewTool("insert_after_symbol",
			mcp.WithDescription("Insert code immediately after the end of a named Delphi symbol."),
			mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute path to the .pas file")),
			mcp.WithString("symbolName", mcp.Required(), mcp.Description("Symbol name, e.g. 'TFoo.Bar'")),
			mcp.WithString("text", mcp.Required(), mcp.Description("Text to insert after the symbol")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filePath, ok := req.Params.Arguments["filePath"].(string)
			if !ok {
				return mcp.NewToolResultError("filePath must be a string"), nil
			}
			symbolName, ok := req.Params.Arguments["symbolName"].(string)
			if !ok {
				return mcp.NewToolResultError("symbolName must be a string"), nil
			}
			text, ok := req.Params.Arguments["text"].(string)
			if !ok {
				return mcp.NewToolResultError("text must be a string"), nil
			}
			result, err := tools.InsertAfterSymbol(s.ctx, s.lspClient, filePath, symbolName, text)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("insert_after_symbol failed: %v", err)), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	// insert_before_symbol
	s.mcpServer.AddTool(
		mcp.NewTool("insert_before_symbol",
			mcp.WithDescription("Insert code immediately before a named Delphi symbol."),
			mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute path to the .pas file")),
			mcp.WithString("symbolName", mcp.Required(), mcp.Description("Symbol name, e.g. 'TFoo.Bar'")),
			mcp.WithString("text", mcp.Required(), mcp.Description("Text to insert before the symbol")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filePath, ok := req.Params.Arguments["filePath"].(string)
			if !ok {
				return mcp.NewToolResultError("filePath must be a string"), nil
			}
			symbolName, ok := req.Params.Arguments["symbolName"].(string)
			if !ok {
				return mcp.NewToolResultError("symbolName must be a string"), nil
			}
			text, ok := req.Params.Arguments["text"].(string)
			if !ok {
				return mcp.NewToolResultError("text must be a string"), nil
			}
			result, err := tools.InsertBeforeSymbol(s.ctx, s.lspClient, filePath, symbolName, text)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("insert_before_symbol failed: %v", err)), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	// === Sprint 2 - Memoria ===
	memoryWriteHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title, _ := request.Params.Arguments["title"].(string)
		content, _ := request.Params.Arguments["content"].(string)
		tagsStr, _ := request.Params.Arguments["tags"].(string)
		var tags []string
		if tagsStr != "" {
			for _, tag := range strings.Split(tagsStr, ",") {
				trimmed := strings.TrimSpace(tag)
				if trimmed != "" {
					tags = append(tags, trimmed)
				}
			}
		}

		id, err := tools.MemoryWrite(title, content, tags)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("Entrada criada com ID: " + id), nil
	}

	memoryReadHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, _ := request.Params.Arguments["id"].(string)
		entry, err := tools.MemoryRead(id)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("# %s\n\n%s\nTags: %s", entry.Title, entry.Content, strings.Join(entry.Tags, ", "))), nil
	}

	memoryListHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tag, _ := request.Params.Arguments["tag"].(string)
		entries, err := tools.MemoryList(tag)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(entries) == 0 {
			return mcp.NewToolResultText("Nenhuma entrada de memoria encontrada"), nil
		}

		var builder strings.Builder
		for _, entry := range entries {
			shortID := entry.ID
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}
			builder.WriteString(fmt.Sprintf("- [%s] %s (tags: %s)\n", shortID, entry.Title, strings.Join(entry.Tags, ", ")))
		}

		return mcp.NewToolResultText(builder.String()), nil
	}

	memoryEditHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, _ := request.Params.Arguments["id"].(string)
		content, _ := request.Params.Arguments["content"].(string)
		if err := tools.MemoryEdit(id, content); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("Entrada atualizada com sucesso"), nil
	}

	memoryDeleteHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, _ := request.Params.Arguments["id"].(string)
		if err := tools.MemoryDelete(id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("Entrada removida com sucesso"), nil
	}

	s.mcpServer.AddTool(mcp.NewTool("memory_write",
		mcp.WithDescription("Cria uma entrada de memoria persistente para o agente"),
		mcp.WithString("title", mcp.Required(), mcp.Description("Titulo da entrada")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Conteudo a memorizar")),
		mcp.WithString("tags", mcp.Description("Tags separadas por virgula")),
	), memoryWriteHandler)

	s.mcpServer.AddTool(mcp.NewTool("write_memory",
		mcp.WithDescription("Alias de memory_write"),
		mcp.WithString("title", mcp.Required(), mcp.Description("Titulo da entrada")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Conteudo a memorizar")),
		mcp.WithString("tags", mcp.Description("Tags separadas por virgula")),
	), memoryWriteHandler)

	s.mcpServer.AddTool(mcp.NewTool("memory_read",
		mcp.WithDescription("Le uma entrada de memoria pelo ID"),
		mcp.WithString("id", mcp.Required(), mcp.Description("ID da entrada")),
	), memoryReadHandler)

	s.mcpServer.AddTool(mcp.NewTool("read_memory",
		mcp.WithDescription("Alias de memory_read"),
		mcp.WithString("id", mcp.Required(), mcp.Description("ID da entrada")),
	), memoryReadHandler)

	s.mcpServer.AddTool(mcp.NewTool("memory_list",
		mcp.WithDescription("Lista entradas de memoria, opcionalmente filtradas por tag"),
		mcp.WithString("tag", mcp.Description("Filtrar por tag (opcional)")),
	), memoryListHandler)

	s.mcpServer.AddTool(mcp.NewTool("list_memories",
		mcp.WithDescription("Alias de memory_list"),
		mcp.WithString("tag", mcp.Description("Filtrar por tag (opcional)")),
	), memoryListHandler)

	s.mcpServer.AddTool(mcp.NewTool("memory_edit",
		mcp.WithDescription("Edita o conteudo de uma entrada de memoria"),
		mcp.WithString("id", mcp.Required(), mcp.Description("ID da entrada")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Novo conteudo")),
	), memoryEditHandler)

	s.mcpServer.AddTool(mcp.NewTool("edit_memory",
		mcp.WithDescription("Alias de memory_edit"),
		mcp.WithString("id", mcp.Required(), mcp.Description("ID da entrada")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Novo conteudo")),
	), memoryEditHandler)

	s.mcpServer.AddTool(mcp.NewTool("memory_delete",
		mcp.WithDescription("Remove uma entrada de memoria pelo ID"),
		mcp.WithString("id", mcp.Required(), mcp.Description("ID da entrada")),
	), memoryDeleteHandler)

	s.mcpServer.AddTool(mcp.NewTool("delete_memory",
		mcp.WithDescription("Alias de memory_delete"),
		mcp.WithString("id", mcp.Required(), mcp.Description("ID da entrada")),
	), memoryDeleteHandler)

	// === Sprint 2 - Edicao e Analise ===
	s.mcpServer.AddTool(mcp.NewTool("safe_delete_symbol",
		mcp.WithDescription("Remove um simbolo Delphi com verificacao de referencias"),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Caminho absoluto do arquivo .pas")),
		mcp.WithString("symbolName", mcp.Required(), mcp.Description("Nome do simbolo a remover")),
		mcp.WithBoolean("force", mcp.Description("true para remover mesmo com referencias existentes")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, _ := request.Params.Arguments["filePath"].(string)
		symbolName, _ := request.Params.Arguments["symbolName"].(string)
		force, _ := request.Params.Arguments["force"].(bool)

		result, err := tools.SafeDeleteSymbol(s.ctx, s.lspClient, filePath, symbolName, force)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(result), nil
	})

	// Sprint 3: analyze_complexity
	s.mcpServer.AddTool(
		mcp.NewTool("analyze_complexity",
			mcp.WithDescription("Analyze cyclomatic complexity of a Delphi symbol"),
			mcp.WithString("src",
				mcp.Required(),
				mcp.Description("Source code content"),
			),
			mcp.WithString("symbol_name",
				mcp.Required(),
				mcp.Description("Symbol name to analyze"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			src, ok := req.Params.Arguments["src"].(string)
			if !ok {
				return mcp.NewToolResultError("src must be a string"), nil
			}

			symbolName, ok := req.Params.Arguments["symbol_name"].(string)
			if !ok {
				return mcp.NewToolResultError("symbol_name must be a string"), nil
			}

			result := tools.AnalyzeComplexity(src, symbolName)
			if result == nil {
				return mcp.NewToolResultText("Symbol not found"), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf(`{"symbol":"%s","complexity":%d,"rating":"%s"}`,
				result.SymbolName, result.Score, result.Rating)), nil
		},
	)

	// Sprint 3: find_similar_code
	s.mcpServer.AddTool(
		mcp.NewTool("find_similar_code",
			mcp.WithDescription("Find code blocks similar to a query using Jaccard similarity"),
			mcp.WithString("src",
				mcp.Required(),
				mcp.Description("Source code content"),
			),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Code snippet to search for"),
			),
			mcp.WithNumber("threshold",
				mcp.Description("Similarity threshold [0.0, 1.0], default 0.3"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			src, ok := req.Params.Arguments["src"].(string)
			if !ok {
				return mcp.NewToolResultError("src must be a string"), nil
			}

			query, ok := req.Params.Arguments["query"].(string)
			if !ok {
				return mcp.NewToolResultError("query must be a string"), nil
			}

			threshold := 0.3
			if raw, exists := req.Params.Arguments["threshold"]; exists && raw != nil {
				switch value := raw.(type) {
				case float64:
					threshold = value
				case int:
					threshold = float64(value)
				default:
					return mcp.NewToolResultError("threshold must be a number"), nil
				}
			}

			blocks := tools.FindSimilarCode(src, query, threshold)
			data, err := json.Marshal(blocks)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to marshal similar blocks: %v", err)), nil
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// Sprint 3: activate_project
	s.mcpServer.AddTool(
		mcp.NewTool("activate_project",
			mcp.WithDescription("Set the active Delphi project directory"),
			mcp.WithString("dir",
				mcp.Required(),
				mcp.Description("Path to the Delphi project directory"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			dir, ok := req.Params.Arguments["dir"].(string)
			if !ok {
				return mcp.NewToolResultError("dir must be a string"), nil
			}

			result, err := tools.ActivateProject(dir)
			if err != nil {
				return mcp.NewToolResultError("error: " + err.Error()), nil
			}
			return mcp.NewToolResultText(result), nil
		},
	)

	// Sprint 3: build_query
	s.mcpServer.AddTool(
		mcp.NewTool("build_query",
			mcp.WithDescription("Build a tree-sitter query for a Delphi node type and symbol"),
			mcp.WithString("node_type",
				mcp.Required(),
				mcp.Description("Tree-sitter node type"),
			),
			mcp.WithString("symbol",
				mcp.Description("Optional symbol name to filter"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			nodeType, ok := req.Params.Arguments["node_type"].(string)
			if !ok {
				return mcp.NewToolResultError("node_type must be a string"), nil
			}

			symbol := ""
			if raw, exists := req.Params.Arguments["symbol"]; exists && raw != nil {
				value, ok := raw.(string)
				if !ok {
					return mcp.NewToolResultError("symbol must be a string"), nil
				}
				symbol = value
			}

			return mcp.NewToolResultText(tools.BuildQuery(nodeType, symbol)), nil
		},
	)

	// Sprint 3: adapt_query
	s.mcpServer.AddTool(
		mcp.NewTool("adapt_query",
			mcp.WithDescription("Adapt a tree-sitter query to a different Pascal dialect"),
			mcp.WithString("base",
				mcp.Required(),
				mcp.Description("Base query string"),
			),
			mcp.WithString("dialect",
				mcp.Required(),
				mcp.Description("Target dialect: delphi6, pascal, fpc"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			base, ok := req.Params.Arguments["base"].(string)
			if !ok {
				return mcp.NewToolResultError("base must be a string"), nil
			}

			dialect, ok := req.Params.Arguments["dialect"].(string)
			if !ok {
				return mcp.NewToolResultError("dialect must be a string"), nil
			}

			return mcp.NewToolResultText(tools.AdaptQuery(base, dialect)), nil
		},
	)

	// Sprint 3: run_query
	s.mcpServer.AddTool(
		mcp.NewTool("run_query",
			mcp.WithDescription("Run a minimal deterministic textual query scan and return JSON results."),
			mcp.WithString("query",
				mcp.Description("Text query used to match source lines."),
			),
			mcp.WithString("node_type",
				mcp.Description("Fallback node type when query is omitted."),
			),
			mcp.WithString("filePath",
				mcp.Description("Optional file path to scan first. Must point to an existing file when provided."),
			),
			mcp.WithBoolean("strictFilePath",
				mcp.Description("When true, requires filePath and scans only that file with no fallback."),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of matches to return. Must be > 0."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query := ""
			if raw, exists := req.Params.Arguments["query"]; exists && raw != nil {
				value, ok := raw.(string)
				if !ok {
					return mcp.NewToolResultError("query must be a string"), nil
				}
				query = strings.TrimSpace(value)
			}

			nodeType := ""
			if raw, exists := req.Params.Arguments["node_type"]; exists && raw != nil {
				value, ok := raw.(string)
				if !ok {
					return mcp.NewToolResultError("node_type must be a string"), nil
				}
				nodeType = strings.TrimSpace(value)
			}

			if query == "" && nodeType == "" {
				return mcp.NewToolResultError("query or node_type must be provided"), nil
			}

			filePath := ""
			if raw, exists := req.Params.Arguments["filePath"]; exists && raw != nil {
				value, ok := raw.(string)
				if !ok {
					return mcp.NewToolResultError("filePath must be a string"), nil
				}
				filePath = value
			}

			strictFilePath := false
			if raw, exists := req.Params.Arguments["strictFilePath"]; exists && raw != nil {
				value, ok := raw.(bool)
				if !ok {
					return mcp.NewToolResultError("strictFilePath must be a boolean"), nil
				}
				strictFilePath = value
			}

			if strictFilePath && strings.TrimSpace(filePath) == "" {
				return mcp.NewToolResultError("strictFilePath=true requires filePath"), nil
			}

			limit, err := parseOptionalPositiveIntegerArgument(req.Params.Arguments["limit"], 20, "limit")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			result, err := runQueryTextScan(query, nodeType, filePath, strictFilePath, limit)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed: %v", err)), nil
			}

			return mcp.NewToolResultText(result), nil
		},
	)
	s.mcpServer.AddTool(mcp.NewTool("get_diagnostics_for_symbol",
		mcp.WithDescription("Retorna diagnosticos do LSP relevantes para um simbolo"),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Caminho absoluto do arquivo .pas")),
		mcp.WithString("symbolName", mcp.Required(), mcp.Description("Nome do simbolo")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, _ := request.Params.Arguments["filePath"].(string)
		symbolName, _ := request.Params.Arguments["symbolName"].(string)

		result, err := tools.GetDiagnosticsForSymbol(s.ctx, s.lspClient, filePath, symbolName)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(result), nil
	})

	s.mcpServer.AddTool(mcp.NewTool("find_implementations",
		mcp.WithDescription("Encontra classes que implementam uma interface Delphi"),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Arquivo onde a interface esta declarada")),
		mcp.WithString("symbolName", mcp.Required(), mcp.Description("Nome da interface")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, _ := request.Params.Arguments["filePath"].(string)
		symbolName, _ := request.Params.Arguments["symbolName"].(string)

		result, err := tools.FindImplementations(s.ctx, s.lspClient, filePath, symbolName, s.config.workspaceDir)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(result), nil
	})

	s.mcpServer.AddTool(mcp.NewTool("get_node_at_position",
		mcp.WithDescription("Retorna o token e contexto textual em uma posicao do arquivo"),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Caminho absoluto do arquivo")),
		mcp.WithNumber("line", mcp.Required(), mcp.Description("Numero de linha (1-indexado)")),
		mcp.WithNumber("column", mcp.Required(), mcp.Description("Numero de coluna (1-indexado)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, _ := request.Params.Arguments["filePath"].(string)
		line, err := parsePositiveIntegerArgument(request.Params.Arguments["line"], "line")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		column, err := parsePositiveIntegerArgument(request.Params.Arguments["column"], "column")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, err := tools.GetNodeAtPosition(s.ctx, s.lspClient, filePath, line, column)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(result), nil
	})

	s.mcpServer.AddTool(mcp.NewTool("get_node_types",
		mcp.WithDescription("Retorna a lista de tipos de no suportados pelo Delphi 6"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := tools.GetNodeTypes(s.ctx, s.lspClient)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(result), nil
	})

	s.mcpServer.AddTool(mcp.NewTool("onboarding",
		mcp.WithDescription("Escaneia estrutura do projeto Delphi e retorna inventario de units, forms e entry point"),
		mcp.WithString("projectPath", mcp.Required(), mcp.Description("Caminho absoluto do diretorio do projeto")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectPath, _ := request.Params.Arguments["projectPath"].(string)
		result, err := tools.PerformOnboarding(projectPath)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(result), nil
	})

	s.mcpServer.AddTool(mcp.NewTool("check_onboarding_performed",
		mcp.WithDescription("Verifica se o onboarding ja foi executado para este projeto"),
		mcp.WithString("projectPath", mcp.Required(), mcp.Description("Caminho absoluto do diretorio do projeto")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectPath, _ := request.Params.Arguments["projectPath"].(string)
		performed, at := tools.CheckOnboardingPerformed(projectPath)
		if !performed {
			return mcp.NewToolResultText("Onboarding ainda nao foi executado para este projeto"), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Onboarding executado em: %s", at.Format(time.RFC3339))), nil
	})

	coreLogger.Info("Successfully registered all MCP tools")
	return nil
}
