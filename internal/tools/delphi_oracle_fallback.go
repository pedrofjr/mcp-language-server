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

type delphiSearchPattern struct {
	regex           *regexp.Regexp
	scoreBonus      int
	declarationOnly bool
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

func inferDelphiSymbolLocation(client *lsp.Client, symbolName string) (protocol.Location, bool) {
	location, found := inferDelphiSymbolLocationFromOpenFiles(client, symbolName)
	if found {
		return location, true
	}

	return inferDelphiSymbolLocationFromWorkspace(client, symbolName)
}

func inferDelphiSymbolLocationFromOpenFiles(client *lsp.Client, symbolName string) (protocol.Location, bool) {
	return inferDelphiSymbolLocationFromPaths(collectOpenDelphiSearchPaths(client), symbolName)
}

func inferDelphiSymbolLocationFromWorkspace(client *lsp.Client, symbolName string) (protocol.Location, bool) {
	return inferDelphiSymbolLocationFromPaths(collectWorkspaceDelphiSearchPaths(client), symbolName)
}

func inferDelphiSymbolLocationFromPaths(paths []string, symbolName string) (protocol.Location, bool) {
	exactPatterns := buildInferredSymbolSearchPatterns(symbolName)
	providerPatterns, preferredProviders := buildDelphiProviderSearchPatterns(symbolName)
	if len(exactPatterns) == 0 && len(providerPatterns) == 0 {
		return protocol.Location{}, false
	}

	sort.Strings(paths)

	var bestCandidate symbolLocationCandidate
	hasCandidate := false

	for _, path := range paths {
		if !isDelphiWorkspaceReferenceFile(path) {
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		uri := protocol.DocumentUri(protocol.URIFromPath(path))
		baseName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		_, preferredProvider := preferredProviders[strings.ToLower(baseName)]

		lines := strings.Split(string(content), "\n")
		for lineIndex, line := range lines {
			searchLine := sanitizePascalSearchLine(line)
			candidate, found := buildBestDelphiSymbolLocationCandidate(
				uri,
				path,
				lineIndex,
				searchLine,
				symbolName,
				exactPatterns,
				providerPatterns,
				preferredProvider,
			)
			if !found {
				continue
			}

			if !hasCandidate || isBetterSymbolLocationCandidate(candidate, bestCandidate) {
				bestCandidate = candidate
				hasCandidate = true
			}
		}
	}

	if !hasCandidate {
		return protocol.Location{}, false
	}

	return bestCandidate.location, true
}

func locationMatchesDelphiSymbol(location protocol.Location, symbolName string) bool {
	path := location.URI.Path()
	if !isDelphiWorkspaceReferenceFile(path) {
		return false
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	lineIndex := int(location.Range.Start.Line)
	lines := strings.Split(string(content), "\n")
	if lineIndex < 0 || lineIndex >= len(lines) {
		return false
	}

	exactPatterns := buildInferredSymbolSearchPatterns(symbolName)
	providerPatterns, preferredProviders := buildDelphiProviderSearchPatterns(symbolName)
	baseName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	_, preferredProvider := preferredProviders[strings.ToLower(baseName)]

	candidate, found := buildBestDelphiSymbolLocationCandidate(
		location.URI,
		path,
		lineIndex,
		sanitizePascalSearchLine(lines[lineIndex]),
		symbolName,
		exactPatterns,
		providerPatterns,
		preferredProvider,
	)
	if !found {
		return false
	}

	if candidate.location.Range.Start.Line != location.Range.Start.Line {
		return false
	}

	return candidate.location.Range.Start.Character <= location.Range.Start.Character && candidate.location.Range.End.Character >= location.Range.Start.Character
}

func collectOpenDelphiSearchPaths(client *lsp.Client) []string {
	openFiles := client.GetOpenFilesSnapshot()
	paths := make([]string, 0, len(openFiles))
	seenPaths := make(map[string]struct{}, len(openFiles))

	for _, uriStr := range openFiles {
		path, ok := uriPathFromString(uriStr)
		if !ok || !isDelphiWorkspaceReferenceFile(path) {
			continue
		}

		cleanPath := filepath.Clean(path)
		if _, seen := seenPaths[cleanPath]; seen {
			continue
		}

		seenPaths[cleanPath] = struct{}{}
		paths = append(paths, cleanPath)
	}

	return paths
}

func collectWorkspaceDelphiSearchPaths(client *lsp.Client) []string {
	roots := collectReferenceWorkspaceRoots(client, nil)
	paths := make([]string, 0)
	seenPaths := make(map[string]struct{})

	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}

			if entry.IsDir() {
				if shouldSkipReferenceWorkspaceDir(entry.Name()) {
					return filepath.SkipDir
				}
				return nil
			}

			if !isDelphiWorkspaceReferenceFile(path) {
				return nil
			}

			cleanPath := filepath.Clean(path)
			if _, seen := seenPaths[cleanPath]; seen {
				return nil
			}

			seenPaths[cleanPath] = struct{}{}
			paths = append(paths, cleanPath)
			return nil
		})
	}

	return paths
}

