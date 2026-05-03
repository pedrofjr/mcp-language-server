package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

	allReferenceLocations := make([]protocol.Location, 0)
	attemptedLocations := make(map[string]struct{}, len(symbolLocations))
	for _, loc := range symbolLocations {
		attemptedLocations[referenceLocationKey(loc)] = struct{}{}

		referenceLocations, err := collectReferenceLocations(ctx, client, loc)
		if err != nil {
			return "", err
		}

		allReferenceLocations = appendUniqueReferenceLocations(allReferenceLocations, referenceLocations)
	}

	if len(allReferenceLocations) == 0 {
		retryLocations := collectOpenFileReferenceRetryLocations(client, symbolName, attemptedLocations)
		for _, loc := range retryLocations {
			attemptedLocations[referenceLocationKey(loc)] = struct{}{}

			referenceLocations, err := collectReferenceLocations(ctx, client, loc)
			if err != nil {
				return "", err
			}
			if len(referenceLocations) == 0 {
				continue
			}

			allReferenceLocations = appendUniqueReferenceLocations(allReferenceLocations, referenceLocations)
			break
		}
	}

	if len(allReferenceLocations) == 0 {
		allReferenceLocations = appendUniqueReferenceLocations(
			allReferenceLocations,
			collectLastResortOpenFileReferenceLocations(client, symbolName, symbolLocations),
		)
	}

	if len(allReferenceLocations) == 0 || referenceLocationsNeedWorkspaceCompletion(allReferenceLocations, symbolLocations) {
		allReferenceLocations = appendUniqueReferenceLocations(
			allReferenceLocations,
			collectLastResortWorkspaceReferenceLocations(client, symbolName, symbolLocations),
		)
	}

	if len(allReferenceLocations) == 0 {
		return fmt.Sprintf("No references found for symbol: %s", symbolName), nil
	}

	allReferenceLocations = postProcessDelphiReferenceLocations(allReferenceLocations)
	if len(allReferenceLocations) == 0 {
		return fmt.Sprintf("No references found for symbol: %s", symbolName), nil
	}

	allReferences, err := formatReferenceBlocks(ctx, client, allReferenceLocations, contextLines)
	if err != nil {
		return "", err
	}

	return strings.Join(allReferences, "\n"), nil
}

func resolveReferenceSymbolLocations(ctx context.Context, client *lsp.Client, symbolName string) ([]protocol.Location, error) {
	if !client.SupportsWorkspaceSymbol() {
		inferredLocation, found := inferSymbolLocationFromOpenFiles(client, symbolName)
		if !found {
			inferredLocation, found = inferDelphiSymbolLocationFromWorkspace(client, symbolName)
		}
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
			inferredLocation, found = inferDelphiSymbolLocationFromWorkspace(client, symbolName)
		}
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
			inferredLocation, found = inferDelphiSymbolLocationFromWorkspace(client, symbolName)
		}
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

	if len(locations) == 0 {
		inferredLocation, found := inferDelphiSymbolLocation(client, symbolName)
		if !found {
			return nil, nil
		}

		return []protocol.Location{inferredLocation}, nil
	}

	return locations, nil
}

func collectReferenceLocations(ctx context.Context, client *lsp.Client, loc protocol.Location) ([]protocol.Location, error) {
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

	return refs, nil
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

func collectLastResortWorkspaceReferenceBlocks(ctx context.Context, client *lsp.Client, symbolName string, symbolLocations []protocol.Location, contextLines int) ([]string, error) {
	textualLocations := collectLastResortWorkspaceReferenceLocations(client, symbolName, symbolLocations)
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

func collectLastResortWorkspaceReferenceLocations(client *lsp.Client, symbolName string, symbolLocations []protocol.Location) []protocol.Location {
	searchPatterns := buildSymbolSearchPatterns(symbolName)
	if len(searchPatterns) == 0 {
		return nil
	}

	declarationLines := make(map[string]struct{}, len(symbolLocations))
	for _, loc := range symbolLocations {
		declarationLines[referenceLineKey(loc)] = struct{}{}
	}

	workspaceRoots := collectReferenceWorkspaceRoots(client, symbolLocations)
	if len(workspaceRoots) == 0 {
		return nil
	}
	var seedLocations []protocol.Location
	seenLocations := make(map[string]struct{}, len(symbolLocations))
	for _, loc := range symbolLocations {
		if !isDelphiWorkspaceReferenceFile(loc.URI.Path()) {
			continue
		}

		locationKey := referenceLocationKey(loc)
		if _, seen := seenLocations[locationKey]; seen {
			continue
		}

		seenLocations[locationKey] = struct{}{}
		seedLocations = append(seedLocations, loc)
	}
	var locations []protocol.Location
	for _, root := range workspaceRoots {
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

			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			uri := protocol.DocumentUri(protocol.URIFromPath(path))
			lines := strings.Split(string(content), "\n")
			fileLocations := findOpenFileTextualReferenceLocations(uri, lines, searchPatterns, declarationLines)
			for _, loc := range fileLocations {
				locationKey := referenceLocationKey(loc)
				if _, seen := seenLocations[locationKey]; seen {
					continue
				}

				seenLocations[locationKey] = struct{}{}
				locations = append(locations, loc)
			}

			return nil
		})
	}

	// Inclui declações apenas se o walk encontrou referências reais
	if len(locations) > 0 {
		locations = append(seedLocations, locations...)
	}

	return locations
}

