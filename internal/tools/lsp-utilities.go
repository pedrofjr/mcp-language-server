package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// Gets the full code block surrounding the start of the input location
func GetFullDefinition(ctx context.Context, client *lsp.Client, startLocation protocol.Location) (string, protocol.Location, error) {
	var symbolRange protocol.Range
	found := false

	if client.SupportsDocumentSymbol() {
		symParams := protocol.DocumentSymbolParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: startLocation.URI,
			},
		}

		symResult, err := client.DocumentSymbol(ctx, symParams)
		if err != nil {
			if !isMethodNotSupportedError(err) {
				return "", protocol.Location{}, fmt.Errorf("failed to get document symbols: %w", err)
			}
		} else {
			symbols, resultErr := symResult.Results()
			if resultErr != nil {
				return "", protocol.Location{}, fmt.Errorf("failed to process document symbols: %w", resultErr)
			}

			symbolRange, found = findDocumentSymbolContainerRange(symbols, startLocation.Range.Start)
		}
	}

	if !found {
		symbolRange = startLocation.Range
		found = true
	}

	if found {
		filePath := startLocation.URI.Path()

		// Read the file to get the full lines of the definition
		// because we may have a start and end column
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", protocol.Location{}, fmt.Errorf("failed to read file: %w", err)
		}

		lines := strings.Split(string(content), "\n")
		if len(lines) == 0 {
			return "", protocol.Location{}, fmt.Errorf("file is empty")
		}

		maxLine := uint32(len(lines) - 1)
		if symbolRange.Start.Line > maxLine {
			symbolRange.Start.Line = maxLine
		}
		if symbolRange.End.Line > maxLine {
			symbolRange.End.Line = maxLine
		}
		if symbolRange.End.Line < symbolRange.Start.Line {
			symbolRange.End.Line = symbolRange.Start.Line
		}

		if delphiRange, ok := tryExpandDelphiRoutineDefinition(filePath, lines, startLocation.Range.Start); ok {
			symbolRange = delphiRange
		}

		// Extend start to beginning of line
		symbolRange.Start.Character = 0

		// Get the line at the end of the range
		if int(symbolRange.End.Line) >= len(lines) {
			return "", protocol.Location{}, fmt.Errorf("line number out of range")
		}

		line := lines[symbolRange.End.Line]
		trimmedLine := strings.TrimSpace(line)

		// In some cases (python), constant definitions do not include the full body and instead
		// end with an opening bracket. In this case, parse the file until the closing bracket
		if len(trimmedLine) > 0 {
			lastChar := trimmedLine[len(trimmedLine)-1]
			if lastChar == '(' || lastChar == '[' || lastChar == '{' || lastChar == '<' {
				// Find matching closing bracket
				bracketStack := []rune{rune(lastChar)}
				lineNum := symbolRange.End.Line + 1

				for lineNum < uint32(len(lines)) {
					line := lines[lineNum]
					for pos, char := range line {
						if char == '(' || char == '[' || char == '{' || char == '<' {
							bracketStack = append(bracketStack, char)
						} else if char == ')' || char == ']' || char == '}' || char == '>' {
							if len(bracketStack) > 0 {
								lastOpen := bracketStack[len(bracketStack)-1]
								if (lastOpen == '(' && char == ')') ||
									(lastOpen == '[' && char == ']') ||
									(lastOpen == '{' && char == '}') ||
									(lastOpen == '<' && char == '>') {
									bracketStack = bracketStack[:len(bracketStack)-1]
									if len(bracketStack) == 0 {
										// Found matching bracket - update range
										symbolRange.End.Line = lineNum
										symbolRange.End.Character = uint32(pos + 1)
										goto foundClosing
									}
								}
							}
						}
					}
					lineNum++
				}
			foundClosing:
			}
		}

		// Update location with new range
		startLocation.Range = symbolRange

		// Return the text within the range
		if int(symbolRange.End.Line) >= len(lines) {
			return "", protocol.Location{}, fmt.Errorf("end line out of range")
		}

		selectedLines := lines[symbolRange.Start.Line : symbolRange.End.Line+1]
		return strings.Join(selectedLines, "\n"), startLocation, nil
	}

	return "", protocol.Location{}, fmt.Errorf("symbol not found")
}

