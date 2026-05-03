package main

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
)

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

		contextLines := 5 // default value
		if contextLinesArg, ok := request.Params.Arguments["contextLines"].(int); ok {
			contextLines = contextLinesArg
		}

		showLineNumbers := true // default value
		if showLineNumbersArg, ok := request.Params.Arguments["showLineNumbers"].(bool); ok {
			showLineNumbers = showLineNumbersArg
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
			direction, _ := req.Params.Arguments["direction"].(string)
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

	coreLogger.Info("Successfully registered all MCP tools")
	return nil
}