func buildBestDelphiSymbolLocationCandidate(
	uri protocol.DocumentUri,
	path string,
	lineIndex int,
	searchLine string,
	symbolName string,
	exactPatterns []*regexp.Regexp,
	providerPatterns []delphiSearchPattern,
	preferredProvider bool,
) (symbolLocationCandidate, bool) {
	declarationScore := scorePotentialDeclarationLine(searchLine)
	var bestCandidate symbolLocationCandidate
	hasCandidate := false

	for _, pattern := range exactPatterns {
		matchRange := pattern.FindStringIndex(searchLine)
		if matchRange == nil {
			continue
		}

		candidateScore := declarationScore
		matchedText := searchLine[matchRange[0]:matchRange[1]]
		if strings.EqualFold(normalizeQualifiedSymbolSeparators(matchedText), normalizeQualifiedSymbolSeparators(symbolName)) {
			candidateScore += 2
		}

		candidate := newSymbolLocationCandidate(uri, path, lineIndex, matchRange, candidateScore)
		if !hasCandidate || isBetterSymbolLocationCandidate(candidate, bestCandidate) {
			bestCandidate = candidate
			hasCandidate = true
		}

		break
	}

	if !preferredProvider {
		return bestCandidate, hasCandidate
	}

	for _, pattern := range providerPatterns {
		if pattern.declarationOnly && declarationScore == 0 {
			continue
		}

		matchRange := pattern.regex.FindStringIndex(searchLine)
		if matchRange == nil {
			continue
		}

		candidate := newSymbolLocationCandidate(uri, path, lineIndex, matchRange, declarationScore+pattern.scoreBonus)
		if !hasCandidate || isBetterSymbolLocationCandidate(candidate, bestCandidate) {
			bestCandidate = candidate
			hasCandidate = true
		}
	}

	return bestCandidate, hasCandidate
}

func newSymbolLocationCandidate(uri protocol.DocumentUri, path string, lineIndex int, matchRange []int, score int) symbolLocationCandidate {
	return symbolLocationCandidate{
		location: protocol.Location{
			URI: uri,
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(lineIndex), Character: uint32(matchRange[0])},
				End:   protocol.Position{Line: uint32(lineIndex), Character: uint32(matchRange[1])},
			},
		},
		score: score,
		line:  lineIndex,
		path:  path,
	}
}

func buildDelphiProviderSearchPatterns(symbolName string) ([]delphiSearchPattern, map[string]struct{}) {
	if !isQualifiedSymbolQuery(symbolName) {
		return nil, nil
	}

	normalizedSymbol := normalizeQualifiedSymbolSeparators(symbolName)
	parts := strings.Split(normalizedSymbol, ".")
	if len(parts) < 2 {
		return nil, nil
	}

	providerBase := strings.TrimSpace(parts[0])
	member := strings.TrimSpace(parts[len(parts)-1])
	if providerBase == "" || member == "" {
		return nil, nil
	}

	preferredProviders := map[string]struct{}{
		strings.ToLower(providerBase): {},
	}

	patterns := make([]delphiSearchPattern, 0, 3)
	ownerSuffix := strings.Join(parts[1:len(parts)-1], ".")
	if ownerSuffix != "" {
		pattern := compileSearchPattern(`(?i)\b` + regexp.QuoteMeta(ownerSuffix) + `(?:\.|::)` + regexp.QuoteMeta(member) + `\b`)
		if pattern != nil {
			patterns = append(patterns, delphiSearchPattern{regex: pattern, scoreBonus: 4, declarationOnly: true})
		}
		return patterns, preferredProviders
	}

	if looksLikeDelphiTypeName(providerBase) {
		return patterns, preferredProviders
	}

	routinePattern := compileSearchPattern(`(?i)\b(?:class\s+)?(?:function|procedure|constructor|destructor|property|operator)\s+` + regexp.QuoteMeta(member) + `\b`)
	if routinePattern != nil {
		patterns = append(patterns, delphiSearchPattern{regex: routinePattern, scoreBonus: 4, declarationOnly: true})
	}

	typePattern := compileSearchPattern(`(?i)\b` + regexp.QuoteMeta(member) + `\s*=\s*(?:class|record|interface|object)\b`)
	if typePattern != nil {
		patterns = append(patterns, delphiSearchPattern{regex: typePattern, scoreBonus: 3, declarationOnly: true})
	}

	return patterns, preferredProviders
}

func compileSearchPattern(expression string) *regexp.Regexp {
	pattern, err := regexp.Compile(expression)
	if err != nil {
		return nil
	}

	return pattern
}

func looksLikeDelphiTypeName(name string) bool {
	if len(name) < 2 {
		return false
	}

	upperName := strings.ToUpper(name[:2])
	if upperName[0] != 'T' && upperName[0] != 'I' && upperName[0] != 'E' {
		return false
	}

	return upperName[1] >= 'A' && upperName[1] <= 'Z'
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

	if strings.Contains(lowerLine, "= class") || strings.Contains(lowerLine, "= record") || strings.Contains(lowerLine, "= interface") || strings.Contains(lowerLine, "= object") {
		return 2
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
	inBraceComment := false
	inParenComment := false

	for i := 0; i < len(masked); i++ {
		current := masked[i]
		next := byte(0)
		if i+1 < len(masked) {
			next = masked[i+1]
		}

		if inBraceComment {
			masked[i] = ' '
			if current == '}' {
				inBraceComment = false
			}
			continue
		}

		if inParenComment {
			masked[i] = ' '
			if current == '*' && next == ')' {
				masked[i+1] = ' '
				i++
				inParenComment = false
			}
			continue
		}

		if !inStringLiteral && current == '/' && next == '/' {
			for j := i; j < len(masked); j++ {
				masked[j] = ' '
			}
			break
		}

		if !inStringLiteral && current == '{' {
			masked[i] = ' '
			inBraceComment = true
			continue
		}

		if !inStringLiteral && current == '(' && next == '*' {
			masked[i] = ' '
			masked[i+1] = ' '
			i++
			inParenComment = true
			continue
		}

		if current != '\'' {
			if inStringLiteral {
				masked[i] = ' '
			}
			continue
		}

		masked[i] = ' '
		if inStringLiteral && next == '\'' {
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