func collectReferenceWorkspaceRoots(client *lsp.Client, symbolLocations []protocol.Location) []string {
	uniqueRoots := make(map[string]struct{})

	addRoot := func(path string) {
		if path == "" {
			return
		}

		uniqueRoots[filepath.Clean(path)] = struct{}{}
	}

	addRoot(client.GetWorkspaceRoot())

	for _, uriStr := range client.GetOpenFilesSnapshot() {
		path, ok := uriPathFromString(uriStr)
		if !ok {
			continue
		}

		addRoot(filepath.Dir(path))
	}

	for _, loc := range symbolLocations {
		addRoot(filepath.Dir(loc.URI.Path()))
	}

	roots := make([]string, 0, len(uniqueRoots))
	for root := range uniqueRoots {
		roots = append(roots, root)
	}

	sort.Strings(roots)
	return roots
}

func isDelphiWorkspaceReferenceFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pas", ".pp", ".dpr", ".dpk", ".lpr", ".inc":
		return true
	default:
		return false
	}
}

func shouldSkipReferenceWorkspaceDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", ".hg", ".svn", "node_modules", "vendor", "target", "build", "dist":
		return true
	default:
		return false
	}
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
		searchLine := sanitizePascalSearchLine(line)
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
		searchLine := sanitizePascalSearchLine(line)
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

func appendUniqueReferenceLocations(existing []protocol.Location, candidates []protocol.Location) []protocol.Location {
	if len(candidates) == 0 {
		return existing
	}

	seenLocations := make(map[string]struct{}, len(existing)+len(candidates))
	for _, loc := range existing {
		seenLocations[referenceLocationKey(loc)] = struct{}{}
	}

	for _, loc := range candidates {
		locationKey := referenceLocationKey(loc)
		if _, seen := seenLocations[locationKey]; seen {
			continue
		}

		seenLocations[locationKey] = struct{}{}
		existing = append(existing, loc)
	}

	return existing
}

func referenceLocationsNeedWorkspaceCompletion(referenceLocations []protocol.Location, symbolLocations []protocol.Location) bool {
	if len(referenceLocations) == 0 {
		return true
	}

	providerFiles := make(map[protocol.DocumentUri]struct{}, len(symbolLocations))
	for _, loc := range symbolLocations {
		providerFiles[loc.URI] = struct{}{}
	}

	for _, loc := range referenceLocations {
		if _, providerOnly := providerFiles[loc.URI]; !providerOnly {
			return false
		}
	}

	return true
}

type delphiReferenceBucket int

const (
	delphiReferenceBucketExecutable delphiReferenceBucket = iota
	delphiReferenceBucketDeclaration
	delphiReferenceBucketOther
)

type delphiReferenceCandidate struct {
	location protocol.Location
	bucket   delphiReferenceBucket
	path     string
}

type delphiReferenceScanState struct {
	inBraceComment   bool
	inBraceDirective bool
	inParenComment   bool
	inParenDirective bool
}

type delphiReferenceLineClassification struct {
	pureComment bool
	bucket      delphiReferenceBucket
}