type delphiRoutineHeader struct {
	kind   string
	owner  string
	member string
}

type delphiConditionalBranch struct {
	active bool
}

func tryExpandDelphiRoutineDefinition(filePath string, lines []string, start protocol.Position) (protocol.Range, bool) {
	if !isDelphiWorkspaceReferenceFile(filePath) || len(lines) == 0 {
		return protocol.Range{}, false
	}

	startLine := int(start.Line)
	if startLine < 0 || startLine >= len(lines) {
		return protocol.Range{}, false
	}

	targetHeader, ok := parseDelphiRoutineHeader(lines[startLine])
	if !ok {
		return protocol.Range{}, false
	}

	implementationLine := findDelphiImplementationLine(lines)
	if implementationLine >= 0 && startLine < implementationLine {
		ownerRequired := false
		if targetHeader.owner != "" {
			ownerRequired = true
		} else if inferredOwner, inferred := inferDelphiInterfaceOwner(lines, startLine); inferred {
			targetHeader.owner = inferredOwner
			ownerRequired = true
		}

		headerLine, endLine, found := findMatchingDelphiImplementationBlock(lines, implementationLine, targetHeader, ownerRequired)
		if found {
			return protocol.Range{
				Start: protocol.Position{Line: uint32(headerLine), Character: 0},
				End:   protocol.Position{Line: uint32(endLine), Character: uint32(len(lines[endLine]))},
			}, true
		}
	}

	endLine, hasBody := findDelphiRoutineBlockEnd(lines, startLine)
	if !hasBody {
		return protocol.Range{}, false
	}

	return protocol.Range{
		Start: protocol.Position{Line: uint32(startLine), Character: 0},
		End:   protocol.Position{Line: uint32(endLine), Character: uint32(len(lines[endLine]))},
	}, true
}

func findDelphiImplementationLine(lines []string) int {
	for lineIndex, line := range lines {
		trimmedLine := strings.ToLower(strings.TrimSpace(sanitizePascalSearchLine(line)))
		if strings.HasPrefix(trimmedLine, "implementation") {
			return lineIndex
		}
	}

	return -1
}

func parseDelphiRoutineHeader(line string) (delphiRoutineHeader, bool) {
	trimmedLine := strings.TrimSpace(strings.ToLower(sanitizePascalSearchLine(line)))
	if trimmedLine == "" {
		return delphiRoutineHeader{}, false
	}

	if strings.HasPrefix(trimmedLine, "class ") {
		trimmedLine = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "class "))
	}

	for _, kind := range []string{"function", "procedure", "constructor", "destructor"} {
		prefix := kind + " "
		if !strings.HasPrefix(trimmedLine, prefix) {
			continue
		}

		namePortion := strings.TrimSpace(trimmedLine[len(prefix):])
		if namePortion == "" {
			return delphiRoutineHeader{}, false
		}

		nameEnd := len(namePortion)
		for index := 0; index < len(namePortion); index++ {
			if isDelphiHeaderNameByte(namePortion[index]) {
				continue
			}
			nameEnd = index
			break
		}

		normalizedName := strings.Trim(normalizeQualifiedSymbolSeparators(strings.TrimSpace(namePortion[:nameEnd])), ".")
		if normalizedName == "" {
			return delphiRoutineHeader{}, false
		}

		parts := strings.Split(normalizedName, ".")
		member := strings.TrimSpace(parts[len(parts)-1])
		if member == "" {
			return delphiRoutineHeader{}, false
		}

		owner := ""
		if len(parts) > 1 {
			owner = strings.Join(parts[:len(parts)-1], ".")
		}

		return delphiRoutineHeader{kind: kind, owner: owner, member: member}, true
	}

	return delphiRoutineHeader{}, false
}

