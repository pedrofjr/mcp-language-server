package tools

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

var (
	ErrDelphiSymbolNotFound  = errors.New("delphi symbol not found")
	ErrDelphiSymbolAmbiguous = errors.New("delphi symbol ambiguous")
)

type delphiRoutineBoundaries struct {
	headerLine int // 1-indexed declaration line
	beginLine  int // 1-indexed outer begin
	endLine    int // 1-indexed end;
}

func resolveDelphiRoutineBoundaries(src, symbolName string) (*delphiRoutineBoundaries, error) {
	lines := strings.Split(src, "\n")
	trimmedName := strings.TrimSpace(symbolName)
	if trimmedName == "" {
		return nil, fmt.Errorf("%w: empty symbol name", ErrDelphiSymbolNotFound)
	}

	qualifiedQuery := strings.Contains(normalizeQualifiedSymbolSeparators(trimmedName), ".")
	implementationLine := findDelphiImplementationLine(lines)

	matches := make([]delphiRoutineBoundaries, 0, 1)
	for lineIndex, line := range lines {
		header, ok := parseDelphiRoutineHeader(line)
		if !ok || !delphiRoutineHeaderMatchesSymbol(header, trimmedName, qualifiedQuery) {
			continue
		}

		bounded, ok := resolveDelphiRoutineBoundariesForHeader(lines, lineIndex, header, implementationLine)
		if !ok {
			continue
		}
		matches = append(matches, bounded)
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("%w: %q", ErrDelphiSymbolNotFound, trimmedName)
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("%w: %q matched %d routines; use a qualified name", ErrDelphiSymbolAmbiguous, trimmedName, len(matches))
	}
}

func delphiRoutineHeaderMatchesSymbol(header delphiRoutineHeader, symbolName string, qualifiedQuery bool) bool {
	if qualifiedQuery {
		normalizedTarget := strings.ToLower(strings.Trim(normalizeQualifiedSymbolSeparators(symbolName), "."))
		candidate := strings.ToLower(strings.Trim(normalizeQualifiedSymbolSeparators(qualifiedRoutineName(header)), "."))
		return candidate == normalizedTarget
	}

	return strings.EqualFold(strings.TrimSpace(header.member), strings.TrimSpace(symbolName))
}

func qualifiedRoutineName(header delphiRoutineHeader) string {
	if strings.TrimSpace(header.owner) == "" {
		return header.member
	}
	return header.owner + "." + header.member
}

func resolveDelphiRoutineBoundariesForHeader(lines []string, headerLine int, header delphiRoutineHeader, implementationLine int) (delphiRoutineBoundaries, bool) {
	endLine0, hasBody := findDelphiRoutineBlockEnd(lines, headerLine)
	if hasBody {
		beginLine0 := findDelphiRoutineOuterBeginLine(lines, headerLine, endLine0)
		if beginLine0 < 0 {
			return delphiRoutineBoundaries{}, false
		}
		return delphiRoutineBoundaries{
			headerLine: headerLine + 1,
			beginLine:  beginLine0 + 1,
			endLine:    endLine0 + 1,
		}, true
	}

	if implementationLine < 0 || headerLine >= implementationLine {
		return delphiRoutineBoundaries{}, false
	}

	targetHeader := header
	ownerRequired := strings.TrimSpace(targetHeader.owner) != ""
	if !ownerRequired {
		if inferredOwner, inferred := inferDelphiInterfaceOwner(lines, headerLine); inferred {
			targetHeader.owner = inferredOwner
			ownerRequired = true
		}
	}

	implHeader, implEnd, found := findMatchingDelphiImplementationBlock(lines, implementationLine, targetHeader, ownerRequired)
	if !found {
		return delphiRoutineBoundaries{}, false
	}

	beginLine0 := findDelphiRoutineOuterBeginLine(lines, implHeader, implEnd)
	if beginLine0 < 0 {
		return delphiRoutineBoundaries{}, false
	}

	return delphiRoutineBoundaries{
		headerLine: implHeader + 1,
		beginLine:  beginLine0 + 1,
		endLine:    implEnd + 1,
	}, true
}

func findDelphiRoutineOuterBeginLine(lines []string, headerLine int, endLine int) int {
	bodyStarted := false
	bodyDepth := 0

	for lineIndex := headerLine; lineIndex <= endLine; lineIndex++ {
		for _, token := range extractDelphiWordTokens(lines[lineIndex]) {
			if !bodyStarted {
				if isDelphiRoutineBodyStartToken(token) {
					bodyStarted = true
					bodyDepth++
					return lineIndex
				}
				if isDelphiRoutineBodyDisqualifier(token) {
					return -1
				}
				continue
			}

			switch token {
			case "begin", "case", "try", "asm":
				bodyDepth++
			case "end":
				if bodyDepth > 0 {
					bodyDepth--
				}
			}
		}
	}

	return -1
}

func resolveSymbolRoutineBoundaries(ctx context.Context, client *lsp.Client, filePath, symbolName, content string) (*delphiRoutineBoundaries, error) {
	if client != nil {
		if bounded, err := tryResolveDelphiRoutineBoundariesViaLSP(ctx, client, filePath, symbolName, content); err == nil {
			return bounded, nil
		} else if errors.Is(err, ErrDelphiSymbolAmbiguous) {
			return nil, err
		}
	}

	return resolveDelphiRoutineBoundaries(content, symbolName)
}

func tryResolveDelphiRoutineBoundariesViaLSP(ctx context.Context, client *lsp.Client, filePath, symbolName, content string) (*delphiRoutineBoundaries, error) {
	normalizedPath, err := normalizeFilePathOrURI(filePath)
	if err != nil {
		return nil, err
	}

	location, found := inferDelphiSymbolLocationFromPaths([]string{normalizedPath}, symbolName)
	if !found {
		return nil, ErrDelphiSymbolNotFound
	}

	locURI, parseErr := protocol.ParseDocumentUri(string(location.URI))
	if parseErr != nil || !pathsEqual(string(locURI), normalizedPath) {
		return nil, ErrDelphiSymbolNotFound
	}

	lines := strings.Split(content, "\n")
	startLine := int(location.Range.Start.Line)
	if startLine < 0 || startLine >= len(lines) {
		return nil, ErrDelphiSymbolNotFound
	}

	expanded, ok := tryExpandDelphiRoutineDefinition(normalizedPath, lines, location.Range.Start)
	if !ok {
		return nil, ErrDelphiSymbolNotFound
	}

	headerLine := int(expanded.Start.Line) + 1
	endLine := int(expanded.End.Line) + 1
	beginLine0 := findDelphiRoutineOuterBeginLine(lines, int(expanded.Start.Line), int(expanded.End.Line))
	if beginLine0 < 0 {
		return nil, ErrDelphiSymbolNotFound
	}

	return &delphiRoutineBoundaries{
		headerLine: headerLine,
		beginLine:  beginLine0 + 1,
		endLine:    endLine,
	}, nil
}

func pathsEqual(left, right string) bool {
	left = strings.ToLower(filepath.Clean(left))
	right = strings.ToLower(filepath.Clean(right))
	return left == right
}