func postProcessDelphiReferenceLocations(referenceLocations []protocol.Location) []protocol.Location {
	if len(referenceLocations) == 0 {
		return nil
	}

	delphiRefsByURI := make(map[protocol.DocumentUri][]protocol.Location)
	nonDelphiRefs := make([]protocol.Location, 0, len(referenceLocations))

	for _, loc := range referenceLocations {
		if !isDelphiWorkspaceReferenceFile(loc.URI.Path()) {
			nonDelphiRefs = append(nonDelphiRefs, loc)
			continue
		}

		delphiRefsByURI[loc.URI] = append(delphiRefsByURI[loc.URI], loc)
	}

	if len(delphiRefsByURI) == 0 {
		return referenceLocations
	}

	processedRefs := make([]protocol.Location, 0, len(referenceLocations))
	processedRefs = append(processedRefs, nonDelphiRefs...)

	uriTexts := make([]string, 0, len(delphiRefsByURI))
	for uri := range delphiRefsByURI {
		uriTexts = append(uriTexts, string(uri))
	}
	sort.Strings(uriTexts)

	for _, uriText := range uriTexts {
		uri := protocol.DocumentUri(uriText)
		fileRefs := delphiRefsByURI[uri]
		path := uri.Path()

		toolsLogger.Debug("Delphi references raw for %s: %s", path, formatReferenceLocationSummary(fileRefs))

		filteredRefs, err := filterDelphiReferenceLocations(path, fileRefs)
		if err != nil {
			toolsLogger.Debug("failed to post-process Delphi references for %s: %v", path, err)
			processedRefs = append(processedRefs, fileRefs...)
			continue
		}

		toolsLogger.Debug("Delphi references filtered for %s: %s", path, formatReferenceLocationSummary(filteredRefs))
		processedRefs = append(processedRefs, filteredRefs...)
	}

	return processedRefs
}

func filterDelphiReferenceLocations(path string, fileRefs []protocol.Location) ([]protocol.Location, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lineClasses := classifyDelphiReferenceLines(strings.Split(string(content), "\n"))
	bestByLine := make(map[int]delphiReferenceCandidate, len(fileRefs))
	extraCandidates := make([]delphiReferenceCandidate, 0)

	for _, loc := range fileRefs {
		lineIndex := int(loc.Range.Start.Line)
		if lineIndex < 0 || lineIndex >= len(lineClasses) {
			extraCandidates = append(extraCandidates, delphiReferenceCandidate{location: loc, bucket: delphiReferenceBucketOther, path: path})
			continue
		}

		lineClass := lineClasses[lineIndex]
		if lineClass.pureComment {
			continue
		}

		candidate := delphiReferenceCandidate{location: loc, bucket: lineClass.bucket, path: path}
		current, exists := bestByLine[lineIndex]
		if !exists || isBetterDelphiReferenceCandidate(candidate, current) {
			bestByLine[lineIndex] = candidate
		}
	}

	rankedCandidates := make([]delphiReferenceCandidate, 0, len(bestByLine)+len(extraCandidates))
	for _, candidate := range bestByLine {
		rankedCandidates = append(rankedCandidates, candidate)
	}
	rankedCandidates = append(rankedCandidates, extraCandidates...)

	sort.Slice(rankedCandidates, func(i int, j int) bool {
		return isBetterDelphiReferenceCandidate(rankedCandidates[i], rankedCandidates[j])
	})
	rankedCandidates = collapseDelphiDeclarationNoise(rankedCandidates)

	filteredRefs := make([]protocol.Location, 0, len(rankedCandidates))
	for _, candidate := range rankedCandidates {
		filteredRefs = append(filteredRefs, candidate.location)
	}

	return filteredRefs, nil
}

func classifyDelphiReferenceLines(lines []string) []delphiReferenceLineClassification {
	lineClasses := make([]delphiReferenceLineClassification, 0, len(lines))
	state := delphiReferenceScanState{}

	for _, line := range lines {
		lineClasses = append(lineClasses, classifyDelphiReferenceLine(line, &state))
	}

	return lineClasses
}

func classifyDelphiReferenceLine(line string, state *delphiReferenceScanState) delphiReferenceLineClassification {
	visibleLine := stripDelphiCommentsPreservingDirectives(line, state)
	trimmedVisibleLine := strings.TrimSpace(visibleLine)
	if trimmedVisibleLine == "" {
		return delphiReferenceLineClassification{pureComment: strings.TrimSpace(line) != ""}
	}

	if looksLikeDelphiDeclarationReferenceLine(trimmedVisibleLine) {
		return delphiReferenceLineClassification{bucket: delphiReferenceBucketDeclaration}
	}

	if looksLikeDelphiExecutableReferenceLine(trimmedVisibleLine) {
		return delphiReferenceLineClassification{bucket: delphiReferenceBucketExecutable}
	}

	return delphiReferenceLineClassification{bucket: delphiReferenceBucketOther}
}

