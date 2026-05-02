package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

func ReadDefinition(ctx context.Context, client *lsp.Client, symbolName string) (string, error) {
	if !client.SupportsWorkspaceSymbol() {
		if _, found := inferDelphiSymbolLocation(client, symbolName); found {
			return readDefinitionWithDelphiInferredPosition(ctx, client, symbolName)
		}

		return readDefinitionWithInferredPosition(ctx, client, symbolName)
	}

	symbolResult, err := client.Symbol(ctx, protocol.WorkspaceSymbolParams{
		Query: symbolName,
	})
	if err != nil {
		if isMethodNotSupportedError(err) {
			return readDefinitionWithInferredPosition(ctx, client, symbolName)
		}
		return "", fmt.Errorf("failed to fetch symbol: %v", err)
	}

	results, err := symbolResult.Results()
	if err != nil {
		return "", fmt.Errorf("failed to parse results: %v", err)
	}

	type queryDefinitionCandidate struct {
		displayName string
		loc         protocol.Location
		kind        string
		container   string
		rank        int
	}

	qualifiedDepth := qualifiedSymbolQueryDepth(symbolName)
	var candidates []queryDefinitionCandidate
	matchedWorkspaceSymbol := false
	for _, symbol := range results {
		kind := ""
		container := ""
		displayName := symbol.GetName()
		symbolKind := protocol.SymbolKind(0)

		// Skip symbols that we are not looking for. workspace/symbol may return
		// a large number of fuzzy matches.
		switch v := symbol.(type) {
		case *protocol.SymbolInformation:
			symbolKind = v.Kind
			// SymbolInformation results have richer data.
			kind = fmt.Sprintf("Kind: %s\n", protocol.TableKindMap[v.Kind])
			if v.ContainerName != "" {
				container = fmt.Sprintf("Container Name: %s\n", v.ContainerName)
			}

			// Handle different matching strategies based on the search term
			if isQualifiedSymbolQuery(symbolName) {
				if !matchesQualifiedWorkspaceSymbol(symbolName, symbol.GetName(), v.ContainerName) {
					continue
				}
				displayName = symbolName
			} else {
				// For unqualified names like "Method"
				if v.Kind == protocol.Method {
					// For methods, only match if the method name matches exactly Type.symbolName or Type::symbolName or symbolName
					if !strings.HasSuffix(symbol.GetName(), "::"+symbolName) && !strings.HasSuffix(symbol.GetName(), "."+symbolName) && symbol.GetName() != symbolName {
						continue
					}
				} else if symbol.GetName() != symbolName {
					// For non-methods, exact match only
					continue
				}
			}
		default:
			if symbol.GetName() != symbolName {
				continue
			}
		}

		matchedWorkspaceSymbol = true

		toolsLogger.Debug("Found symbol: %s", symbol.GetName())
		candidates = append(candidates, queryDefinitionCandidate{
			displayName: displayName,
			loc:         symbol.GetLocation(),
			kind:        kind,
			container:   container,
			rank:        rankWorkspaceSymbolForDefinition(symbolKind, qualifiedDepth),
		})
	}

	if len(candidates) == 0 {
		if !matchedWorkspaceSymbol || isQualifiedSymbolQuery(symbolName) {
			return readDefinitionWithDelphiInferredPosition(ctx, client, symbolName)
		}

		return fmt.Sprintf("%s not found", symbolName), nil
	}

	selectedCandidates := candidates
	if qualifiedDepth >= 2 {
		best := candidates[0]
		for _, candidate := range candidates[1:] {
			if candidate.rank > best.rank {
				best = candidate
			}
		}

		selectedCandidates = []queryDefinitionCandidate{best}
	}

	var definitions []string
	for _, candidate := range selectedCandidates {
		definitionText, defErr := buildDefinitionBlock(ctx, client, candidate.displayName, candidate.loc, candidate.kind, candidate.container)
		if defErr != nil {
			toolsLogger.Error("Error getting definition: %v", defErr)
			continue
		}

		definitions = append(definitions, definitionText)
	}

	if len(definitions) == 0 {
		if isQualifiedSymbolQuery(symbolName) {
			return readDefinitionWithDelphiInferredPosition(ctx, client, symbolName)
		}

		return fmt.Sprintf("%s not found", symbolName), nil
	}

	return strings.Join(definitions, ""), nil
}

