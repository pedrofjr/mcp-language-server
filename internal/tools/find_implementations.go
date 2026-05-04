package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// findImplementationsTextScan busca classes que implementam uma interface via scan textual.
// A heranca em Delphi e: TMyClass = class(TObject, IFoo, IBar).
// A funcao retorna uma lista textual com linha e classe encontrada.
func findImplementationsTextScan(src, interfaceName string) []string {
	var results []string

	lowerIface := strings.ToLower(strings.TrimSpace(interfaceName))
	if lowerIface == "" {
		return results
	}

	for lineIndex, line := range strings.Split(src, "\n") {
		lineWithoutComment := line
		if commentIndex := strings.Index(lineWithoutComment, "//"); commentIndex >= 0 {
			lineWithoutComment = lineWithoutComment[:commentIndex]
		}

		lower := strings.ToLower(strings.TrimSpace(lineWithoutComment))
		if !strings.Contains(lower, "= class(") && !strings.Contains(lower, "= class (") {
			continue
		}

		start := strings.Index(lower, "(")
		if start < 0 {
			continue
		}

		endRel := strings.Index(lower[start:], ")")
		if endRel < 0 {
			continue
		}

		heritage := lower[start+1 : start+endRel]
		parts := strings.Split(heritage, ",")
		for partIndex, part := range parts {
			if partIndex == 0 {
				continue
			}
			if strings.TrimSpace(part) != lowerIface {
				continue
			}

			equalIndex := strings.Index(lineWithoutComment, "=")
			if equalIndex <= 0 {
				continue
			}

			className := strings.TrimSpace(lineWithoutComment[:equalIndex])
			if className == "" {
				continue
			}

			results = append(results, fmt.Sprintf("Line %d: %s implements %s", lineIndex+1, className, interfaceName))
			break
		}
	}

	return results
}

// FindImplementations procura implementacoes de uma interface no workspace.
func FindImplementations(ctx context.Context, client *lsp.Client, filePath, symbolName string) (string, error) {
	lspResult, err := findImplementationsViaLSP(ctx, client, filePath, symbolName)
	if err == nil && strings.TrimSpace(lspResult) != "" {
		return lspResult, nil
	}

	dir := filepath.Dir(filePath)
	return findImplementationsInDir(dir, symbolName)
}

func findImplementationsViaLSP(ctx context.Context, client *lsp.Client, filePath, symbolName string) (string, error) {
	normalizedPath, err := normalizeFilePathOrURI(filePath)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(normalizedPath)
	if err != nil {
		return "", err
	}

	line := -1
	column := -1
	for lineIndex, sourceLine := range strings.Split(string(content), "\n") {
		lowerLine := strings.ToLower(sourceLine)
		matchIndex := strings.Index(lowerLine, strings.ToLower(symbolName))
		if matchIndex >= 0 {
			line = lineIndex
			column = matchIndex
			break
		}
	}

	if line < 0 {
		return "", fmt.Errorf("simbolo %q nao encontrado em %s", symbolName, normalizedPath)
	}

	result, err := client.Implementation(ctx, protocol.ImplementationParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.URIFromPath(normalizedPath)},
			Position: protocol.Position{
				Line:      uint32(line),
				Character: uint32(column),
			},
		},
	})
	if err != nil {
		return "", err
	}

	locations := implementationResultToLocations(result)
	if len(locations) == 0 {
		return "", nil
	}

	lines := make([]string, 0, len(locations))
	for _, loc := range locations {
		lines = append(lines, fmt.Sprintf("%s:L%d:C%d", loc.URI, loc.Range.Start.Line+1, loc.Range.Start.Character+1))
	}
	return strings.Join(lines, "\n"), nil
}

func implementationResultToLocations(result protocol.Or_Result_textDocument_implementation) []protocol.Location {
	if result.Value == nil {
		return nil
	}

	switch value := result.Value.(type) {
	case protocol.Definition:
		switch definition := value.Value.(type) {
		case protocol.Location:
			return []protocol.Location{definition}
		case []protocol.Location:
			return definition
		}
	case []protocol.DefinitionLink:
		locations := make([]protocol.Location, 0, len(value))
		for _, link := range value {
			locations = append(locations, protocol.Location{URI: link.TargetURI, Range: link.TargetRange})
		}
		return locations
	case protocol.Location:
		return []protocol.Location{value}
	case []protocol.Location:
		return value
	}

	return nil
}

func findImplementationsInDir(dir, symbolName string) (string, error) {
	var results []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info == nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".pas") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		found := findImplementationsTextScan(string(content), symbolName)
		for _, entry := range found {
			results = append(results, fmt.Sprintf("%s: %s", filepath.Base(path), entry))
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return fmt.Sprintf("Nenhuma implementacao de %q encontrada", symbolName), nil
	}

	return strings.Join(results, "\n"), nil
}
