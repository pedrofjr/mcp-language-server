package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/isaacphi/mcp-language-server/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
	sitter "github.com/smacker/go-tree-sitter"
	tree_sitter_delphi6 "github.com/tree-sitter/tree-sitter-delphi6"
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

func parseOptionalContextArgument(raw any) (string, error) {
	if raw == nil {
		return "", nil
	}

	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("context must be a string")
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

		if tools.IsDelphiWorkspaceSourceFile(path) {
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

func nodeTypeKeywordPrefix(nodeType string) string {
	prefixes := map[string]string{
		"procedure_declaration":          "procedure",
		"function_declaration":           "function",
		"constructor_declaration":        "constructor",
		"destructor_declaration":         "destructor",
		"class_procedure_declaration":    "class",
		"class_function_declaration":     "class",
		"procedure_implementation":       "procedure",
		"function_implementation":        "function",
		"constructor_implementation":     "constructor",
		"destructor_implementation":      "destructor",
		"class_procedure_implementation": "class",
		"class_function_implementation":  "class",
		"unit_declaration":               "unit",
		"program_declaration":            "program",
		"library_declaration":            "library",
		"uses_clause":                    "uses",
		"const_section":                  "const",
		"type_section":                   "type",
		"var_section":                    "var",
	}
	return prefixes[nodeType]
}

func isRoutineDeclarationNodeType(nodeType string) bool {
	switch nodeType {
	case "procedure_declaration",
		"function_declaration",
		"constructor_declaration",
		"destructor_declaration",
		"class_procedure_declaration",
		"class_function_declaration":
		return true
	default:
		return false
	}
}

func isModuleDeclarationNodeType(nodeType string) bool {
	switch nodeType {
	case "unit_declaration", "program_declaration", "library_declaration":
		return true
	default:
		return false
	}
}

func extractRoutineSymbolNameFromDeclarationText(nodeText string) string {
	trimmed := strings.TrimSpace(nodeText)
	if trimmed == "" {
		return ""
	}

	type routinePrefix struct {
		lowerPrefix string
		rawPrefix   string
	}

	prefixes := []routinePrefix{
		{lowerPrefix: "class procedure", rawPrefix: "class procedure"},
		{lowerPrefix: "class function", rawPrefix: "class function"},
		{lowerPrefix: "procedure", rawPrefix: "procedure"},
		{lowerPrefix: "function", rawPrefix: "function"},
		{lowerPrefix: "constructor", rawPrefix: "constructor"},
		{lowerPrefix: "destructor", rawPrefix: "destructor"},
	}

	lower := strings.ToLower(trimmed)
	for _, prefix := range prefixes {
		if !strings.HasPrefix(lower, prefix.lowerPrefix) {
			continue
		}

		remainder := strings.TrimSpace(trimmed[len(prefix.rawPrefix):])
		if remainder == "" {
			return ""
		}

		nameEnd := 0
		for nameEnd < len(remainder) {
			char := remainder[nameEnd]
			isLetter := (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
			isDigit := char >= '0' && char <= '9'
			if isLetter || isDigit || char == '_' || char == '.' {
				nameEnd++
				continue
			}
			break
		}

		if nameEnd == 0 {
			return ""
		}

		symbolName := strings.Trim(remainder[:nameEnd], ".")
		if symbolName == "" {
			return ""
		}

		first := symbolName[0]
		firstIsLetter := (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')
		if !firstIsLetter && first != '_' {
			return ""
		}

		if strings.EqualFold(symbolName, "operator") {
			return ""
		}

		return symbolName
	}

	return ""
}

func trimLeadingBOMAndWhitespace(text string) string {
	trimmed := strings.TrimLeftFunc(text, unicode.IsSpace)
	trimmed = strings.TrimPrefix(trimmed, "\ufeff")
	return strings.TrimLeftFunc(trimmed, unicode.IsSpace)
}

func extractModuleSymbolNameFromDeclarationText(nodeType string, nodeText string) string {
	if !isModuleDeclarationNodeType(nodeType) {
		return ""
	}

	trimmed := trimLeadingBOMAndWhitespace(nodeText)
	if trimmed == "" {
		return ""
	}

	keywordByNodeType := map[string]string{
		"unit_declaration":    "unit",
		"program_declaration": "program",
		"library_declaration": "library",
	}

	keyword := keywordByNodeType[nodeType]
	if keyword == "" {
		return ""
	}

	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, keyword) {
		return ""
	}

	remainder := trimmed[len(keyword):]
	if remainder == "" {
		return ""
	}

	if !unicode.IsSpace([]rune(remainder)[0]) {
		return ""
	}

	remainder = strings.TrimLeftFunc(remainder, unicode.IsSpace)
	if remainder == "" {
		return ""
	}

	nameEnd := 0
	for nameEnd < len(remainder) {
		char := remainder[nameEnd]
		isLetter := (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
		isDigit := char >= '0' && char <= '9'
		if isLetter || isDigit || char == '_' || char == '.' {
			nameEnd++
			continue
		}
		break
	}

	if nameEnd == 0 {
		return ""
	}

	symbolName := strings.Trim(remainder[:nameEnd], ".")
	if symbolName == "" {
		return ""
	}

	first := symbolName[0]
	firstIsLetter := (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')
	if !firstIsLetter && first != '_' {
		return ""
	}

	trailer := strings.TrimLeftFunc(remainder[nameEnd:], unicode.IsSpace)
	if trailer == "" || trailer[0] != ';' {
		return ""
	}

	return symbolName
}

func extractDpkPackageSymbolNameFromErrorNode(filePath string, nodeType string, nodeText string) string {
	if !strings.EqualFold(nodeType, "ERROR") {
		return ""
	}

	if !strings.EqualFold(filepath.Ext(filePath), ".dpk") {
		return ""
	}

	trimmed := trimLeadingBOMAndWhitespace(nodeText)
	if trimmed == "" {
		return ""
	}

	const keyword = "package"
	if len(trimmed) < len(keyword) || !strings.EqualFold(trimmed[:len(keyword)], keyword) {
		return ""
	}

	remainder := trimmed[len(keyword):]
	if remainder == "" {
		return ""
	}

	if !unicode.IsSpace([]rune(remainder)[0]) {
		return ""
	}

	remainder = strings.TrimLeftFunc(remainder, unicode.IsSpace)
	if remainder == "" {
		return ""
	}

	nameEnd := 0
	for nameEnd < len(remainder) {
		char := remainder[nameEnd]
		isLetter := (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
		isDigit := char >= '0' && char <= '9'
		if isLetter || isDigit || char == '_' {
			nameEnd++
			continue
		}
		break
	}

	if nameEnd == 0 {
		return ""
	}

	symbolName := remainder[:nameEnd]
	first := symbolName[0]
	firstIsLetter := (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')
	if !firstIsLetter && first != '_' {
		return ""
	}

	trailer := strings.TrimLeftFunc(remainder[nameEnd:], unicode.IsSpace)
	if trailer == "" || trailer[0] != ';' {
		return ""
	}

	return symbolName
}

func extractDeclarationSymbolName(filePath string, nodeType string, nodeText string) string {
	if isRoutineDeclarationNodeType(nodeType) {
		return extractRoutineSymbolNameFromDeclarationText(nodeText)
	}

	if isModuleDeclarationNodeType(nodeType) {
		return extractModuleSymbolNameFromDeclarationText(nodeType, nodeText)
	}

	if symbolName := extractDpkPackageSymbolNameFromErrorNode(filePath, nodeType, nodeText); symbolName != "" {
		return symbolName
	}

	return ""
}

func isImplicitRunQueryNodeTypeCandidate(query string) bool {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return false
	}

	for _, char := range trimmed {
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char == '_' {
			continue
		}
		return false
	}

	return true
}

func runQueryContextCheckpoint(ctx context.Context) error {
	if runQueryCheckpointHook != nil {
		runQueryCheckpointHook()
	}

	if ctx == nil {
		return nil
	}

	err := ctx.Err()
	if err == nil {
		return nil
	}

	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("run_query canceled: %w", context.Canceled)
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("run_query deadline exceeded: %w", context.DeadlineExceeded)
	}

	return fmt.Errorf("run_query context error: %w", err)
}

var runQueryCheckpointHook func()

func runQueryTextScan(ctx context.Context, query string, nodeType string, filePath string, strictFilePath bool, limit int) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := runQueryContextCheckpoint(ctx); err != nil {
		return "", err
	}

	type runQueryMatch struct {
		FilePath    string `json:"filePath"`
		StartLine   int    `json:"startLine"`
		StartColumn int    `json:"startColumn"`
		EndLine     int    `json:"endLine"`
		EndColumn   int    `json:"endColumn"`
		NodeType    string `json:"nodeType"`
		CaptureName string `json:"captureName,omitempty"`
		SymbolName  string `json:"symbolName,omitempty"`
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
	needleLower := strings.ToLower(needle)
	effectiveNeedleLower := needleLower

	fileCandidates, err := runQueryCandidates(filePath, strictFilePath)
	if err != nil {
		return "", err
	}

	if err := runQueryContextCheckpoint(ctx); err != nil {
		return "", err
	}

	matches := make([]runQueryMatch, 0, limit)
	readableFiles := 0

	lang := sitter.NewLanguage(tree_sitter_delphi6.Language())
	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	appendNodeMatch := func(candidate string, previewLines []string, treeContent []byte, node *sitter.Node, lineOffset int, minByte uint32, maxByte uint32, captureName string) {
		if node == nil || len(matches) >= limit {
			return
		}

		if maxByte > minByte {
			if node.StartByte() < minByte || node.EndByte() > maxByte {
				return
			}
		}

		nodeTypeValue := node.Type()
		nodeTypeLower := strings.ToLower(nodeTypeValue)
		if nodeTypeLower == "comment" || strings.Contains(nodeTypeLower, "comment") {
			return
		}

		nodeText := node.Content(treeContent)
		if effectiveNeedleLower != "" && !strings.Contains(strings.ToLower(nodeText), effectiveNeedleLower) {
			return
		}

		symbolName := extractDeclarationSymbolName(candidate, nodeTypeValue, nodeText)

		startPoint := node.StartPoint()
		endPoint := node.EndPoint()
		startLine := int(startPoint.Row) + 1 + lineOffset
		startColumn := int(startPoint.Column) + 1
		endLine := int(endPoint.Row) + 1 + lineOffset
		endColumn := int(endPoint.Column) + 1
		if startLine < 1 || endLine < 1 {
			return
		}
		if endColumn < 1 {
			endColumn = 1
		}

		preview := ""
		if startLine >= 1 && startLine <= len(previewLines) {
			preview = strings.TrimSpace(previewLines[startLine-1])
		}

		matches = append(matches, runQueryMatch{
			FilePath:    candidate,
			StartLine:   startLine,
			StartColumn: startColumn,
			EndLine:     endLine,
			EndColumn:   endColumn,
			NodeType:    nodeTypeValue,
			CaptureName: captureName,
			SymbolName:  symbolName,
			Preview:     preview,
			File:        candidate,
			Line:        startLine,
			Text:        preview,
		})
	}

	trimmedNodeType := strings.TrimSpace(nodeType)
	var queryNodeType *sitter.Query
	if trimmedNodeType == "" && isImplicitRunQueryNodeTypeCandidate(needle) {
		pattern := fmt.Sprintf("(%s) @match", needle)
		q, qErr := sitter.NewQuery([]byte(pattern), lang)
		if qErr == nil {
			trimmedNodeType = needle
			queryNodeType = q
			effectiveNeedleLower = ""
		}
	}
	if trimmedNodeType != "" {
		if queryNodeType == nil {
			pattern := fmt.Sprintf("(%s) @match", trimmedNodeType)
			q, qErr := sitter.NewQuery([]byte(pattern), lang)
			if qErr != nil {
				return "", fmt.Errorf("invalid node_type %q: tree-sitter query error: %w", nodeType, qErr)
			}
			queryNodeType = q
		}
		defer queryNodeType.Close()
	}

	for _, candidate := range fileCandidates {
		if err := runQueryContextCheckpoint(ctx); err != nil {
			return "", err
		}

		if len(matches) >= limit {
			break
		}

		content, readErr := os.ReadFile(candidate)
		if readErr != nil {
			continue
		}
		readableFiles++

		processTree := func(tree *sitter.Tree, treeContent []byte, previewLines []string, lineOffset int, minByte uint32, maxByte uint32) error {
			if err := runQueryContextCheckpoint(ctx); err != nil {
				return err
			}

			if tree == nil {
				return nil
			}
			root := tree.RootNode()
			if root == nil {
				return nil
			}
			startCount := len(matches)

			if queryNodeType != nil {
				cursor := sitter.NewQueryCursor()
				cursor.Exec(queryNodeType, root)
				for len(matches) < limit {
					if err := runQueryContextCheckpoint(ctx); err != nil {
						cursor.Close()
						return err
					}

					match, ok := cursor.NextMatch()
					if !ok {
						break
					}

					match = cursor.FilterPredicates(match, treeContent)
					if match == nil {
						continue
					}

					for _, capture := range match.Captures {
						if err := runQueryContextCheckpoint(ctx); err != nil {
							cursor.Close()
							return err
						}

						if len(matches) >= limit {
							break
						}
						captureName := queryNodeType.CaptureNameForId(capture.Index)
						appendNodeMatch(candidate, previewLines, treeContent, capture.Node, lineOffset, minByte, maxByte, captureName)
					}
				}
				cursor.Close()

				if len(matches) > startCount {
					return nil
				}

				stack := []*sitter.Node{root}
				for len(stack) > 0 && len(matches) < limit {
					if err := runQueryContextCheckpoint(ctx); err != nil {
						return err
					}

					last := len(stack) - 1
					node := stack[last]
					stack = stack[:last]

					if node == nil {
						continue
					}

					if node.Type() == trimmedNodeType {
						appendNodeMatch(candidate, previewLines, treeContent, node, lineOffset, minByte, maxByte, "")
					}

					for idx := int(node.ChildCount()) - 1; idx >= 0; idx-- {
						child := node.Child(idx)
						if child != nil {
							stack = append(stack, child)
						}
					}
				}
				return nil
			}

			stack := []*sitter.Node{root}
			for len(stack) > 0 && len(matches) < limit {
				if err := runQueryContextCheckpoint(ctx); err != nil {
					return err
				}

				last := len(stack) - 1
				node := stack[last]
				stack = stack[:last]

				if node == nil {
					continue
				}

				if node.IsNamed() {
					appendNodeMatch(candidate, previewLines, treeContent, node, lineOffset, minByte, maxByte, "")
				}

				for idx := int(node.ChildCount()) - 1; idx >= 0; idx-- {
					child := node.Child(idx)
					if child != nil {
						stack = append(stack, child)
					}
				}
			}

			return nil
		}

		if err := runQueryContextCheckpoint(ctx); err != nil {
			return "", err
		}

		lines := strings.Split(string(content), "\n")
		tree, parseErr := parser.ParseCtx(ctx, nil, content)
		if parseErr == nil && tree != nil {
			before := len(matches)
			if err := processTree(tree, content, lines, 0, 0, uint32(len(content))); err != nil {
				return "", err
			}

			root := tree.RootNode()
			needsSynthetic := len(matches) == before && root != nil && (root.IsError() || root.HasError())
			if !needsSynthetic {
				continue
			}
		}

		prefix := "unit __run_query_tmp__;\ninterface\n"
		suffix := "\nimplementation\nend.\n"
		wrappedContent := []byte(prefix + string(content) + suffix)
		wrappedTree, wrappedErr := parser.ParseCtx(ctx, nil, wrappedContent)
		if wrappedErr != nil {
			if errors.Is(wrappedErr, context.Canceled) || errors.Is(wrappedErr, context.DeadlineExceeded) {
				if checkpointErr := runQueryContextCheckpoint(ctx); checkpointErr != nil {
					return "", checkpointErr
				}
				return "", fmt.Errorf("run_query context error: %w", wrappedErr)
			}
			continue
		}
		if wrappedTree == nil {
			continue
		}

		prefixLines := strings.Count(prefix, "\n")
		startByte := uint32(len(prefix))
		endByte := uint32(len(prefix) + len(content))
		if err := processTree(wrappedTree, wrappedContent, lines, -prefixLines, startByte, endByte); err != nil {
			return "", err
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

	s.mcpServer.AddTool(applyTextEditTool, withToolLogging("edit_file", withLSPGuard(s.lspClient, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return OpValidationError("filePath must be a string")
		}

		// Extract edits array
		editsArg, ok := request.Params.Arguments["edits"]
		if !ok {
			return OpValidationError("edits is required")
		}

		// Type assert and convert the edits
		editsArray, ok := editsArg.([]any)
		if !ok {
			return OpValidationError("edits must be an array")
		}

		var edits []tools.TextEdit
		for _, editItem := range editsArray {
			editMap, ok := editItem.(map[string]any)
			if !ok {
				return OpValidationError("each edit must be an object")
			}

			startLine, ok := editMap["startLine"].(float64)
			if !ok {
				return OpValidationError("startLine must be a number")
			}

			endLine, ok := editMap["endLine"].(float64)
			if !ok {
				return OpValidationError("endLine must be a number")
			}

			newText, _ := editMap["newText"].(string) // newText can be empty

			edits = append(edits, tools.TextEdit{
				StartLine: int(startLine),
				EndLine:   int(endLine),
				NewText:   newText,
			})
		}

		coreLogger.Debug("Executing edit_file for file: %s", filePath)
		opCtx, cancel := handlerOperationContext(ctx)
		defer cancel()

		response, err := tools.ApplyTextEdits(opCtx, s.lspClient, filePath, edits)
		if err != nil {
			coreLogger.Error("Failed to apply edits: %v", err)
			return handleLSPBackedToolError("edit_file", opCtx, err, "verify filePath and edit ranges, then retry")
		}
		return mcp.NewToolResultText(response), nil
	})))

	readDefinitionTool := mcp.NewTool("definition",
		mcp.WithDescription("Read the source code definition of a symbol (function, type, constant, etc.) from the codebase. symbolName can be unqualified, but package/type/unit-qualified names may be required or resolve more precisely depending on the language server."),
		mcp.WithString("symbolName",
			mcp.Required(),
			mcp.Description("The symbol to resolve. Unqualified names often work, but package/type/unit-qualified names can be required or more precise depending on the language server (e.g. 'MyFunction', 'mypackage.MyFunction', 'MyType.MyMethod', 'UnitName.Symbol')."),
		),
	)

	s.mcpServer.AddTool(readDefinitionTool, withToolLogging("definition", withLSPGuard(s.lspClient, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		symbolName, ok := request.Params.Arguments["symbolName"].(string)
		if !ok {
			return OpValidationError("symbolName must be a string")
		}

		coreLogger.Debug("Executing definition for symbol: %s", symbolName)
		opCtx, cancel := context.WithTimeout(ctx, definitionReferencesHandlerTimeout)
		defer cancel()

		text, err := tools.ReadDefinition(opCtx, s.lspClient, symbolName)
		if err != nil {
			if deterministicError := deterministicDefinitionReferencesContextError("definition", opCtx, err); deterministicError != nil {
				coreLogger.Error("Failed to get definition: %v", err)
				return deterministicError, nil
			}

			coreLogger.Error("Failed to get definition: %v", err)
			return OpToolFailedError("definition", err.Error(), "verify symbol name and LSP availability, then retry")
		}
		return mcp.NewToolResultText(text), nil
	})))

	findReferencesTool := mcp.NewTool("references",
		mcp.WithDescription("Find all usages and references of a symbol throughout the codebase. symbolName can be unqualified, but package/type/unit-qualified names may be required or resolve more precisely depending on the language server."),
		mcp.WithString("symbolName",
			mcp.Required(),
			mcp.Description("The symbol to search for. Unqualified names often work, but package/type/unit-qualified names can be required or more precise depending on the language server (e.g. 'MyFunction', 'mypackage.MyFunction', 'MyType.MyMethod', 'UnitName.Symbol')."),
		),
	)

	s.mcpServer.AddTool(findReferencesTool, withToolLogging("references", withLSPGuard(s.lspClient, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		symbolName, ok := request.Params.Arguments["symbolName"].(string)
		if !ok {
			return OpValidationError("symbolName must be a string")
		}

		coreLogger.Debug("Executing references for symbol: %s", symbolName)
		opCtx, cancel := context.WithTimeout(ctx, definitionReferencesHandlerTimeout)
		defer cancel()

		text, err := tools.FindReferences(opCtx, s.lspClient, symbolName)
		if err != nil {
			if deterministicError := deterministicDefinitionReferencesContextError("references", opCtx, err); deterministicError != nil {
				coreLogger.Error("Failed to find references: %v", err)
				return deterministicError, nil
			}

			coreLogger.Error("Failed to find references: %v", err)
			return OpToolFailedError("references", err.Error(), "verify symbol name and LSP availability, then retry")
		}
		return mcp.NewToolResultText(text), nil
	})))

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

	s.mcpServer.AddTool(getDiagnosticsTool, withToolLogging("diagnostics", withLSPGuard(s.lspClient, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return OpValidationError("filePath must be a string")
		}

		contextLines, err := parseContextLinesArgument(request.Params.Arguments["contextLines"], 5)
		if err != nil {
			return OpErrorFromParseArg(err)
		}

		showLineNumbers := true // default value
		if showLineNumbersArg, ok := request.Params.Arguments["showLineNumbers"].(bool); ok {
			showLineNumbers = showLineNumbersArg
		}

		coreLogger.Debug("Executing diagnostics for file: %s", filePath)
		opCtx, cancel := handlerOperationContext(ctx)
		defer cancel()

		text, err := tools.GetDiagnosticsForFile(opCtx, s.lspClient, filePath, contextLines, showLineNumbers)
		if err != nil {
			coreLogger.Error("Failed to get diagnostics: %v", err)
			return handleLSPBackedToolError("diagnostics", opCtx, err, "verify filePath and LSP availability, then retry")
		}
		return mcp.NewToolResultText(text), nil
	})))

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

	s.mcpServer.AddTool(hoverTool, withToolLogging("hover", withLSPGuard(s.lspClient, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return OpValidationError("filePath must be a string")
		}

		// Handle both float64 and int for line and column due to JSON parsing
		var line, column int
		switch v := request.Params.Arguments["line"].(type) {
		case float64:
			line = int(v)
		case int:
			line = v
		default:
			return OpValidationError("line must be a number")
		}

		switch v := request.Params.Arguments["column"].(type) {
		case float64:
			column = int(v)
		case int:
			column = v
		default:
			return OpValidationError("column must be a number")
		}

		coreLogger.Debug("Executing hover for file: %s line: %d column: %d", filePath, line, column)
		opCtx, cancel := handlerOperationContext(ctx)
		defer cancel()

		text, err := tools.GetHoverInfo(opCtx, s.lspClient, filePath, line, column)
		if err != nil {
			coreLogger.Error("Failed to get hover information: %v", err)
			return handleLSPBackedToolError("hover", opCtx, err, "verify filePath, line/column and LSP availability, then retry")
		}
		return mcp.NewToolResultText(text), nil
	})))

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

	s.mcpServer.AddTool(renameSymbolTool, withToolLogging("rename_symbol", withLSPGuard(s.lspClient, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return OpValidationError("filePath must be a string")
		}

		newName, ok := request.Params.Arguments["newName"].(string)
		if !ok {
			return OpValidationError("newName must be a string")
		}

		// Handle both float64 and int for line and column due to JSON parsing
		var line, column int
		switch v := request.Params.Arguments["line"].(type) {
		case float64:
			line = int(v)
		case int:
			line = v
		default:
			return OpValidationError("line must be a number")
		}

		switch v := request.Params.Arguments["column"].(type) {
		case float64:
			column = int(v)
		case int:
			column = v
		default:
			return OpValidationError("column must be a number")
		}

		coreLogger.Debug("Executing rename_symbol for file: %s line: %d column: %d newName: %s", filePath, line, column, newName)
		opCtx, cancel := handlerOperationContext(ctx)
		defer cancel()

		text, err := tools.RenameSymbol(opCtx, s.lspClient, filePath, line, column, newName)
		if err != nil {
			coreLogger.Error("Failed to rename symbol: %v", err)
			return handleLSPBackedToolError("rename_symbol", opCtx, err, "verify position, newName and LSP availability, then retry")
		}
		return mcp.NewToolResultText(text), nil
	})))

	// workspace_symbols
	s.mcpServer.AddTool(
		mcp.NewTool("workspace_symbols",
			mcp.WithDescription("Search for symbols (types, functions, constants, variables) across the entire Delphi workspace by name or substring. Returns name, kind, and file location for each match."),
			mcp.WithString("query",
				mcp.Description("Substring to filter symbols (case-insensitive). Leave empty to list all exported symbols."),
			),
		),
		withToolLogging("workspace_symbols", withPreToolValidation(func(req mcp.CallToolRequest) (*mcp.CallToolResult, bool) {
			if queryRaw := req.Params.Arguments["query"]; queryRaw != nil {
				if _, ok := queryRaw.(string); !ok {
					result, _ := OpValidationError("query must be a string")
					return result, true
				}
			}
			return nil, false
		}, withLSPGuard(s.lspClient, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query, _ := req.Params.Arguments["query"].(string)

			opCtx, cancel := handlerOperationContext(ctx)
			defer cancel()

			result, err := tools.GetWorkspaceSymbols(opCtx, s.lspClient, query)
			if err != nil {
				return handleLSPBackedToolError("workspace_symbols", opCtx, err, "check LSP availability and query value, then retry")
			}
			return mcp.NewToolResultText(result), nil
		}))),
	)

	// get_symbols_overview
	s.mcpServer.AddTool(
		mcp.NewTool("get_symbols_overview",
			mcp.WithDescription("Aggregate workspace symbols by URI and return a compact JSON overview grouped per unit/file."),
			mcp.WithString("query",
				mcp.Description("Optional filter string forwarded to workspace/symbol. Whitespace-only values are trimmed to empty."),
			),
		),
		withToolLogging("get_symbols_overview", withPreToolValidation(func(req mcp.CallToolRequest) (*mcp.CallToolResult, bool) {
			if queryRaw := req.Params.Arguments["query"]; queryRaw != nil {
				if _, ok := queryRaw.(string); !ok {
					result, _ := OpValidationError("query must be a string")
					return result, true
				}
			}
			return nil, false
		}, withLSPGuard(s.lspClient, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query, _ := req.Params.Arguments["query"].(string)

			opCtx, cancel := handlerOperationContext(ctx)
			defer cancel()

			result, err := tools.GetSymbolsOverview(opCtx, s.lspClient, query)
			if err != nil {
				return handleLSPBackedToolError("get_symbols_overview", opCtx, err, "check LSP availability and query value, then retry")
			}
			return mcp.NewToolResultText(result), nil
		}))),
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
		withToolLogging("ast_summary", withLSPGuard(s.lspClient, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, ok := req.Params.Arguments["uri"].(string)
			if !ok || uri == "" {
				return OpValidationError("uri must be a non-empty string")
			}

			opCtx, cancel := handlerOperationContext(ctx)
			defer cancel()

			result, err := tools.GetAstSummary(opCtx, s.lspClient, uri)
			if err != nil {
				return handleLSPBackedToolError("ast_summary", opCtx, err, "verify uri points to a Delphi source file and retry")
			}
			return mcp.NewToolResultText(result), nil
		})),
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
		withToolLogging("dependency_tree", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, ok := req.Params.Arguments["uri"].(string)
			if !ok || uri == "" {
				return OpValidationError("uri must be a non-empty string")
			}
			direction := ""
			if directionRaw, exists := req.Params.Arguments["direction"]; exists && directionRaw != nil {
				directionValue, ok := directionRaw.(string)
				if !ok {
					return OpValidationError("direction must be a string: 'imports' or 'importedBy'")
				}
				direction = directionValue
			}
			if direction != "" && direction != "imports" && direction != "importedBy" {
				return OpValidationError("direction must be 'imports' or 'importedBy'")
			}

			opCtx, cancel := handlerOperationContext(ctx)
			defer cancel()

			result, err := tools.GetDependencyTree(opCtx, s.lspClient, uri, direction)
			if err != nil {
				return handleLSPBackedToolError("dependency_tree", opCtx, err, "verify uri and LSP graph index, then retry")
			}
			return mcp.NewToolResultText(result), nil
		}),
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
		withToolLogging("graph_neighbors", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, ok := req.Params.Arguments["uri"].(string)
			if !ok || strings.TrimSpace(uri) == "" {
				return OpValidationError("uri must be a non-empty string")
			}

			relationType := ""
			if relationTypeRaw, exists := req.Params.Arguments["relationType"]; exists && relationTypeRaw != nil {
				relationTypeValue, ok := relationTypeRaw.(string)
				if !ok {
					return OpValidationError("relationType must be 'uses_unit'")
				}
				relationType = strings.TrimSpace(relationTypeValue)
			}
			if relationType != "" && relationType != "uses_unit" {
				return OpValidationError("relationType must be 'uses_unit'")
			}

			direction := ""
			if directionRaw, exists := req.Params.Arguments["direction"]; exists && directionRaw != nil {
				directionValue, ok := directionRaw.(string)
				if !ok {
					return OpValidationError("direction must be 'imports' or 'importedBy'")
				}
				direction = strings.TrimSpace(directionValue)
			}
			if direction != "" && direction != "imports" && direction != "importedBy" {
				return OpValidationError("direction must be 'imports' or 'importedBy'")
			}

			opCtx, cancel := handlerOperationContext(ctx)
			defer cancel()

			result, err := tools.GetGraphNeighbors(opCtx, s.lspClient, uri, relationType, direction)
			if err != nil {
				return handleLSPBackedToolError("graph_neighbors", opCtx, err, "verify uri, relationType and direction, then retry")
			}
			return mcp.NewToolResultText(result), nil
		}),
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
		withToolLogging("graph_node", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, ok := req.Params.Arguments["uri"].(string)
			if !ok || strings.TrimSpace(uri) == "" {
				return OpValidationError("uri must be a non-empty string")
			}

			relationType := ""
			if relationTypeRaw, exists := req.Params.Arguments["relationType"]; exists && relationTypeRaw != nil {
				relationTypeValue, ok := relationTypeRaw.(string)
				if !ok {
					return OpValidationError("relationType must be 'uses_unit'")
				}
				relationType = strings.TrimSpace(relationTypeValue)
			}
			if relationType != "" && relationType != "uses_unit" {
				return OpValidationError("relationType must be 'uses_unit'")
			}

			opCtx, cancel := handlerOperationContext(ctx)
			defer cancel()

			result, err := tools.GetGraphNode(opCtx, s.lspClient, uri, relationType)
			if err != nil {
				return handleLSPBackedToolError("graph_node", opCtx, err, "verify uri and relationType, then retry")
			}
			return mcp.NewToolResultText(result), nil
		}),
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
		withToolLogging("graph_query", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, ok := req.Params.Arguments["uri"].(string)
			if !ok || strings.TrimSpace(uri) == "" {
				return OpValidationError("uri must be a non-empty string")
			}

			relationType := ""
			if relationTypeRaw, exists := req.Params.Arguments["relationType"]; exists && relationTypeRaw != nil {
				relationTypeValue, ok := relationTypeRaw.(string)
				if !ok {
					return OpValidationError("relationType must be 'uses_unit'")
				}
				relationType = strings.TrimSpace(relationTypeValue)
			}
			if relationType != "" && relationType != "uses_unit" {
				return OpValidationError("relationType must be 'uses_unit'")
			}

			direction := ""
			if directionRaw, exists := req.Params.Arguments["direction"]; exists && directionRaw != nil {
				directionValue, ok := directionRaw.(string)
				if !ok {
					return OpValidationError("direction must be 'imports', 'importedBy' or 'both'")
				}
				direction = strings.TrimSpace(directionValue)
			}
			if direction != "" && direction != "imports" && direction != "importedBy" && direction != "both" {
				return OpValidationError("direction must be 'imports', 'importedBy' or 'both'")
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
					return OpValidationError("depth must be an integer")
				}

				if depthNumber != math.Trunc(depthNumber) {
					return OpValidationError("depth must be an integer")
				}
				if depthNumber < 0 {
					return OpValidationError("depth must be greater than or equal to 0")
				}

				depth = int(depthNumber)
			}

			opCtx, cancel := handlerOperationContext(ctx)
			defer cancel()

			result, err := tools.GetGraphQuery(opCtx, s.lspClient, uri, relationType, direction, depth)
			if err != nil {
				return handleLSPBackedToolError("graph_query", opCtx, err, "verify uri, direction, depth and LSP graph index, then retry")
			}
			return mcp.NewToolResultText(result), nil
		}),
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
		withToolLogging("call_graph", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			symbolName, ok := req.Params.Arguments["symbolName"].(string)
			if !ok {
				return OpValidationError("symbolName must be a string")
			}

			symbolName = strings.TrimSpace(symbolName)
			if symbolName == "" {
				return OpValidationError("symbolName must be a non-empty string")
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
					return OpValidationError("depth must be a number")
				}

				if depthNumber <= 0 {
					return OpValidationError("depth must be a positive integer")
				}
				if depthNumber != math.Trunc(depthNumber) {
					return OpValidationError("depth must be an integer")
				}

				depth = int(depthNumber)
			}

			opCtx, cancel := handlerOperationContext(ctx)
			defer cancel()

			result, err := tools.GetCallGraph(opCtx, s.lspClient, symbolName, depth)
			if err != nil {
				return handleLSPBackedToolError("call_graph", opCtx, err, "verify symbolName and LSP call graph index, then retry")
			}
			return mcp.NewToolResultText(result), nil
		}),
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
		withToolLogging("semantic_search", withPreToolValidation(func(req mcp.CallToolRequest) (*mcp.CallToolResult, bool) {
			query, ok := req.Params.Arguments["query"].(string)
			if !ok || strings.TrimSpace(query) == "" {
				result, _ := OpValidationError("query must be a non-empty string")
				return result, true
			}

			scope := "workspace"
			if scopeRaw, exists := req.Params.Arguments["scope"]; exists && scopeRaw != nil {
				scopeText, ok := scopeRaw.(string)
				if !ok {
					result, _ := OpValidationError("scope must be 'workspace' or 'file'")
					return result, true
				}
				scopeText = strings.TrimSpace(scopeText)
				if scopeText != "" {
					scope = scopeText
				}
			}

			if scope != "workspace" && scope != "file" {
				result, _ := OpValidationError("scope must be 'workspace' or 'file'")
				return result, true
			}

			if scope == "file" {
				uriRaw, exists := req.Params.Arguments["uri"]
				if !exists || uriRaw == nil {
					result, _ := OpValidationError("uri is required when scope='file'")
					return result, true
				}
				uriText, ok := uriRaw.(string)
				if !ok {
					result, _ := OpValidationError("uri must be a string")
					return result, true
				}
				if strings.TrimSpace(uriText) == "" {
					result, _ := OpValidationError("uri is required when scope='file'")
					return result, true
				}
			}

			if limitRaw, exists := req.Params.Arguments["limit"]; exists && limitRaw != nil {
				var limitNumber float64
				switch v := limitRaw.(type) {
				case float64:
					limitNumber = v
				case int:
					limitNumber = float64(v)
				default:
					result, _ := OpValidationError("limit must be a positive integer")
					return result, true
				}

				if limitNumber <= 0 || limitNumber != math.Trunc(limitNumber) {
					result, _ := OpValidationError("limit must be a positive integer")
					return result, true
				}
			}

			return nil, false
		}, withLSPGuard(s.lspClient, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query := req.Params.Arguments["query"].(string)

			scope := "workspace"
			if scopeRaw, exists := req.Params.Arguments["scope"]; exists && scopeRaw != nil {
				scopeText, _ := scopeRaw.(string)
				scopeText = strings.TrimSpace(scopeText)
				if scopeText != "" {
					scope = scopeText
				}
			}

			uri := ""
			if uriRaw, exists := req.Params.Arguments["uri"]; exists && uriRaw != nil {
				uri, _ = uriRaw.(string)
				uri = strings.TrimSpace(uri)
			}

			limit := 20
			if limitRaw, exists := req.Params.Arguments["limit"]; exists && limitRaw != nil {
				switch v := limitRaw.(type) {
				case float64:
					limit = int(v)
				case int:
					limit = v
				}
			}

			opCtx, cancel := handlerOperationContext(ctx)
			defer cancel()

			result, err := tools.GetSemanticSearch(opCtx, s.lspClient, query, scope, uri, limit)
			if err != nil {
				return handleLSPBackedToolError("semantic_search", opCtx, err, "verify query, scope/uri and LSP semantic index, then retry")
			}
			return mcp.NewToolResultText(result), nil
		}))),
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
		withToolLogging("code_actions", withPreToolValidation(func(req mcp.CallToolRequest) (*mcp.CallToolResult, bool) {
			if _, ok := req.Params.Arguments["filePath"].(string); !ok {
				result, _ := OpValidationError("filePath must be a string")
				return result, true
			}

			switch req.Params.Arguments["line"].(type) {
			case float64, int:
			default:
				result, _ := OpValidationError("line must be a number")
				return result, true
			}

			switch req.Params.Arguments["column"].(type) {
			case float64, int:
			default:
				result, _ := OpValidationError("column must be a number")
				return result, true
			}

			if onlyRaw, exists := req.Params.Arguments["only"]; exists && onlyRaw != nil {
				onlyArr, ok := onlyRaw.([]any)
				if !ok {
					result, _ := OpValidationError("only must be an array of strings")
					return result, true
				}
				for _, item := range onlyArr {
					if _, ok := item.(string); !ok {
						result, _ := OpValidationError("only must be an array of strings")
						return result, true
					}
				}
			}

			if inclRaw, exists := req.Params.Arguments["includeDiagnostics"]; exists && inclRaw != nil {
				if _, ok := inclRaw.(bool); !ok {
					result, _ := OpValidationError("includeDiagnostics must be a boolean")
					return result, true
				}
			}

			return nil, false
		}, withLSPGuard(s.lspClient, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filePath := req.Params.Arguments["filePath"].(string)

			var line int
			switch v := req.Params.Arguments["line"].(type) {
			case float64:
				line = int(v)
			case int:
				line = v
			}

			var column int
			switch v := req.Params.Arguments["column"].(type) {
			case float64:
				column = int(v)
			case int:
				column = v
			}

			var only []string
			if onlyRaw, exists := req.Params.Arguments["only"]; exists && onlyRaw != nil {
				for _, item := range onlyRaw.([]any) {
					only = append(only, item.(string))
				}
			}

			includeDiagnostics := false
			if inclRaw, exists := req.Params.Arguments["includeDiagnostics"]; exists && inclRaw != nil {
				includeDiagnostics = inclRaw.(bool)
			}

			opCtx, cancel := handlerOperationContext(ctx)
			defer cancel()

			text, err := tools.GetCodeActions(opCtx, s.lspClient, filePath, line, column, only, includeDiagnostics)
			if err != nil {
				return handleLSPBackedToolError("code_actions", opCtx, err, "verify filePath, position and LSP availability, then retry")
			}
			return mcp.NewToolResultText(text), nil
		}))),
	)

	// replace_symbol_body
	s.mcpServer.AddTool(
		mcp.NewTool("replace_symbol_body",
			mcp.WithDescription("Replace the begin..end body of a named Delphi symbol with new code."),
			mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute path to the .pas file")),
			mcp.WithString("symbolName", mcp.Required(), mcp.Description("Symbol name, e.g. 'TFoo.Bar' or 'Bar'")),
			mcp.WithString("newBody", mcp.Required(), mcp.Description("Replacement text for the begin..end block (include begin and end lines)")),
		),
		withToolLogging("replace_symbol_body", withLSPGuard(s.lspClient, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filePath, ok := req.Params.Arguments["filePath"].(string)
			if !ok {
				return OpValidationError("filePath must be a string")
			}
			symbolName, ok := req.Params.Arguments["symbolName"].(string)
			if !ok {
				return OpValidationError("symbolName must be a string")
			}
			newBody, ok := req.Params.Arguments["newBody"].(string)
			if !ok {
				return OpValidationError("newBody must be a string")
			}

			opCtx, cancel := handlerOperationContext(ctx)
			defer cancel()

			result, err := tools.ReplaceSymbolBody(opCtx, s.lspClient, filePath, symbolName, newBody)
			if err != nil {
				return handleLSPBackedToolError("replace_symbol_body", opCtx, err, "verify symbol exists and newBody is valid, then retry")
			}
			return mcp.NewToolResultText(result), nil
		})),
	)

	// insert_after_symbol
	s.mcpServer.AddTool(
		mcp.NewTool("insert_after_symbol",
			mcp.WithDescription("Insert code immediately after the end of a named Delphi symbol."),
			mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute path to the .pas file")),
			mcp.WithString("symbolName", mcp.Required(), mcp.Description("Symbol name, e.g. 'TFoo.Bar'")),
			mcp.WithString("text", mcp.Required(), mcp.Description("Text to insert after the symbol")),
		),
		withToolLogging("insert_after_symbol", withLSPGuard(s.lspClient, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filePath, ok := req.Params.Arguments["filePath"].(string)
			if !ok {
				return OpValidationError("filePath must be a string")
			}
			symbolName, ok := req.Params.Arguments["symbolName"].(string)
			if !ok {
				return OpValidationError("symbolName must be a string")
			}
			text, ok := req.Params.Arguments["text"].(string)
			if !ok {
				return OpValidationError("text must be a string")
			}

			opCtx, cancel := handlerOperationContext(ctx)
			defer cancel()

			result, err := tools.InsertAfterSymbol(opCtx, s.lspClient, filePath, symbolName, text)
			if err != nil {
				return handleLSPBackedToolError("insert_after_symbol", opCtx, err, "verify symbol exists and text payload, then retry")
			}
			return mcp.NewToolResultText(result), nil
		})),
	)

	// insert_before_symbol
	s.mcpServer.AddTool(
		mcp.NewTool("insert_before_symbol",
			mcp.WithDescription("Insert code immediately before a named Delphi symbol."),
			mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute path to the .pas file")),
			mcp.WithString("symbolName", mcp.Required(), mcp.Description("Symbol name, e.g. 'TFoo.Bar'")),
			mcp.WithString("text", mcp.Required(), mcp.Description("Text to insert before the symbol")),
		),
		withToolLogging("insert_before_symbol", withLSPGuard(s.lspClient, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filePath, ok := req.Params.Arguments["filePath"].(string)
			if !ok {
				return OpValidationError("filePath must be a string")
			}
			symbolName, ok := req.Params.Arguments["symbolName"].(string)
			if !ok {
				return OpValidationError("symbolName must be a string")
			}
			text, ok := req.Params.Arguments["text"].(string)
			if !ok {
				return OpValidationError("text must be a string")
			}

			opCtx, cancel := handlerOperationContext(ctx)
			defer cancel()

			result, err := tools.InsertBeforeSymbol(opCtx, s.lspClient, filePath, symbolName, text)
			if err != nil {
				return handleLSPBackedToolError("insert_before_symbol", opCtx, err, "verify symbol exists and text payload, then retry")
			}
			return mcp.NewToolResultText(result), nil
		})),
	)

	// === Sprint 2 - Memoria ===
	memoryWriteHandler := withToolLogging("memory_write", func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
			return OpErrorFromMemory("memory_write", err)
		}
		return mcp.NewToolResultText("Entrada criada com ID: " + id), nil
	})

	memoryReadHandler := withToolLogging("memory_read", func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, _ := request.Params.Arguments["id"].(string)
		entry, err := tools.MemoryRead(id)
		if err != nil {
			return OpErrorFromMemory("memory_read", err)
		}
		return mcp.NewToolResultText(fmt.Sprintf("# %s\n\n%s\nTags: %s", entry.Title, entry.Content, strings.Join(entry.Tags, ", "))), nil
	})

	memoryListHandler := withToolLogging("memory_list", func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var tag string
		if rawTag, ok := request.Params.Arguments["tag"]; ok && rawTag != nil {
			parsedTag, ok := rawTag.(string)
			if !ok {
				return OpValidationError("tag must be a string")
			}
			tag = parsedTag
		}
		entries, err := tools.MemoryList(tag)
		if err != nil {
			return OpErrorFromMemory("memory_list", err)
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
	})

	memoryEditHandler := withToolLogging("memory_edit", func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, _ := request.Params.Arguments["id"].(string)
		content, _ := request.Params.Arguments["content"].(string)
		if err := tools.MemoryEdit(id, content); err != nil {
			return OpErrorFromMemory("memory_edit", err)
		}
		return mcp.NewToolResultText("Entrada atualizada com sucesso"), nil
	})

	memoryDeleteHandler := withToolLogging("memory_delete", func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, _ := request.Params.Arguments["id"].(string)
		if err := tools.MemoryDelete(id); err != nil {
			return OpErrorFromMemory("memory_delete", err)
		}
		return mcp.NewToolResultText("Entrada removida com sucesso"), nil
	})

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
	), withToolLogging("safe_delete_symbol", withLSPGuard(s.lspClient, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, _ := request.Params.Arguments["filePath"].(string)
		symbolName, _ := request.Params.Arguments["symbolName"].(string)
		force, _ := request.Params.Arguments["force"].(bool)

		opCtx, cancel := handlerOperationContext(ctx)
		defer cancel()

		result, err := tools.SafeDeleteSymbol(opCtx, s.lspClient, filePath, symbolName, force)
		if err != nil {
			if deterministic := deterministicHandlerContextError("safe_delete_symbol", opCtx, err); deterministic != nil {
				return deterministic, nil
			}
			return OpErrorFromDomain("safe_delete_symbol", err, "verify symbol has no blocking references or set force=true")
		}
		return mcp.NewToolResultText(result), nil
	})))

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
		withToolLogging("analyze_complexity", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			src, ok := req.Params.Arguments["src"].(string)
			if !ok {
				return OpValidationError("src must be a string")
			}

			symbolName, ok := req.Params.Arguments["symbol_name"].(string)
			if !ok {
				return OpValidationError("symbol_name must be a string")
			}

			result := tools.AnalyzeComplexity(src, symbolName)
			if result == nil {
				return mcp.NewToolResultText("Symbol not found"), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf(`{"symbol":"%s","complexity":%d,"rating":"%s"}`,
				result.SymbolName, result.Score, result.Rating)), nil
		}),
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
			mcp.WithNumber("window_lines",
				mcp.Description("Sliding window size in lines [1, 200], default 10"),
			),
		),
		withToolLogging("find_similar_code", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			src, ok := req.Params.Arguments["src"].(string)
			if !ok {
				return OpValidationError("src must be a string")
			}

			query, ok := req.Params.Arguments["query"].(string)
			if !ok {
				return OpValidationError("query must be a string")
			}

			threshold := 0.3
			if raw, exists := req.Params.Arguments["threshold"]; exists && raw != nil {
				switch value := raw.(type) {
				case float64:
					threshold = value
				case int:
					threshold = float64(value)
				default:
					return OpValidationError("threshold must be a number")
				}

				if math.IsNaN(threshold) || math.IsInf(threshold, 0) || threshold < 0.0 || threshold > 1.0 {
					return OpValidationError("threshold must be a finite number between 0.0 and 1.0")
				}
			}

			windowLines := 10
			if raw, exists := req.Params.Arguments["window_lines"]; exists && raw != nil {
				switch value := raw.(type) {
				case float64:
					if math.IsNaN(value) || math.IsInf(value, 0) {
						return OpValidationError("window_lines must be an integer between 1 and 200")
					}
					if value != math.Trunc(value) {
						return OpValidationError("window_lines must be an integer between 1 and 200")
					}
					windowLines = int(value)
				case int:
					windowLines = value
				default:
					return OpValidationError("window_lines must be an integer between 1 and 200")
				}

				if windowLines < 1 || windowLines > 200 {
					return OpValidationError("window_lines must be an integer between 1 and 200")
				}
			}

			blocks := tools.FindSimilarCodeWithOptions(src, query, tools.FindSimilarCodeOptions{
				Threshold:   threshold,
				WindowLines: windowLines,
			})
			data, err := json.Marshal(blocks)
			if err != nil {
				return OpToolFailedError("find_similar_code", err.Error(), "retry with smaller src/query payload")
			}
			return mcp.NewToolResultText(string(data)), nil
		}),
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
		withToolLogging("activate_project", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			dir, ok := req.Params.Arguments["dir"].(string)
			if !ok {
				return OpValidationError("dir must be a string")
			}

			result, err := tools.ActivateProject(dir)
			if err != nil {
				return OpToolFailedError("activate_project", err.Error(), "verify dir points to a Delphi project root, then retry")
			}
			return mcp.NewToolResultText(result), nil
		}),
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
		withToolLogging("build_query", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			nodeType, ok := req.Params.Arguments["node_type"].(string)
			if !ok {
				return OpValidationError("node_type must be a string")
			}

			symbol := ""
			if raw, exists := req.Params.Arguments["symbol"]; exists && raw != nil {
				value, ok := raw.(string)
				if !ok {
					return OpValidationError("symbol must be a string")
				}
				symbol = value
			}

			return mcp.NewToolResultText(tools.BuildQuery(nodeType, symbol)), nil
		}),
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
		withToolLogging("adapt_query", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			base, ok := req.Params.Arguments["base"].(string)
			if !ok {
				return OpValidationError("base must be a string")
			}

			dialect, ok := req.Params.Arguments["dialect"].(string)
			if !ok {
				return OpValidationError("dialect must be a string")
			}

			return mcp.NewToolResultText(tools.AdaptQuery(base, dialect)), nil
		}),
	)

	// Sprint 3: run_query
	s.mcpServer.AddTool(
		mcp.NewTool("run_query",
			mcp.WithDescription("run_query v2 ("+runQueryPublicContract+" tree-sitter em .pas/.pp/.dpr/.dpk/.lpr/.inc; mesma matriz de references. Requer query e/ou node_type; com node_type executa query tree-sitter (captureName/symbolName); sem node_type filtra nos AST por substring. Campos legados file/line/text preservados."),
			mcp.WithString("query",
				mcp.Description("Optional text filter on node content, or implicit node_type when it matches a valid tree-sitter node type name (lowercase + underscores)."),
			),
			mcp.WithString("node_type",
				mcp.Description("Tree-sitter node type for structural query `(node_type) @match`; required when query is omitted."),
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
		withToolLogging("run_query", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query := ""
			if raw, exists := req.Params.Arguments["query"]; exists && raw != nil {
				value, ok := raw.(string)
				if !ok {
					return OpValidationError("query must be a string")
				}
				query = strings.TrimSpace(value)
			}

			nodeType := ""
			if raw, exists := req.Params.Arguments["node_type"]; exists && raw != nil {
				value, ok := raw.(string)
				if !ok {
					return OpValidationError("node_type must be a string")
				}
				nodeType = strings.TrimSpace(value)
			}

			if query == "" && nodeType == "" {
				return OpValidationError("query or node_type must be provided")
			}

			filePath := ""
			if raw, exists := req.Params.Arguments["filePath"]; exists && raw != nil {
				value, ok := raw.(string)
				if !ok {
					return OpValidationError("filePath must be a string")
				}
				filePath = value
			}

			strictFilePath := false
			if raw, exists := req.Params.Arguments["strictFilePath"]; exists && raw != nil {
				value, ok := raw.(bool)
				if !ok {
					return OpValidationError("strictFilePath must be a boolean")
				}
				strictFilePath = value
			}

			if strictFilePath && strings.TrimSpace(filePath) == "" {
				return OpValidationError("strictFilePath=true requires filePath")
			}

			limit, err := parseOptionalPositiveIntegerArgument(req.Params.Arguments["limit"], 20, "limit")
			if err != nil {
				return OpErrorFromParseArg(err)
			}

			opCtx, cancel := context.WithTimeout(ctx, runQueryHandlerTimeout)
			defer cancel()

			result, err := runQueryTextScan(opCtx, query, nodeType, filePath, strictFilePath, limit)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return mcp.NewToolResultError(opErrMsgWithRecovery(OpRunQueryCanceled, fmt.Sprintf("failed: %v | action: retry when context is active", err))), nil
				}
				if errors.Is(err, context.DeadlineExceeded) {
					return mcp.NewToolResultError(opErrMsgWithRecovery(OpRunQueryDeadline, fmt.Sprintf("failed: %v | action: retry with longer timeout", err))), nil
				}
				return OpToolFailedError("run_query", err.Error(), "verify query/node_type/filePath and workspace scan scope, then retry")
			}

			return mcp.NewToolResultText(result), nil
		}),
	)
	s.mcpServer.AddTool(mcp.NewTool("get_diagnostics_for_symbol",
		mcp.WithDescription("Retorna diagnosticos do LSP relevantes para um simbolo"),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Caminho absoluto do arquivo .pas")),
		mcp.WithString("symbolName", mcp.Required(), mcp.Description("Nome do simbolo")),
	), withToolLogging("get_diagnostics_for_symbol", withLSPGuard(s.lspClient, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, _ := request.Params.Arguments["filePath"].(string)
		symbolName, _ := request.Params.Arguments["symbolName"].(string)

		opCtx, cancel := handlerOperationContext(ctx)
		defer cancel()

		result, err := tools.GetDiagnosticsForSymbol(opCtx, s.lspClient, filePath, symbolName)
		if err != nil {
			if deterministic := deterministicHandlerContextError("get_diagnostics_for_symbol", opCtx, err); deterministic != nil {
				return deterministic, nil
			}
			return OpErrorFromDomain("get_diagnostics_for_symbol", err, "verify filePath and symbolName, then retry")
		}
		return mcp.NewToolResultText(result), nil
	})))

	s.mcpServer.AddTool(mcp.NewTool("find_implementations",
		mcp.WithDescription("Encontra classes que implementam uma interface Delphi"),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Arquivo onde a interface esta declarada")),
		mcp.WithString("symbolName", mcp.Required(), mcp.Description("Nome da interface")),
	), withToolLogging("find_implementations", withLSPGuard(s.lspClient, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, _ := request.Params.Arguments["filePath"].(string)
		symbolName, _ := request.Params.Arguments["symbolName"].(string)

		opCtx, cancel := handlerOperationContext(ctx)
		defer cancel()

		result, err := tools.FindImplementations(opCtx, s.lspClient, filePath, symbolName, s.config.workspaceDir)
		if err != nil {
			if deterministic := deterministicHandlerContextError("find_implementations", opCtx, err); deterministic != nil {
				return deterministic, nil
			}
			return OpErrorFromDomain("find_implementations", err, "verify interface symbol and workspace, then retry")
		}
		return mcp.NewToolResultText(result), nil
	})))

	s.mcpServer.AddTool(mcp.NewTool("get_node_at_position",
		mcp.WithDescription("Retorna o token e contexto textual em uma posicao do arquivo"),
		mcp.WithString("filePath", mcp.Required(), mcp.Description("Caminho absoluto do arquivo")),
		mcp.WithNumber("line", mcp.Required(), mcp.Description("Numero de linha (1-indexado)")),
		mcp.WithNumber("column", mcp.Required(), mcp.Description("Numero de coluna (1-indexado)")),
	), withToolLogging("get_node_at_position", withPreToolValidation(func(request mcp.CallToolRequest) (*mcp.CallToolResult, bool) {
		if _, err := parsePositiveIntegerArgument(request.Params.Arguments["line"], "line"); err != nil {
			result, _ := OpErrorFromParseArg(err)
			return result, true
		}
		if _, err := parsePositiveIntegerArgument(request.Params.Arguments["column"], "column"); err != nil {
			result, _ := OpErrorFromParseArg(err)
			return result, true
		}
		return nil, false
	}, withLSPGuard(s.lspClient, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, _ := request.Params.Arguments["filePath"].(string)
		line, _ := parsePositiveIntegerArgument(request.Params.Arguments["line"], "line")
		column, _ := parsePositiveIntegerArgument(request.Params.Arguments["column"], "column")

		opCtx, cancel := handlerOperationContext(ctx)
		defer cancel()

		result, err := tools.GetNodeAtPosition(opCtx, s.lspClient, filePath, line, column)
		if err != nil {
			if deterministic := deterministicHandlerContextError("get_node_at_position", opCtx, err); deterministic != nil {
				return deterministic, nil
			}
			return OpErrorFromDomain("get_node_at_position", err, "verify filePath and line/column, then retry")
		}
		return mcp.NewToolResultText(result), nil
	}))))

	s.mcpServer.AddTool(mcp.NewTool("get_node_types",
		mcp.WithDescription("Retorna a lista de tipos de no suportados pelo Delphi 6"),
	), withToolLogging("get_node_types", withLSPGuard(s.lspClient, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		opCtx, cancel := handlerOperationContext(ctx)
		defer cancel()

		result, err := tools.GetNodeTypes(opCtx, s.lspClient)
		if err != nil {
			if deterministic := deterministicHandlerContextError("get_node_types", opCtx, err); deterministic != nil {
				return deterministic, nil
			}
			return OpErrorFromDomain("get_node_types", err, "verify LSP availability, then retry")
		}
		return mcp.NewToolResultText(result), nil
	})))

	s.mcpServer.AddTool(mcp.NewTool("onboarding",
		mcp.WithDescription("Escaneia estrutura do projeto Delphi e retorna inventario de units, forms e entry point"),
		mcp.WithString("projectPath", mcp.Required(), mcp.Description("Caminho absoluto do diretorio do projeto")),
		mcp.WithString("context", mcp.Description("Chave de contexto de onboarding (opcional; default quando omitido)")),
		mcp.WithBoolean("persistInProject",
			mcp.Description("Opt-in: quando true, grava .oracle-onboarding.json no workspace; padrao persiste fora do projeto")),
	), withToolLogging("onboarding", func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectPath, ok := request.Params.Arguments["projectPath"].(string)
		if !ok || strings.TrimSpace(projectPath) == "" {
			return OpValidationError("projectPath must be a non-empty string")
		}
		contextKey, err := parseOptionalContextArgument(request.Params.Arguments["context"])
		if err != nil {
			return OpErrorFromParseArg(err)
		}
		persistInProject, _ := request.Params.Arguments["persistInProject"].(bool)

		result, err := tools.PerformOnboardingWithContextAndOptions(
			ctx,
			projectPath,
			contextKey,
			tools.OnboardingOptions{PersistInProject: persistInProject},
		)
		if err != nil {
			return OpErrorFromDomain("onboarding", err, "verify projectPath and workspace layout, then retry")
		}
		return mcp.NewToolResultText(result), nil
	}))

	s.mcpServer.AddTool(mcp.NewTool("check_onboarding_performed",
		mcp.WithDescription("Verifica se o onboarding ja foi executado para este projeto"),
		mcp.WithString("projectPath", mcp.Required(), mcp.Description("Caminho absoluto do diretorio do projeto")),
		mcp.WithString("context", mcp.Description("Chave de contexto de onboarding (opcional; default quando omitido)")),
	), withToolLogging("check_onboarding_performed", func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectPath, ok := request.Params.Arguments["projectPath"].(string)
		if !ok || strings.TrimSpace(projectPath) == "" {
			return OpValidationError("projectPath must be a non-empty string")
		}
		contextKey, err := parseOptionalContextArgument(request.Params.Arguments["context"])
		if err != nil {
			return OpErrorFromParseArg(err)
		}

		performed, at := tools.CheckOnboardingPerformedWithContext(projectPath, contextKey)
		normalizedContext := strings.TrimSpace(contextKey)
		if normalizedContext == "" {
			normalizedContext = "default"
		}

		contextSuffix := ""
		if normalizedContext != "default" {
			contextSuffix = fmt.Sprintf(" (contexto: %s)", normalizedContext)
		}

		if !performed {
			_, readinessGuidance := tools.EvaluateOnboardingReadiness(projectPath)
			message := "Onboarding ainda nao foi executado para este projeto" + contextSuffix
			if strings.TrimSpace(readinessGuidance) != "" {
				message = message + ". " + readinessGuidance
			}
			return mcp.NewToolResultText(message), nil
		}
		performedMessage := fmt.Sprintf("Onboarding executado em: %s%s", at.Format(time.RFC3339), contextSuffix)
		performedMessage += ". Proximo passo: execute get_symbols_overview para mapear unidades/simbolos."
		return mcp.NewToolResultText(performedMessage), nil
	}))

	coreLogger.Info("Successfully registered all MCP tools")
	return nil
}
