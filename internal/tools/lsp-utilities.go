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