func isDelphiHeaderNameByte(value byte) bool {
	if value >= 'a' && value <= 'z' {
		return true
	}
	if value >= 'A' && value <= 'Z' {
		return true
	}
	if value >= '0' && value <= '9' {
		return true
	}

	switch value {
	case '_', '.', ':':
		return true
	default:
		return false
	}
}

func inferDelphiInterfaceOwner(lines []string, startLine int) (string, bool) {
	for lineIndex := startLine - 1; lineIndex >= 0; lineIndex-- {
		trimmedLine := strings.ToLower(strings.TrimSpace(sanitizePascalSearchLine(lines[lineIndex])))
		if trimmedLine == "" {
			continue
		}

		if trimmedLine == "end" || trimmedLine == "end;" || strings.HasPrefix(trimmedLine, "implementation") || strings.HasPrefix(trimmedLine, "interface") {
			return "", false
		}

		if owner, ok := parseDelphiInterfaceOwnerDeclaration(trimmedLine); ok {
			return owner, true
		}
	}

	return "", false
}

func parseDelphiInterfaceOwnerDeclaration(line string) (string, bool) {
	equalsIndex := strings.Index(line, "=")
	if equalsIndex <= 0 {
		return "", false
	}

	owner := strings.TrimSpace(line[:equalsIndex])
	if !isSimpleDelphiIdentifier(owner) {
		return "", false
	}

	rightHandSide := strings.TrimSpace(line[equalsIndex+1:])
	if strings.HasPrefix(rightHandSide, "packed ") {
		rightHandSide = strings.TrimSpace(strings.TrimPrefix(rightHandSide, "packed "))
	}

	for _, prefix := range []string{"class", "record", "object", "interface"} {
		if strings.HasPrefix(rightHandSide, prefix) {
			return owner, true
		}
	}

	return "", false
}

func isSimpleDelphiIdentifier(value string) bool {
	if value == "" {
		return false
	}

	for index := 0; index < len(value); index++ {
		current := value[index]
		if (current >= 'a' && current <= 'z') || (current >= 'A' && current <= 'Z') || (current >= '0' && current <= '9') || current == '_' {
			continue
		}
		return false
	}

	return true
}

func findMatchingDelphiImplementationBlock(lines []string, implementationLine int, target delphiRoutineHeader, ownerRequired bool) (int, int, bool) {
	normalizedTargetOwner := normalizeQualifiedSymbolSeparators(target.owner)

	for lineIndex := implementationLine + 1; lineIndex < len(lines); lineIndex++ {
		candidate, ok := parseDelphiRoutineHeader(lines[lineIndex])
		if !ok {
			continue
		}

		if candidate.kind != target.kind || !strings.EqualFold(candidate.member, target.member) {
			continue
		}

		normalizedCandidateOwner := normalizeQualifiedSymbolSeparators(candidate.owner)
		if ownerRequired {
			if normalizedCandidateOwner == "" || !strings.EqualFold(normalizedCandidateOwner, normalizedTargetOwner) {
				continue
			}
		} else if normalizedCandidateOwner != "" {
			continue
		}

		endLine, hasBody := findDelphiRoutineBlockEnd(lines, lineIndex)
		if !hasBody {
			continue
		}

		return lineIndex, endLine, true
	}

	return 0, 0, false
}