func qualifiedSymbolQueryDepth(symbolName string) int {
	normalized := strings.Trim(normalizeQualifiedSymbolSeparators(symbolName), ".")
	if normalized == "" {
		return 0
	}

	segments := strings.Split(normalized, ".")
	depth := 0
	for _, segment := range segments {
		if strings.TrimSpace(segment) != "" {
			depth++
		}
	}

	return depth
}

func rankWorkspaceSymbolForDefinition(kind protocol.SymbolKind, qualifiedDepth int) int {
	if qualifiedDepth < 2 {
		return 0
	}

	if qualifiedDepth == 2 {
		switch kind {
		case protocol.Class, protocol.Interface, protocol.Struct, protocol.Enum, protocol.TypeParameter:
			return 300
		case protocol.Method, protocol.Constructor, protocol.Function, protocol.Property, protocol.Field:
			return 200
		default:
			return 100
		}
	}

	switch kind {
	case protocol.Method, protocol.Constructor, protocol.Function, protocol.Property, protocol.Field:
		return 300
	case protocol.Class, protocol.Interface, protocol.Struct, protocol.Enum, protocol.TypeParameter:
		return 200
	default:
		return 100
	}
}

func readDefinitionWithDelphiInferredPosition(ctx context.Context, client *lsp.Client, symbolName string) (string, error) {
	inferredLocation, found := inferDelphiSymbolLocation(client, symbolName)
	if !found {
		return fmt.Sprintf("%s not found", symbolName), nil
	}

	defResult, err := client.Definition(ctx, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: inferredLocation.URI},
			Position:     inferredLocation.Range.Start,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch definition fallback: %v", err)
	}

	locations := definitionResultToLocations(defResult)
	if len(locations) == 0 {
		locations = []protocol.Location{inferredLocation}
	} else if !locationMatchesDelphiSymbol(locations[0], symbolName) {
		locations = []protocol.Location{inferredLocation}
	}

	definitionText, buildErr := buildDefinitionBlock(ctx, client, symbolName, locations[0], "", "")
	if buildErr != nil {
		return "", fmt.Errorf("failed to build fallback definition: %v", buildErr)
	}

	return definitionText, nil
}

func isQualifiedSymbolQuery(symbolName string) bool {
	return strings.Contains(symbolName, ".") || strings.Contains(symbolName, "::")
}

func matchesQualifiedWorkspaceSymbol(query string, name string, container string) bool {
	if name == query {
		return true
	}

	if container == "" {
		return false
	}

	return container+"."+name == query || container+"::"+name == query
}

func readDefinitionWithInferredPosition(ctx context.Context, client *lsp.Client, symbolName string) (string, error) {
	inferredLocation, found := inferSymbolLocationFromOpenFiles(client, symbolName)
	if !found {
		return fmt.Sprintf("%s not found", symbolName), nil
	}

	defResult, err := client.Definition(ctx, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: inferredLocation.URI},
			Position:     inferredLocation.Range.Start,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch definition fallback: %v", err)
	}

	locations := definitionResultToLocations(defResult)
	if len(locations) == 0 {
		locations = []protocol.Location{inferredLocation}
	}

	definitionText, buildErr := buildDefinitionBlock(ctx, client, symbolName, locations[0], "", "")
	if buildErr != nil {
		return "", fmt.Errorf("failed to build fallback definition: %v", buildErr)
	}

	return definitionText, nil
}

func buildDefinitionBlock(ctx context.Context, client *lsp.Client, symbolName string, loc protocol.Location, kind string, container string) (string, error) {
	if err := client.OpenFile(ctx, loc.URI.Path()); err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}

	definition, fullLocation, err := GetFullDefinition(ctx, client, loc)
	if err != nil {
		return "", err
	}

	fileDisplayPath := string(fullLocation.URI)
	if normalizedPath, ok := uriPathFromString(fileDisplayPath); ok {
		fileDisplayPath = normalizedPath
	}

	banner := "---\n\n"
	locationInfo := fmt.Sprintf(
		"Symbol: %s\n"+
			"File: %s\n"+
			kind+
			container+
			"Range: L%d:C%d - L%d:C%d\n\n",
		symbolName,
		fileDisplayPath,
		fullLocation.Range.Start.Line+1,
		fullLocation.Range.Start.Character+1,
		fullLocation.Range.End.Line+1,
		fullLocation.Range.End.Character+1,
	)

	definition = addLineNumbers(definition, int(fullLocation.Range.Start.Line)+1)
	return banner + locationInfo + definition + "\n", nil
}
