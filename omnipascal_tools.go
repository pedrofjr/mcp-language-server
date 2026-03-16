package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/isaacphi/mcp-language-server/internal/omnipascal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
)

func (s *mcpServer) registerOmniPascalTools() error {
	coreLogger.Debug("Registering OmniPascal MCP tools")

	applyTextEditTool := mcp.NewTool("edit_file",
		mcp.WithDescription("Apply multiple text edits to a file and resynchronize the OmniPascal backend with the saved content."),
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
						"description": "Replacement text. Leave blank to remove lines.",
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
		filePath, err := requireStringArg(request.Params.Arguments, "filePath")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		edits, err := parseTextEditsArg(request.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		response, err := tools.OmniPascalApplyTextEdits(s.ctx, s.omniClient, filePath, edits)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to apply edits: %v", err)), nil
		}

		return mcp.NewToolResultText(response), nil
	})

	// omnipascal_set_config: desabilitado — setConfig é executado automaticamente no init.
	// Para reativar, descomente o bloco abaixo.
	/*
	setConfigTool := mcp.NewTool("omnipascal_set_config",
		mcp.WithDescription("Execute the OmniPascal setConfig command. Use configJson for arbitrary settings or provide the common Delphi settings directly."),
		mcp.WithString("configJson", mcp.Description("Optional raw JSON object with OmniPascal configuration values.")),
		mcp.WithString("workspacePaths", mcp.Description("Semicolon-separated workspace paths.")),
		mcp.WithString("delphiInstallationPath", mcp.Description("Path to the Delphi installation.")),
		mcp.WithString("freePascalSourcePath", mcp.Description("Path to the Free Pascal source files.")),
		mcp.WithString("defaultDevelopmentEnvironment", mcp.Description("Delphi or FreePascal.")),
		mcp.WithString("searchPath", mcp.Description("Additional search path list separated by semicolons.")),
		mcp.WithString("msbuildPath", mcp.Description("Full path to MSBuild.exe.")),
		mcp.WithString("lazbuildPath", mcp.Description("Full path to lazbuild.exe.")),
		mcp.WithBoolean("createBuildScripts", mcp.Description("Whether OmniPascal should create build scripts.")),
		mcp.WithString("symbolIndex", mcp.Description("off, workspace, or searchPath.")),
		mcp.WithString("usesListStyle", mcp.Description("multipleItemsPerLine or oneItemPerLine.")),
		mcp.WithString("namingConventionString", mcp.Description("pascalCase or camelCase.")),
	)

	s.mcpServer.AddTool(setConfigTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		config, err := buildOmniPascalConfig(request.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		response, err := tools.OmniPascalSetConfig(s.ctx, s.omniClient, config)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to synchronize config: %v", err)), nil
		}

		return mcp.NewToolResultText(response), nil
	})
	*/

	// omnipascal_open / omnipascal_close: desabilitados — buffer sync gerenciado internamente.
	/*
	openTool := mcp.NewTool("omnipascal_open",
		mcp.WithDescription("Execute the OmniPascal open command for a file."),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute or workspace-relative file path.")),
	)

	s.mcpServer.AddTool(openTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, err := requireStringArg(request.Params.Arguments, "filePath")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		response, err := tools.OmniPascalOpenFile(s.ctx, s.omniClient, filePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to open file: %v", err)), nil
		}
		return mcp.NewToolResultText(response), nil
	})

	closeTool := mcp.NewTool("omnipascal_close",
		mcp.WithDescription("Execute the OmniPascal close command for a file."),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute or workspace-relative file path.")),
	)

	s.mcpServer.AddTool(closeTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, err := requireStringArg(request.Params.Arguments, "filePath")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		response, err := tools.OmniPascalCloseFile(s.ctx, s.omniClient, filePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to close file: %v", err)), nil
		}
		return mcp.NewToolResultText(response), nil
	})
	*/

	// omnipascal_change: desabilitado — buffer sync gerenciado internamente.
	/*
	changeTool := mcp.NewTool("omnipascal_change",
		mcp.WithDescription("Execute the OmniPascal change command. The insertString is plain text and will be encoded before sending."),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("File path to update.")),
		mcp.WithNumber("startLine", mcp.Required(), mcp.Description("Start line, one-indexed.")),
		mcp.WithNumber("startColumn", mcp.Required(), mcp.Description("Start column, one-indexed.")),
		mcp.WithNumber("endLine", mcp.Required(), mcp.Description("End line, one-indexed.")),
		mcp.WithNumber("endColumn", mcp.Required(), mcp.Description("End column, one-indexed.")),
		mcp.WithString("insertString", mcp.Required(), mcp.Description("Replacement content before URI encoding.")),
	)

	s.mcpServer.AddTool(changeTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, err := requireStringArg(request.Params.Arguments, "filePath")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		startLine, err := requireIntArg(request.Params.Arguments, "startLine")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		startColumn, err := requireIntArg(request.Params.Arguments, "startColumn")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		endLine, err := requireIntArg(request.Params.Arguments, "endLine")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		endColumn, err := requireIntArg(request.Params.Arguments, "endColumn")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		insertString, err := requireStringArg(request.Params.Arguments, "insertString")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		response, err := tools.OmniPascalChangeFile(s.ctx, s.omniClient, omnipascal.ChangeArgs{
			File:         filePath,
			Line:         startLine,
			Offset:       startColumn,
			EndLine:      endLine,
			EndOffset:    endColumn,
			InsertString: insertString,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to send change: %v", err)), nil
		}
		return mcp.NewToolResultText(response), nil
	})
	*/

	getErrTool := mcp.NewTool("omnipascal_geterr",
		mcp.WithDescription("Execute the OmniPascal geterr command and return the latest cached diagnostics for the requested files."),
		mcp.WithArray("filePaths",
			mcp.Required(),
			mcp.Description("Files to diagnose."),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithNumber("delayMs", mcp.Description("Delay argument forwarded to OmniPascal."), mcp.DefaultNumber(0)),
		mcp.WithNumber("waitMs", mcp.Description("Time to wait locally for diagnostic events before reading the cache."), mcp.DefaultNumber(1500)),
	)

	s.mcpServer.AddTool(getErrTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePaths, err := requireStringArrayArg(request.Params.Arguments, "filePaths")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		delayMs := intArgOrDefault(request.Params.Arguments, "delayMs", 0)
		waitMs := intArgOrDefault(request.Params.Arguments, "waitMs", 1500)

		response, err := tools.OmniPascalGetDiagnostics(s.ctx, s.omniClient, filePaths, delayMs, waitMs)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get diagnostics: %v", err)), nil
		}
		return mcp.NewToolResultText(response), nil
	})

	completionsTool := mcp.NewTool("omnipascal_completions",
		mcp.WithDescription("Execute the OmniPascal completions command."),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("File path.")),
		mcp.WithNumber("line", mcp.Required(), mcp.Description("Line, one-indexed.")),
		mcp.WithNumber("column", mcp.Required(), mcp.Description("Column, one-indexed.")),
	)

	s.addOmniPascalPositionTool(completionsTool, func(filePath string, line, column int) (string, error) {
		return tools.OmniPascalCompletions(s.ctx, s.omniClient, filePath, line, column)
	})

	definitionTool := mcp.NewTool("omnipascal_definition",
		mcp.WithDescription("Execute the OmniPascal definition command."),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("File path.")),
		mcp.WithNumber("line", mcp.Required(), mcp.Description("Line, one-indexed.")),
		mcp.WithNumber("column", mcp.Required(), mcp.Description("Column, one-indexed.")),
	)

	s.addOmniPascalPositionTool(definitionTool, func(filePath string, line, column int) (string, error) {
		return tools.OmniPascalDefinition(s.ctx, s.omniClient, filePath, line, column)
	})

	quickInfoTool := mcp.NewTool("omnipascal_quickinfo",
		mcp.WithDescription("Execute the OmniPascal quickinfo command."),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("File path.")),
		mcp.WithNumber("line", mcp.Required(), mcp.Description("Line, one-indexed.")),
		mcp.WithNumber("column", mcp.Required(), mcp.Description("Column, one-indexed.")),
	)

	s.addOmniPascalPositionTool(quickInfoTool, func(filePath string, line, column int) (string, error) {
		return tools.OmniPascalQuickInfo(s.ctx, s.omniClient, filePath, line, column)
	})

	signatureHelpTool := mcp.NewTool("omnipascal_signature_help",
		mcp.WithDescription("Execute the OmniPascal signatureHelp command."),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("File path.")),
		mcp.WithNumber("line", mcp.Required(), mcp.Description("Line, one-indexed.")),
		mcp.WithNumber("column", mcp.Required(), mcp.Description("Column, one-indexed.")),
	)

	s.addOmniPascalPositionTool(signatureHelpTool, func(filePath string, line, column int) (string, error) {
		return tools.OmniPascalSignatureHelp(s.ctx, s.omniClient, filePath, line, column)
	})

	// omnipascal_document_symbol / omnipascal_workspace_symbol / omnipascal_get_project_files:
	// desabilitados — outline, busca global e helper de UI fora do escopo atual.
	/*
	documentSymbolTool := mcp.NewTool("omnipascal_document_symbol",
		mcp.WithDescription("Execute the OmniPascal textDocument/documentSymbol command."),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("File path.")),
	)

	s.mcpServer.AddTool(documentSymbolTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, err := requireStringArg(request.Params.Arguments, "filePath")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		response, err := tools.OmniPascalDocumentSymbols(s.ctx, s.omniClient, filePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get document symbols: %v", err)), nil
		}
		return mcp.NewToolResultText(response), nil
	})

	workspaceSymbolTool := mcp.NewTool("omnipascal_workspace_symbol",
		mcp.WithDescription("Execute the OmniPascal workspace/symbol command."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Workspace symbol query.")),
	)

	s.mcpServer.AddTool(workspaceSymbolTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := requireStringArg(request.Params.Arguments, "query")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		response, err := tools.OmniPascalWorkspaceSymbols(s.ctx, s.omniClient, query)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get workspace symbols: %v", err)), nil
		}
		return mcp.NewToolResultText(response), nil
	})

	getProjectFilesTool := mcp.NewTool("omnipascal_get_project_files",
		mcp.WithDescription("Execute the OmniPascal getProjectFiles command."),
		mcp.WithString("filePath", mcp.Description("Optional current file path used by OmniPascal to resolve project context.")),
	)

	s.mcpServer.AddTool(getProjectFilesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath := stringArgOrEmpty(request.Params.Arguments, "filePath")
		response, err := tools.OmniPascalGetProjectFiles(s.ctx, s.omniClient, filePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get project files: %v", err)), nil
		}
		return mcp.NewToolResultText(response), nil
	})
	*/

	loadProjectTool := mcp.NewTool("omnipascal_load_project",
		mcp.WithDescription("Execute the OmniPascal loadProject command."),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Project file to load.")),
	)

	s.mcpServer.AddTool(loadProjectTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, err := requireStringArg(request.Params.Arguments, "filePath")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		response, err := tools.OmniPascalLoadProject(s.ctx, s.omniClient, filePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to load project: %v", err)), nil
		}
		return mcp.NewToolResultText(response), nil
	})

	getAllUnitsTool := mcp.NewTool("omnipascal_get_all_units",
		mcp.WithDescription("Execute the OmniPascal getAllUnits command."),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Current file path.")),
	)

	s.mcpServer.AddTool(getAllUnitsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, err := requireStringArg(request.Params.Arguments, "filePath")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		response, err := tools.OmniPascalGetAllUnits(s.ctx, s.omniClient, filePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get units: %v", err)), nil
		}
		return mcp.NewToolResultText(response), nil
	})

	possibleUsesSectionsTool := mcp.NewTool("omnipascal_get_possible_uses_sections",
		mcp.WithDescription("Execute the OmniPascal getPossibleUsesSections command."),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Current file path.")),
	)

	s.mcpServer.AddTool(possibleUsesSectionsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, err := requireStringArg(request.Params.Arguments, "filePath")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		response, err := tools.OmniPascalGetPossibleUsesSections(s.ctx, s.omniClient, filePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get uses sections: %v", err)), nil
		}
		return mcp.NewToolResultText(response), nil
	})

	addUsesTool := mcp.NewTool("omnipascal_add_uses",
		mcp.WithDescription("Execute the OmniPascal addUses command."),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Current file path.")),
		mcp.WithString("usesSection", mcp.Required(), mcp.Description("Selected uses section.")),
		mcp.WithString("unitToAdd", mcp.Required(), mcp.Description("Unit to insert.")),
		mcp.WithBoolean("applyChanges", mcp.Description("Apply the returned text changes to disk."), mcp.DefaultBool(true)),
	)

	s.mcpServer.AddTool(addUsesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, err := requireStringArg(request.Params.Arguments, "filePath")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		usesSection, err := requireStringArg(request.Params.Arguments, "usesSection")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		unitToAdd, err := requireStringArg(request.Params.Arguments, "unitToAdd")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		applyChanges := boolArgOrDefault(request.Params.Arguments, "applyChanges", true)

		response, err := tools.OmniPascalAddUses(s.ctx, s.omniClient, filePath, usesSection, unitToAdd, applyChanges)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to add uses entry: %v", err)), nil
		}
		return mcp.NewToolResultText(response), nil
	})

	getCodeActionsTool := mcp.NewTool("omnipascal_get_code_actions",
		mcp.WithDescription("Execute the OmniPascal getCodeActions command."),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Current file path.")),
		mcp.WithNumber("startLine", mcp.Required(), mcp.Description("Selection start line, one-indexed.")),
		mcp.WithNumber("startColumn", mcp.Required(), mcp.Description("Selection start column, one-indexed.")),
		mcp.WithNumber("endLine", mcp.Required(), mcp.Description("Selection end line, one-indexed.")),
		mcp.WithNumber("endColumn", mcp.Required(), mcp.Description("Selection end column, one-indexed.")),
	)

	s.mcpServer.AddTool(getCodeActionsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, err := requireStringArg(request.Params.Arguments, "filePath")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		selection, err := spanFromArgs(request.Params.Arguments, "startLine", "startColumn", "endLine", "endColumn")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		response, err := tools.OmniPascalGetCodeActions(s.ctx, s.omniClient, filePath, selection)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get code actions: %v", err)), nil
		}
		return mcp.NewToolResultText(response), nil
	})

	runCodeActionTool := mcp.NewTool("omnipascal_run_code_action",
		mcp.WithDescription("Execute the OmniPascal runCodeAction command."),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Current file path.")),
		mcp.WithString("identifier", mcp.Required(), mcp.Description("Code action identifier/command returned by getCodeActions.")),
		mcp.WithNumber("startLine", mcp.Required(), mcp.Description("Selection start line, one-indexed.")),
		mcp.WithNumber("startColumn", mcp.Required(), mcp.Description("Selection start column, one-indexed.")),
		mcp.WithNumber("endLine", mcp.Required(), mcp.Description("Selection end line, one-indexed.")),
		mcp.WithNumber("endColumn", mcp.Required(), mcp.Description("Selection end column, one-indexed.")),
		mcp.WithBoolean("wantsTextChanges", mcp.Description("Request text changes from OmniPascal."), mcp.DefaultBool(true)),
		mcp.WithBoolean("applyChanges", mcp.Description("Apply the returned text changes to disk."), mcp.DefaultBool(true)),
	)

	s.mcpServer.AddTool(runCodeActionTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, err := requireStringArg(request.Params.Arguments, "filePath")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		identifier, err := requireStringArg(request.Params.Arguments, "identifier")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		selection, err := spanFromArgs(request.Params.Arguments, "startLine", "startColumn", "endLine", "endColumn")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		wantsTextChanges := boolArgOrDefault(request.Params.Arguments, "wantsTextChanges", true)
		applyChanges := boolArgOrDefault(request.Params.Arguments, "applyChanges", true)

		response, err := tools.OmniPascalRunCodeAction(s.ctx, s.omniClient, filePath, identifier, selection, wantsTextChanges, applyChanges)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to run code action: %v", err)), nil
		}
		return mcp.NewToolResultText(response), nil
	})

	coreLogger.Info("Successfully registered OmniPascal MCP tools")
	return nil
}

