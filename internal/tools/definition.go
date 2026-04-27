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

	var definitions []string
	matchedWorkspaceSymbol := false
	for _, symbol := range results {
		kind := ""
		container := ""
		displayName := symbol.GetName()

		// Skip symbols that we are not looking for. workspace/symbol may return
		// a large number of fuzzy matches.
		switch v := symbol.(type) {
		case *protocol.SymbolInformation:
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
		loc := symbol.GetLocation()

		definitionText, defErr := buildDefinitionBlock(ctx, client, displayName, loc, kind, container)
		if defErr != nil {
			toolsLogger.Error("Error getting definition: %v", defErr)
			continue
		}

		definitions = append(definitions, definitionText)
	}

	if len(definitions) == 0 {
		if !matchedWorkspaceSymbol || isQualifiedSymbolQuery(symbolName) {
			return readDefinitionWithDelphiInferredPosition(ctx, client, symbolName)
		}

		return fmt.Sprintf("%s not found", symbolName), nil
	}

	return strings.Join(definitions, ""), nil
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