func findDelphiRoutineBlockEnd(lines []string, headerLine int) (int, bool) {
	conditionalBranches := make([]delphiConditionalBranch, 0)
	bodyStarted := false
	pendingConditionalClose := false
	bodyDepth := 0

	for lineIndex := headerLine; lineIndex < len(lines); lineIndex++ {
		if keyword, ok := parseDelphiOwnLineDirectiveKeyword(lines[lineIndex]); ok {
			nextBranches, advanced := advanceDelphiConditionalBranches(conditionalBranches, keyword)
			if !advanced {
				return 0, false
			}

			conditionalBranches = nextBranches
			if pendingConditionalClose && len(conditionalBranches) == 0 {
				return lineIndex, true
			}
			continue
		}

		if !isActiveDelphiConditionalBranch(conditionalBranches) {
			continue
		}

		if pendingConditionalClose {
			if strings.TrimSpace(sanitizePascalSearchLine(lines[lineIndex])) == "" {
				continue
			}
			return 0, false
		}

		if !bodyStarted && lineIndex > headerLine {
			if _, isNestedHeader := parseDelphiRoutineHeader(lines[lineIndex]); isNestedHeader {
				return 0, false
			}
		}

		for _, token := range extractDelphiWordTokens(lines[lineIndex]) {
			if !bodyStarted {
				if isDelphiRoutineBodyStartToken(token) {
					bodyStarted = true
					bodyDepth++
					continue
				}

				if isDelphiRoutineBodyDisqualifier(token) {
					return 0, false
				}

				continue
			}

			switch token {
			case "begin", "case", "try", "asm":
				bodyDepth++
			case "end":
				if bodyDepth == 0 {
					continue
				}
				bodyDepth--
				if bodyDepth == 0 {
					if len(conditionalBranches) == 0 {
						return lineIndex, true
					}

					pendingConditionalClose = true
					break
				}
			}
		}
	}

	return 0, false
}

func parseDelphiOwnLineDirectiveKeyword(line string) (string, bool) {
	trimmedLine := strings.TrimSpace(trimSingleLineComment(line))
	if trimmedLine == "" {
		return "", false
	}

	if strings.HasPrefix(trimmedLine, "{$") && strings.HasSuffix(trimmedLine, "}") {
		trimmedLine = strings.TrimSpace(trimmedLine[2 : len(trimmedLine)-1])
	} else if strings.HasPrefix(trimmedLine, "(*$") && strings.HasSuffix(trimmedLine, "*)") {
		trimmedLine = strings.TrimSpace(trimmedLine[3 : len(trimmedLine)-2])
	} else {
		return "", false
	}

	fields := strings.Fields(strings.ToLower(trimmedLine))
	if len(fields) == 0 {
		return "", false
	}

	return fields[0], true
}

func advanceDelphiConditionalBranches(branches []delphiConditionalBranch, keyword string) ([]delphiConditionalBranch, bool) {
	switch keyword {
	case "if", "ifdef", "ifndef", "ifopt":
		return append(branches, delphiConditionalBranch{active: isActiveDelphiConditionalBranch(branches)}), true
	case "else", "elseif":
		if len(branches) == 0 {
			return nil, false
		}

		branches[len(branches)-1].active = false
		return branches, true
	case "endif", "ifend":
		if len(branches) == 0 {
			return nil, false
		}

		return branches[:len(branches)-1], true
	default:
		return branches, true
	}
}

func isActiveDelphiConditionalBranch(branches []delphiConditionalBranch) bool {
	if len(branches) == 0 {
		return true
	}

	return branches[len(branches)-1].active
}

func isDelphiRoutineBodyStartToken(token string) bool {
	switch token {
	case "begin", "case", "try", "asm":
		return true
	default:
		return false
	}
}

func isDelphiRoutineBodyDisqualifier(token string) bool {
	switch token {
	case "forward", "external", "message", "virtual", "abstract", "overload":
		return true
	default:
		return false
	}
}

func containsDelphiRoutineBodyStart(line string) bool {
	for _, token := range extractDelphiWordTokens(line) {
		if isDelphiRoutineBodyStartToken(token) {
			return true
		}
	}

	return false
}

func extractDelphiWordTokens(line string) []string {
	sanitizedLine := strings.ToLower(sanitizePascalSearchLine(line))
	tokens := make([]string, 0)
	tokenStart := -1

	for index := 0; index < len(sanitizedLine); index++ {
		if isSimpleDelphiTokenByte(sanitizedLine[index]) {
			if tokenStart < 0 {
				tokenStart = index
			}
			continue
		}

		if tokenStart >= 0 {
			tokens = append(tokens, sanitizedLine[tokenStart:index])
			tokenStart = -1
		}
	}

	if tokenStart >= 0 {
		tokens = append(tokens, sanitizedLine[tokenStart:])
	}

	return tokens
}

