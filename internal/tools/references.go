package tools

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

func FindReferences(ctx context.Context, client *lsp.Client, symbolName string) (string, error) {
	// Get context lines from environment variable
	contextLines := 5
	if envLines := os.Getenv("LSP_CONTEXT_LINES"); envLines != "" {
		if val, err := strconv.Atoi(envLines); err == nil && val >= 0 {
			contextLines = val
		}
	}

	symbolLocations, err := resolveReferenceSymbolLocations(ctx, client, symbolName)
	if err != nil {
		return "", err
	}
	if len(symbolLocations) == 0 {
		return fmt.Sprintf("No references found for symbol: %s", symbolName), nil
	}

	var allReferences []string
	attemptedLocations := make(map[string]struct{}, len(symbolLocations))
	for _, loc := range symbolLocations {
		attemptedLocations[referenceLocationKey(loc)] = struct{}{}

		referenceBlocks, err := collectReferenceBlocks(ctx, client, loc, contextLines)
		if err != nil {
			return "", err
		}

		allReferences = append(allReferences, referenceBlocks...)
	}

	if len(allReferences) == 0 {
		retryLocations := collectOpenFileReferenceRetryLocations(client, symbolName, attemptedLocations)
		for _, loc := range retryLocations {
			attemptedLocations[referenceLocationKey(loc)] = struct{}{}

			referenceBlocks, err := collectReferenceBlocks(ctx, client, loc, contextLines)
			if err != nil {
				return "", err
			}
			if len(referenceBlocks) == 0 {
				continue
			}

			allReferences = append(allReferences, referenceBlocks...)
			break
		}
	}

	if len(allReferences) == 0 {
		referenceBlocks, err := collectLastResortOpenFileReferenceBlocks(ctx, client, symbolName, symbolLocations, contextLines)
		if err != nil {
			return "", err
		}

		allReferences = append(allReferences, referenceBlocks...)
	}

	if len(allReferences) == 0 {
		return fmt.Sprintf("No references found for symbol: %s", symbolName), nil
	}

	return strings.Join(allReferences, "\n"), nil
}

func resolveReferenceSymbolLocations(ctx context.Context, client *lsp.Client, symbolName string) ([]protocol.Location, error) {
	if !client.SupportsWorkspaceSymbol() {
		inferredLocation, found := inferSymbolLocationFromOpenFiles(client, symbolName)
		if !found {
			return nil, nil
		}

		return []protocol.Location{inferredLocation}, nil
	}

	symbolResult, err := client.Symbol(ctx, protocol.WorkspaceSymbolParams{Query: symbolName})
	if err != nil {
		if !isMethodNotSupportedError(err) {
			return nil, fmt.Errorf("failed to fetch symbol: %v", err)
		}

		inferredLocation, found := inferSymbolLocationFromOpenFiles(client, symbolName)
		if !found {
			return nil, nil
		}

		return []protocol.Location{inferredLocation}, nil
	}

	results, err := symbolResult.Results()
	if err != nil {
		return nil, fmt.Errorf("failed to parse results: %v", err)
	}
	if len(results) == 0 {
		inferredLocation, found := inferSymbolLocationFromOpenFiles(client, symbolName)
		if !found {
			return nil, nil
		}

		return []protocol.Location{inferredLocation}, nil
	}

	locations := make([]protocol.Location, 0, len(results))
	for _, symbol := range results {
		if isQualifiedSymbolQuery(symbolName) {
			symbolInfo, ok := symbol.(*protocol.SymbolInformation)
			if !ok {
				if symbol.GetName() != symbolName {
					continue
				}
			} else if !matchesQualifiedWorkspaceSymbol(symbolName, symbol.GetName(), symbolInfo.ContainerName) {
				continue
			}
		} else if symbol.GetName() != symbolName {
			continue
		}

		locations = append(locations, symbol.GetLocation())
	}

	return locations, nil
}

func collectReferenceBlocks(ctx context.Context, client *lsp.Client, loc protocol.Location, contextLines int) ([]string, error) {
	if err := client.OpenFile(ctx, loc.URI.Path()); err != nil {
		toolsLogger.Error("Error opening file: %v", err)
		return nil, nil
	}

	refsParams := protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: loc.URI,
			},
			Position: loc.Range.Start,
		},
		Context: protocol.ReferenceContext{
			IncludeDeclaration: false,
		},
	}
	refs, err := client.References(ctx, refsParams)
	if err != nil {
		return nil, fmt.Errorf("failed to get references: %v", err)
	}

	return formatReferenceBlocks(ctx, client, refs, contextLines)
}

func formatReferenceBlocks(ctx context.Context, client *lsp.Client, refs []protocol.Location, contextLines int) ([]string, error) {

	refsByFile := make(map[protocol.DocumentUri][]protocol.Location)
	for _, ref := range refs {
		refsByFile[ref.URI] = append(refsByFile[ref.URI], ref)
	}

	uris := make([]string, 0, len(refsByFile))
	for uri := range refsByFile {
		uris = append(uris, string(uri))
	}
	sort.Strings(uris)

	allReferences := make([]string, 0, len(uris))
	for _, uriStr := range uris {
		uri := protocol.DocumentUri(uriStr)
		fileRefs := refsByFile[uri]
		filePath, ok := uriPathFromString(uriStr)
		if !ok {
			filePath = uriStr
		}

		fileInfo := fmt.Sprintf("---\n\n%s\nReferences in File: %d\n",
			filePath,
			len(fileRefs),
		)

		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			allReferences = append(allReferences, fileInfo+"\nError reading file: "+err.Error())
			continue
		}

		lines := strings.Split(string(fileContent), "\n")

		var locStrings []string
		for _, ref := range fileRefs {
			locStr := fmt.Sprintf("L%d:C%d",
				ref.Range.Start.Line+1,
				ref.Range.Start.Character+1)
			locStrings = append(locStrings, locStr)
		}

		linesToShow, err := GetLineRangesToDisplay(ctx, client, fileRefs, len(lines), contextLines)
		if err != nil {
			continue
		}

		lineRanges := ConvertLinesToRanges(linesToShow, len(lines))

		formattedOutput := fileInfo
		if len(locStrings) > 0 {
			formattedOutput += "At: " + strings.Join(locStrings, ", ") + "\n"
		}

		formattedOutput += "\n" + FormatLinesWithRanges(lines, lineRanges)
		allReferences = append(allReferences, formattedOutput)
	}

	return allReferences, nil
}