func (s *mcpServer) addOmniPascalPositionTool(tool mcp.Tool, fn func(filePath string, line, column int) (string, error)) {
	s.mcpServer.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, err := requireStringArg(request.Params.Arguments, "filePath")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		line, err := requireIntArg(request.Params.Arguments, "line")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		column, err := requireIntArg(request.Params.Arguments, "column")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		response, err := fn(filePath, line, column)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(response), nil
	})
}

func buildOmniPascalConfig(args map[string]any) (map[string]any, error) {
	config := map[string]any{}

	if rawConfig := stringArgOrEmpty(args, "configJson"); rawConfig != "" {
		if err := json.Unmarshal([]byte(rawConfig), &config); err != nil {
			return nil, fmt.Errorf("configJson must be a valid JSON object: %w", err)
		}
	}

	addStringConfig(args, config, "workspacePaths")
	addStringConfig(args, config, "delphiInstallationPath")
	addStringConfig(args, config, "freePascalSourcePath")
	addStringConfig(args, config, "defaultDevelopmentEnvironment")
	addStringConfig(args, config, "searchPath")
	addStringConfig(args, config, "msbuildPath")
	addStringConfig(args, config, "lazbuildPath")
	addStringConfig(args, config, "symbolIndex")
	addStringConfig(args, config, "usesListStyle")
	addStringConfig(args, config, "namingConventionString")
	if value, ok := args["createBuildScripts"].(bool); ok {
		config["createBuildScripts"] = value
	}

	if len(config) == 0 {
		return nil, fmt.Errorf("provide configJson or at least one OmniPascal configuration field")
	}

	return config, nil
}