func isSimpleDelphiTokenByte(value byte) bool {
	return (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') || (value >= '0' && value <= '9') || value == '_'
}

func findDocumentSymbolContainerRange(symbols []protocol.DocumentSymbolResult, position protocol.Position) (protocol.Range, bool) {
	for _, sym := range symbols {
		r := sym.GetRange()
		if containsPosition(r, position) {
			return r, true
		}

		if ds, ok := sym.(*protocol.DocumentSymbol); ok && len(ds.Children) > 0 {
			childSymbols := make([]protocol.DocumentSymbolResult, len(ds.Children))
			for i := range ds.Children {
				childSymbols[i] = &ds.Children[i]
			}

			if childRange, childFound := findDocumentSymbolContainerRange(childSymbols, position); childFound {
				return childRange, true
			}
		}
	}

	return protocol.Range{}, false
}

// GetLineRangesToDisplay determines which lines should be displayed for a set of locations
func GetLineRangesToDisplay(ctx context.Context, client *lsp.Client, locations []protocol.Location, totalLines int, contextLines int) (map[int]bool, error) {
	// Set to track which lines need to be displayed
	linesToShow := make(map[int]bool)
	canUseDocumentSymbols := client.SupportsDocumentSymbol()
	documentSymbolCache := make(map[protocol.DocumentUri][]protocol.DocumentSymbolResult)
	documentSymbolDisabledByURI := make(map[protocol.DocumentUri]bool)

	// For each location, get its container and add relevant lines
	for _, loc := range locations {
		containerRange := protocol.Range{}
		foundContainer := false

		if canUseDocumentSymbols && !documentSymbolDisabledByURI[loc.URI] {
			symbols, cached := documentSymbolCache[loc.URI]
			if !cached {
				symResult, err := client.DocumentSymbol(ctx, protocol.DocumentSymbolParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: loc.URI},
				})
				if err != nil {
					if isMethodNotSupportedError(err) {
						canUseDocumentSymbols = false
					} else {
						toolsLogger.Debug("failed to get document symbols for %s: %v", loc.URI, err)
						documentSymbolDisabledByURI[loc.URI] = true
					}
					documentSymbolCache[loc.URI] = nil
				} else {
					results, resultErr := symResult.Results()
					if resultErr != nil {
						toolsLogger.Debug("failed to parse document symbols for %s: %v", loc.URI, resultErr)
						documentSymbolDisabledByURI[loc.URI] = true
						documentSymbolCache[loc.URI] = nil
					} else {
						documentSymbolCache[loc.URI] = results
					}
				}

				symbols = documentSymbolCache[loc.URI]
			}

			if len(symbols) > 0 {
				if resolvedRange, ok := findDocumentSymbolContainerRange(symbols, loc.Range.Start); ok {
					containerRange = resolvedRange
					foundContainer = true
				}
			}
		}

		if !foundContainer {
			// If container not found, just use the location's line
			refLine := int(loc.Range.Start.Line)
			linesToShow[refLine] = true

			// Add context lines
			for i := refLine - contextLines; i <= refLine+contextLines; i++ {
				if i >= 0 && i < totalLines {
					linesToShow[i] = true
				}
			}
			continue
		}

		// Add container start and end lines
		containerStart := int(containerRange.Start.Line)
		containerEnd := int(containerRange.End.Line)
		linesToShow[containerStart] = true
		// linesToShow[containerEnd] = true

		// Add the reference line
		refLine := int(loc.Range.Start.Line)
		linesToShow[refLine] = true

		// Add context lines around the reference
		for i := refLine - contextLines; i <= refLine+contextLines; i++ {
			if i >= 0 && i < totalLines && i >= containerStart && i <= containerEnd {
				linesToShow[i] = true
			}
		}
	}

	return linesToShow, nil
}