func collectLastResortOpenFileReferenceBlocks(ctx context.Context, client *lsp.Client, symbolName string, symbolLocations []protocol.Location, contextLines int) ([]string, error) {
	textualLocations := collectLastResortOpenFileReferenceLocations(client, symbolName, symbolLocations)
	if len(textualLocations) == 0 {
		return nil, nil
	}

	return formatReferenceBlocks(ctx, client, textualLocations, contextLines)
}

func collectLastResortOpenFileReferenceLocations(client *lsp.Client, symbolName string, symbolLocations []protocol.Location) []protocol.Location {
	searchPatterns := buildSymbolSearchPatterns(symbolName)
	if len(searchPatterns) == 0 {
		return nil
	}

	declarationLines := make(map[string]struct{}, len(symbolLocations))
	for _, loc := range symbolLocations {
		declarationLines[referenceLineKey(loc)] = struct{}{}
	}

	openFiles := client.GetOpenFilesSnapshot()
	sort.Strings(openFiles)

	locations := make([]protocol.Location, 0, len(openFiles))
	for _, uriStr := range openFiles {
		path, ok := uriPathFromString(uriStr)
		if !ok {
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		locations = append(locations, findOpenFileTextualReferenceLocations(protocol.DocumentUri(uriStr), lines, searchPatterns, declarationLines)...)
	}

	return locations
}

func collectOpenFileReferenceRetryLocations(client *lsp.Client, symbolName string, attemptedLocations map[string]struct{}) []protocol.Location {
	searchPatterns := buildSymbolSearchPatterns(symbolName)
	if len(searchPatterns) == 0 {
		return nil
	}

	openFiles := client.GetOpenFilesSnapshot()
	retryLocations := make([]protocol.Location, 0, len(openFiles))

	for _, uriStr := range openFiles {
		path, ok := uriPathFromString(uriStr)
		if !ok {
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		candidate, found := findOpenFileReferenceRetryLocation(protocol.DocumentUri(uriStr), lines, searchPatterns, attemptedLocations)
		if !found {
			continue
		}

		retryLocations = append(retryLocations, candidate)
	}

	return retryLocations
}

func findOpenFileReferenceRetryLocation(uri protocol.DocumentUri, lines []string, searchPatterns []*regexp.Regexp, attemptedLocations map[string]struct{}) (protocol.Location, bool) {
	for lineIndex, line := range lines {
		searchLine := trimSingleLineComment(line)
		for _, pattern := range searchPatterns {
			matchRange := pattern.FindStringIndex(searchLine)
			if matchRange == nil {
				continue
			}

			candidate := protocol.Location{
				URI: uri,
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(lineIndex), Character: uint32(matchRange[0])},
					End:   protocol.Position{Line: uint32(lineIndex), Character: uint32(matchRange[1])},
				},
			}
			if _, alreadyTried := attemptedLocations[referenceLocationKey(candidate)]; alreadyTried {
				continue
			}

			return candidate, true
		}
	}

	return protocol.Location{}, false
}

func findOpenFileTextualReferenceLocations(uri protocol.DocumentUri, lines []string, searchPatterns []*regexp.Regexp, declarationLines map[string]struct{}) []protocol.Location {
	locations := make([]protocol.Location, 0)
	seenLocations := make(map[string]struct{})

	for lineIndex, line := range lines {
		searchLine := trimSingleLineComment(line)
		_, declarationLine := declarationLines[referenceLineKeyForURI(uri, lineIndex)]

		for _, pattern := range searchPatterns {
			matchRanges := pattern.FindAllStringIndex(searchLine, -1)
			for _, matchRange := range matchRanges {
				if declarationLine && scorePotentialDeclarationLine(searchLine) > 0 {
					continue
				}

				candidate := protocol.Location{
					URI: uri,
					Range: protocol.Range{
						Start: protocol.Position{Line: uint32(lineIndex), Character: uint32(matchRange[0])},
						End:   protocol.Position{Line: uint32(lineIndex), Character: uint32(matchRange[1])},
					},
				}

				locationKey := referenceLocationKey(candidate)
				if _, seen := seenLocations[locationKey]; seen {
					continue
				}

				seenLocations[locationKey] = struct{}{}
				locations = append(locations, candidate)
			}
		}
	}

	return locations
}

func referenceLocationKey(loc protocol.Location) string {
	return fmt.Sprintf("%s:%d:%d:%d:%d",
		loc.URI,
		loc.Range.Start.Line,
		loc.Range.Start.Character,
		loc.Range.End.Line,
		loc.Range.End.Character,
	)
}

func referenceLineKey(loc protocol.Location) string {
	return referenceLineKeyForURI(loc.URI, int(loc.Range.Start.Line))
}

func referenceLineKeyForURI(uri protocol.DocumentUri, line int) string {
	return fmt.Sprintf("%s:%d", uri, line)
}
