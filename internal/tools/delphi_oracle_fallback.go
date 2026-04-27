package tools

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

type symbolLocationCandidate struct {
	location protocol.Location
	score    int
	line     int
	path     string
}

func isMethodNotSupportedError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	if strings.Contains(message, "(code: -32601)") {
		return true
	}

	return strings.Contains(message, "method not found") || strings.Contains(message, "not supported")
}

func inferSymbolLocationFromOpenFiles(client *lsp.Client, symbolName string) (protocol.Location, bool) {
	searchPatterns := buildInferredSymbolSearchPatterns(symbolName)
	if len(searchPatterns) == 0 {
		return protocol.Location{}, false
	}

	openFiles := client.GetOpenFilesSnapshot()
	sort.Strings(openFiles)

	var bestCandidate symbolLocationCandidate
	hasCandidate := false

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
		for lineIndex, line := range lines {
			searchLine := sanitizePascalSearchLine(line)
			for _, pattern := range searchPatterns {
				matchRange := pattern.FindStringIndex(searchLine)
				if matchRange == nil {
					continue
				}
				if lineIndex < 0 || matchRange[0] < 0 || matchRange[1] < matchRange[0] {
					continue
				}

				candidateScore := scorePotentialDeclarationLine(searchLine)
				matchedText := searchLine[matchRange[0]:matchRange[1]]
				if strings.EqualFold(normalizeQualifiedSymbolSeparators(matchedText), normalizeQualifiedSymbolSeparators(symbolName)) {
					candidateScore += 2
				}

				candidate := symbolLocationCandidate{
					location: protocol.Location{
						URI: protocol.DocumentUri(uriStr),
						Range: protocol.Range{
							Start: protocol.Position{Line: uint32(lineIndex), Character: uint32(matchRange[0])},
							End:   protocol.Position{Line: uint32(lineIndex), Character: uint32(matchRange[1])},
						},
					},
					score: candidateScore,
					line:  lineIndex,
					path:  path,
				}

				if !hasCandidate || isBetterSymbolLocationCandidate(candidate, bestCandidate) {
					bestCandidate = candidate
					hasCandidate = true
				}

				break
			}
		}
	}

	if !hasCandidate {
		return protocol.Location{}, false
	}

	return bestCandidate.location, true
}

func buildInferredSymbolSearchPatterns(symbolName string) []*regexp.Regexp {
	if !isQualifiedSymbolQuery(symbolName) {
		return buildSymbolSearchPatterns(symbolName)
	}

	owner, member, ok := splitQualifiedSymbolQuery(symbolName)
	if !ok {
		return nil
	}

	pattern, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(owner) + `(?:\.|::)` + regexp.QuoteMeta(member) + `\b`)
	if err != nil {
		return nil
	}

	return []*regexp.Regexp{pattern}
}

func scorePotentialDeclarationLine(line string) int {
	lowerLine := strings.ToLower(line)
	if strings.Contains(lowerLine, "constructor ") || strings.Contains(lowerLine, "destructor ") {
		return 3
	}

	if strings.Contains(lowerLine, "procedure ") || strings.Contains(lowerLine, "function ") || strings.Contains(lowerLine, "method ") {
		return 1
	}

	return 0
}

func isBetterSymbolLocationCandidate(candidate symbolLocationCandidate, current symbolLocationCandidate) bool {
	if candidate.score != current.score {
		return candidate.score > current.score
	}

	if candidate.line != current.line {
		return candidate.line < current.line
	}

	return candidate.path < current.path
}

func definitionResultToLocations(result protocol.Or_Result_textDocument_definition) []protocol.Location {
	if result.Value == nil {
		return nil
	}

	switch value := result.Value.(type) {
	case protocol.Or_Definition:
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

func buildSymbolSearchPatterns(symbolName string) []*regexp.Regexp {
	if owner, member, ok := splitQualifiedSymbolQuery(symbolName); ok {
		pattern, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(owner) + `(?:\.|::)` + regexp.QuoteMeta(member) + `\b`)
		if err != nil {
			return nil
		}

		return []*regexp.Regexp{pattern}
	}

	terms := []string{symbolName}
	if strings.Contains(symbolName, ".") {
		parts := strings.Split(symbolName, ".")
		terms = append(terms, parts[len(parts)-1])
	}
	if strings.Contains(symbolName, "::") {
		parts := strings.Split(symbolName, "::")
		terms = append(terms, parts[len(parts)-1])
	}

	uniqueTerms := make(map[string]struct{})
	patterns := make([]*regexp.Regexp, 0, len(terms))
	for _, term := range terms {
		cleanTerm := strings.TrimSpace(term)
		if cleanTerm == "" {
			continue
		}
		if _, exists := uniqueTerms[cleanTerm]; exists {
			continue
		}
		uniqueTerms[cleanTerm] = struct{}{}

		pattern, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(cleanTerm) + `\b`)
		if err != nil {
			continue
		}
		patterns = append(patterns, pattern)
	}

	return patterns
}

func splitQualifiedSymbolQuery(symbolName string) (string, string, bool) {
	separatorIndex := strings.LastIndex(symbolName, ".")
	separatorLength := 1

	if doubleColonIndex := strings.LastIndex(symbolName, "::"); doubleColonIndex > separatorIndex {
		separatorIndex = doubleColonIndex
		separatorLength = 2
	}

	if separatorIndex <= 0 || separatorIndex+separatorLength >= len(symbolName) {
		return "", "", false
	}

	owner := strings.TrimSpace(symbolName[:separatorIndex])
	member := strings.TrimSpace(symbolName[separatorIndex+separatorLength:])
	if owner == "" || member == "" {
		return "", "", false
	}

	return owner, member, true
}

func normalizeQualifiedSymbolSeparators(symbolName string) string {
	return strings.ReplaceAll(symbolName, "::", ".")
}

func sanitizePascalSearchLine(line string) string {
	masked := []byte(line)
	inStringLiteral := false

	for i := 0; i < len(masked); i++ {
		if !inStringLiteral && masked[i] == '/' && i+1 < len(masked) && masked[i+1] == '/' {
			for j := i; j < len(masked); j++ {
				masked[j] = ' '
			}
			break
		}

		if masked[i] != '\'' {
			if inStringLiteral {
				masked[i] = ' '
			}
			continue
		}

		masked[i] = ' '
		if inStringLiteral && i+1 < len(masked) && masked[i+1] == '\'' {
			masked[i+1] = ' '
			i++
			continue
		}

		inStringLiteral = !inStringLiteral
	}

	return string(masked)
}

func trimSingleLineComment(line string) string {
	commentIndex := strings.Index(line, "//")
	if commentIndex < 0 {
		return line
	}
	return line[:commentIndex]
}

func uriPathFromString(uriText string) (path string, ok bool) {
	parsed, err := protocol.ParseDocumentUri(uriText)
	if err == nil {
		return parsed.Path(), true
	}

	rawPath := strings.TrimPrefix(uriText, "file://")
	if rawPath == uriText {
		return "", false
	}

	if unescaped, unescapeErr := url.PathUnescape(rawPath); unescapeErr == nil {
		rawPath = unescaped
	}

	rawPath = strings.TrimPrefix(rawPath, "/")
	rawPath = filepath.FromSlash(rawPath)

	if rawPath == "" {
		return "", false
	}

	return rawPath, true
}