func addStringConfig(args map[string]any, config map[string]any, key string) {
	if value, ok := args[key].(string); ok && value != "" {
		config[key] = value
	}
}

func parseTextEditsArg(args map[string]any) ([]tools.TextEdit, error) {
	editsArg, ok := args["edits"]
	if !ok {
		return nil, fmt.Errorf("edits is required")
	}

	editsArray, ok := editsArg.([]any)
	if !ok {
		return nil, fmt.Errorf("edits must be an array")
	}

	var edits []tools.TextEdit
	for _, editItem := range editsArray {
		editMap, ok := editItem.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("each edit must be an object")
		}

		startLine, err := requireIntArg(editMap, "startLine")
		if err != nil {
			return nil, err
		}
		endLine, err := requireIntArg(editMap, "endLine")
		if err != nil {
			return nil, err
		}

		newText := ""
		if value, ok := editMap["newText"].(string); ok {
			newText = value
		}

		edits = append(edits, tools.TextEdit{
			StartLine: startLine,
			EndLine:   endLine,
			NewText:   newText,
		})
	}

	return edits, nil
}

func spanFromArgs(args map[string]any, startLineKey, startColumnKey, endLineKey, endColumnKey string) (omnipascal.TextSpan, error) {
	startLine, err := requireIntArg(args, startLineKey)
	if err != nil {
		return omnipascal.TextSpan{}, err
	}
	startColumn, err := requireIntArg(args, startColumnKey)
	if err != nil {
		return omnipascal.TextSpan{}, err
	}
	endLine, err := requireIntArg(args, endLineKey)
	if err != nil {
		return omnipascal.TextSpan{}, err
	}
	endColumn, err := requireIntArg(args, endColumnKey)
	if err != nil {
		return omnipascal.TextSpan{}, err
	}

	return omnipascal.TextSpan{
		Start: omnipascal.Point{Line: startLine, Offset: startColumn},
		End:   omnipascal.Point{Line: endLine, Offset: endColumn},
	}, nil
}

