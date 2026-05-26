package tools

import (
	"context"
	"errors"
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

	lines := strings.Split(src, "\n")
	inClassHeritage := false
	classLine := 0
	className := ""
	var heritageBuilder strings.Builder

	for lineIndex, line := range lines {
		lineWithoutComment := line
		if commentIndex := strings.Index(lineWithoutComment, "//"); commentIndex >= 0 {
			lineWithoutComment = lineWithoutComment[:commentIndex]
		}

		trimmedLower := strings.ToLower(strings.TrimSpace(lineWithoutComment))
		if !inClassHeritage {
			if !strings.Contains(trimmedLower, "= class(") && !strings.Contains(trimmedLower, "= class (") {
				continue
			}

			equalIndex := strings.Index(lineWithoutComment, "=")
			if equalIndex <= 0 {
				continue
			}

			className = strings.TrimSpace(lineWithoutComment[:equalIndex])
			if className == "" {
				continue
			}

			openParenIndex := strings.Index(lineWithoutComment, "(")
			if openParenIndex < 0 {
				continue
			}

			inClassHeritage = true
			classLine = lineIndex + 1
			heritageBuilder.Reset()
			heritageBuilder.WriteString(lineWithoutComment[openParenIndex+1:])
		} else {
			if heritageBuilder.Len() > 0 {
				heritageBuilder.WriteByte('\n')
			}
			heritageBuilder.WriteString(lineWithoutComment)
		}

		heritageText := heritageBuilder.String()
		closeParenIndex := strings.Index(heritageText, ")")
		if closeParenIndex < 0 {
			continue
		}

		heritageLower := strings.ToLower(heritageText[:closeParenIndex])
		parts := strings.Split(heritageLower, ",")
		for partIndex, part := range parts {
			if partIndex == 0 {
				continue
			}
			if strings.TrimSpace(part) != lowerIface {
				continue
			}

			results = append(results, fmt.Sprintf("Line %d: %s implements %s", classLine, className, interfaceName))
			break
		}

		inClassHeritage = false
		classLine = 0
		className = ""
		heritageBuilder.Reset()
	}

	return results
}

// FindImplementations procura implementacoes de uma interface no workspace.
// workspaceDir e o diretorio raiz do workspace; se vazio, usa o diretorio do arquivo.
func FindImplementations(ctx context.Context, client *lsp.Client, filePath, symbolName, workspaceDir string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if client != nil {
		lspResult, err := findImplementationsViaLSP(ctx, client, filePath, symbolName)
		if err == nil && strings.TrimSpace(lspResult) != "" {
			return lspResult, nil
		}
		if err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
			return "", err
		}
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	searchDir := workspaceDir
	if searchDir == "" {
		searchDir = filepath.Dir(filePath)
	}
	return findImplementationsInDir(searchDir, symbolName)
}

func findImplementationsViaLSP(ctx context.Context, client *lsp.Client, filePath, symbolName string) (string, error) {
	if client == nil {
		return "", fmt.Errorf("lsp client is nil")
	}

	normalizedPath, err := normalizeFilePathOrURI(filePath)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(normalizedPath)
	if err != nil {
		return "", err
	}

	line, column, err := findSymbolPositionExact(string(content), symbolName)
	if err != nil {
		return "", fmt.Errorf("simbolo %q nao encontrado com token exato em %s", symbolName, normalizedPath)
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

func findSymbolPositionExact(src, symbolName string) (int, int, error) {
	trimmedSymbol := strings.TrimSpace(symbolName)
	if trimmedSymbol == "" {
		return -1, -1, fmt.Errorf("symbol name is empty")
	}

	searchCandidates := []string{trimmedSymbol}
	if dot := strings.LastIndex(trimmedSymbol, "."); dot >= 0 && dot+1 < len(trimmedSymbol) {
		simpleName := trimmedSymbol[dot+1:]
		if simpleName != "" && !strings.EqualFold(simpleName, trimmedSymbol) {
			searchCandidates = append(searchCandidates, simpleName)
		}
	}

	for lineIndex, sourceLine := range strings.Split(src, "\n") {
		lineWithoutComment := sourceLine
		if commentIndex := strings.Index(lineWithoutComment, "//"); commentIndex >= 0 {
			lineWithoutComment = lineWithoutComment[:commentIndex]
		}

		for _, candidate := range searchCandidates {
			if candidate == "" {
				continue
			}

			lowerLine := strings.ToLower(lineWithoutComment)
			lowerCandidate := strings.ToLower(candidate)
			searchFrom := 0

			for {
				matchOffset := strings.Index(lowerLine[searchFrom:], lowerCandidate)
				if matchOffset < 0 {
					break
				}

				matchIndex := searchFrom + matchOffset
				matchEnd := matchIndex + len(candidate)

				if isIdentifierBoundary(lineWithoutComment, matchIndex-1) && isIdentifierBoundary(lineWithoutComment, matchEnd) {
					return lineIndex, matchIndex, nil
				}

				searchFrom = matchIndex + len(candidate)
				if searchFrom >= len(lowerLine) {
					break
				}
			}
		}
	}

	return -1, -1, fmt.Errorf("exact token not found")
}

func isIdentifierBoundary(line string, index int) bool {
	if index < 0 || index >= len(line) {
		return true
	}

	b := line[index]
	if b == '_' {
		return false
	}
	if b >= '0' && b <= '9' {
		return false
	}
	if b >= 'A' && b <= 'Z' {
		return false
	}
	if b >= 'a' && b <= 'z' {
		return false
	}

	return true
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