func stripDelphiCommentsPreservingDirectives(line string, state *delphiReferenceScanState) string {
	lineBytes := []byte(line)
	visibleLine := []byte(line)
	inStringLiteral := false

	for i := 0; i < len(lineBytes); i++ {
		current := lineBytes[i]
		next := byte(0)
		if i+1 < len(lineBytes) {
			next = lineBytes[i+1]
		}

		if state.inBraceComment {
			visibleLine[i] = ' '
			if current == '}' {
				state.inBraceComment = false
			}
			continue
		}

		if state.inBraceDirective {
			if current == '}' {
				state.inBraceDirective = false
			}
			continue
		}

		if state.inParenComment {
			visibleLine[i] = ' '
			if current == '*' && next == ')' {
				visibleLine[i+1] = ' '
				i++
				state.inParenComment = false
			}
			continue
		}

		if state.inParenDirective {
			if current == '*' && next == ')' {
				i++
				state.inParenDirective = false
			}
			continue
		}

		if inStringLiteral {
			if current == '\'' {
				if next == '\'' {
					i++
					continue
				}
				inStringLiteral = false
			}
			continue
		}

		if current == '\'' {
			inStringLiteral = true
			continue
		}

		if current == '/' && next == '/' {
			for j := i; j < len(visibleLine); j++ {
				visibleLine[j] = ' '
			}
			break
		}

		if current == '{' {
			if next == '$' {
				state.inBraceDirective = true
				continue
			}

			visibleLine[i] = ' '
			state.inBraceComment = true
			continue
		}

		if current == '(' && next == '*' {
			if i+2 < len(lineBytes) && lineBytes[i+2] == '$' {
				state.inParenDirective = true
				i++
				continue
			}

			visibleLine[i] = ' '
			visibleLine[i+1] = ' '
			i++
			state.inParenComment = true
			continue
		}
	}

	return string(visibleLine)
}

func looksLikeDelphiDeclarationReferenceLine(line string) bool {
	lowerLine := strings.ToLower(line)
	if scorePotentialDeclarationLine(lowerLine) > 0 {
		return true
	}

	return strings.Contains(lowerLine, "property ") || strings.Contains(lowerLine, "operator ")
}

func looksLikeDelphiExecutableReferenceLine(line string) bool {
	lowerLine := strings.ToLower(strings.TrimSpace(line))
	if lowerLine == "" {
		return false
	}

	if strings.HasPrefix(lowerLine, "{$") || strings.HasPrefix(lowerLine, "(*$") {
		return false
	}

	switch lowerLine {
	case "interface", "implementation", "type", "var", "const", "resourcestring", "label", "threadvar", "begin", "end", "end;", "private", "protected", "public", "published", "automated", "strict private", "strict protected", "strict public", "strict published":
		return false
	}

	if strings.HasPrefix(lowerLine, "unit ") || strings.HasPrefix(lowerLine, "uses ") || strings.HasPrefix(lowerLine, "exports ") {
		return false
	}

	if strings.Contains(lowerLine, ":") && !strings.Contains(lowerLine, ":=") && !strings.Contains(lowerLine, "(") && !strings.Contains(lowerLine, "[") {
		return false
	}

	return true
}

func isBetterDelphiReferenceCandidate(left delphiReferenceCandidate, right delphiReferenceCandidate) bool {
	if left.bucket != right.bucket {
		return left.bucket < right.bucket
	}

	if left.path != right.path {
		return left.path < right.path
	}

	if left.location.Range.Start.Line != right.location.Range.Start.Line {
		return left.location.Range.Start.Line < right.location.Range.Start.Line
	}

	if left.location.Range.Start.Character != right.location.Range.Start.Character {
		return left.location.Range.Start.Character < right.location.Range.Start.Character
	}

	if left.location.Range.End.Line != right.location.Range.End.Line {
		return left.location.Range.End.Line < right.location.Range.End.Line
	}

	return left.location.Range.End.Character < right.location.Range.End.Character
}

func collapseDelphiDeclarationNoise(candidates []delphiReferenceCandidate) []delphiReferenceCandidate {
	hasExecutable := false
	for _, candidate := range candidates {
		if candidate.bucket == delphiReferenceBucketExecutable {
			hasExecutable = true
			break
		}
	}

	if !hasExecutable {
		return candidates
	}

	filtered := make([]delphiReferenceCandidate, 0, len(candidates))
	keptDeclaration := false
	for _, candidate := range candidates {
		if candidate.bucket == delphiReferenceBucketDeclaration {
			if keptDeclaration {
				continue
			}

			keptDeclaration = true
		}

		filtered = append(filtered, candidate)
	}

	return filtered
}

func formatReferenceLocationSummary(locations []protocol.Location) string {
	if len(locations) == 0 {
		return "[]"
	}

	parts := make([]string, 0, len(locations))
	for _, loc := range locations {
		parts = append(parts, fmt.Sprintf("L%d:C%d", loc.Range.Start.Line+1, loc.Range.Start.Character+1))
	}

	return "[" + strings.Join(parts, ", ") + "]"
}