func requireStringArg(args map[string]any, key string) (string, error) {
	value, ok := args[key].(string)
	if !ok || value == "" {
		return "", fmt.Errorf("%s must be a non-empty string", key)
	}
	return value, nil
}

func stringArgOrEmpty(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return value
}

func requireIntArg(args map[string]any, key string) (int, error) {
	value, ok := args[key]
	if !ok {
		return 0, fmt.Errorf("%s is required", key)
	}

	switch typed := value.(type) {
	case float64:
		return int(typed), nil
	case int:
		return typed, nil
	default:
		return 0, fmt.Errorf("%s must be a number", key)
	}
}

func intArgOrDefault(args map[string]any, key string, defaultValue int) int {
	value, err := requireIntArg(args, key)
	if err != nil {
		return defaultValue
	}
	return value
}

func boolArgOrDefault(args map[string]any, key string, defaultValue bool) bool {
	value, ok := args[key].(bool)
	if !ok {
		return defaultValue
	}
	return value
}

func requireStringArrayArg(args map[string]any, key string) ([]string, error) {
	raw, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}

	rawArray, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array of strings", key)
	}

	values := make([]string, 0, len(rawArray))
	for _, item := range rawArray {
		value, ok := item.(string)
		if !ok || value == "" {
			return nil, fmt.Errorf("%s must contain only non-empty strings", key)
		}
		values = append(values, value)
	}

	return values, nil
}
