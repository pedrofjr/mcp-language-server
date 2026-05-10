package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

type symbolsOverviewResponse struct {
	QueryUsed    string                     `json:"queryUsed"`
	TotalUnits   int                        `json:"totalUnits"`
	TotalSymbols int                        `json:"totalSymbols"`
	Units        []symbolsOverviewUnitEntry `json:"units"`
}

type symbolsOverviewUnitEntry struct {
	URI          string                `json:"uri"`
	UnitName     string                `json:"unitName"`
	TotalSymbols int                   `json:"totalSymbols"`
	Symbols      []symbolsOverviewItem `json:"symbols"`
}

type symbolsOverviewItem struct {
	Name          string                   `json:"name"`
	Kind          any                      `json:"kind"`
	ContainerName string                   `json:"containerName,omitempty"`
	Trace         symbolsOverviewItemTrace `json:"trace"`
}

type symbolsOverviewItemTrace struct {
	PreferredSymbolName  string                          `json:"preferredSymbolName"`
	SymbolNameCandidates []string                        `json:"symbolNameCandidates"`
	Definition           symbolsOverviewTraceToolRequest `json:"definition"`
	References           symbolsOverviewTraceToolRequest `json:"references"`
}

type symbolsOverviewTraceToolRequest struct {
	SymbolName string `json:"symbolName"`
}

type symbolsOverviewRawSymbol struct {
	Name          string                     `json:"name"`
	Kind          any                        `json:"kind"`
	ContainerName string                     `json:"containerName,omitempty"`
	Location      symbolsOverviewRawLocation `json:"location"`
}

type symbolsOverviewRawLocation struct {
	URI string `json:"uri"`
}

// GetSymbolsOverview aggregates workspace symbols by URI and returns a compact JSON overview.
func GetSymbolsOverview(ctx context.Context, client *lsp.Client, query string) (string, error) {
	trimmedQuery := strings.TrimSpace(query)

	results, err := fetchSymbolsOverviewRaw(ctx, client, trimmedQuery)
	if err != nil {
		return "", err
	}

	payload := symbolsOverviewResponse{
		QueryUsed: trimmedQuery,
		Units:     make([]symbolsOverviewUnitEntry, 0),
	}

	if len(results) == 0 {
		return marshalSymbolsOverview(payload)
	}

	unitsByURI := make(map[string]*symbolsOverviewUnitEntry)
	for _, sym := range results {
		symbolName := strings.TrimSpace(sym.Name)
		if symbolName == "" {
			continue
		}

		uri := strings.TrimSpace(sym.Location.URI)
		if uri == "" {
			uri = "unknown:///"
		}

		unit := unitsByURI[uri]
		if unit == nil {
			unit = &symbolsOverviewUnitEntry{
				URI:      uri,
				UnitName: deriveUnitNameFromURI(uri),
				Symbols:  make([]symbolsOverviewItem, 0),
			}
			unitsByURI[uri] = unit
		}

		unit.Symbols = append(unit.Symbols, symbolsOverviewItem{
			Name:          symbolName,
			Kind:          normalizeOverviewKind(sym.Kind),
			ContainerName: strings.TrimSpace(sym.ContainerName),
			Trace:         buildSymbolsOverviewTrace(symbolName, unit.UnitName, sym.ContainerName),
		})
	}

	payload.Units = make([]symbolsOverviewUnitEntry, 0, len(unitsByURI))
	for _, unit := range unitsByURI {
		sort.Slice(unit.Symbols, func(i, j int) bool {
			return compareOverviewItems(unit.Symbols[i], unit.Symbols[j]) < 0
		})

		unit.TotalSymbols = len(unit.Symbols)
		payload.TotalSymbols += unit.TotalSymbols
		payload.Units = append(payload.Units, *unit)
	}

	sort.Slice(payload.Units, func(i, j int) bool {
		leftUnit := strings.ToLower(payload.Units[i].UnitName)
		rightUnit := strings.ToLower(payload.Units[j].UnitName)
		if leftUnit != rightUnit {
			return leftUnit < rightUnit
		}
		return payload.Units[i].URI < payload.Units[j].URI
	})

	payload.TotalUnits = len(payload.Units)
	return marshalSymbolsOverview(payload)
}

func fetchSymbolsOverviewRaw(ctx context.Context, client *lsp.Client, query string) ([]symbolsOverviewRawSymbol, error) {
	var rawResult json.RawMessage
	if err := client.Call(ctx, "workspace/symbol", protocol.WorkspaceSymbolParams{Query: query}, &rawResult); err != nil {
		return nil, fmt.Errorf("workspace/symbol request failed: %w", err)
	}

	if len(rawResult) == 0 || string(rawResult) == "null" {
		return make([]symbolsOverviewRawSymbol, 0), nil
	}

	var rawSymbols []symbolsOverviewRawSymbol
	if err := json.Unmarshal(rawResult, &rawSymbols); err != nil {
		return nil, fmt.Errorf("failed to parse workspace symbols: %w", err)
	}

	if rawSymbols == nil {
		return make([]symbolsOverviewRawSymbol, 0), nil
	}

	return rawSymbols, nil
}

func marshalSymbolsOverview(payload symbolsOverviewResponse) (string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to encode symbols overview: %w", err)
	}
	return string(encoded), nil
}

func deriveUnitNameFromURI(uri string) string {
	if parsed, err := url.Parse(uri); err == nil {
		base := path.Base(parsed.Path)
		name := strings.TrimSuffix(base, path.Ext(base))
		if strings.TrimSpace(name) != "" && name != "." && name != "/" {
			return name
		}
	}

	cleanURI := strings.ReplaceAll(uri, "\\", "/")
	base := path.Base(cleanURI)
	name := strings.TrimSuffix(base, path.Ext(base))
	if strings.TrimSpace(name) != "" && name != "." && name != "/" {
		return name
	}

	if strings.TrimSpace(uri) == "" {
		return "unknown"
	}

	return uri
}

func normalizeOverviewKind(kind any) any {
	if kind == nil {
		return "unknown"
	}
	return kind
}

func buildSymbolsOverviewTrace(symbolName string, unitName string, containerName string) symbolsOverviewItemTrace {
	simpleCandidate := strings.TrimSpace(symbolName)
	containerCandidate := composeQualifiedCandidate(containerName, simpleCandidate)
	unitCandidate := ""
	if isTraceEligibleUnitName(unitName) {
		unitCandidate = composeQualifiedCandidate(unitName, simpleCandidate)
	}

	preferred := simpleCandidate
	if containerCandidate != "" {
		preferred = containerCandidate
	} else if unitCandidate != "" {
		preferred = unitCandidate
	}

	orderedCandidates := reorderAndDeduplicateCandidates(preferred, containerCandidate, unitCandidate, simpleCandidate)

	return symbolsOverviewItemTrace{
		PreferredSymbolName:  preferred,
		SymbolNameCandidates: orderedCandidates,
		Definition: symbolsOverviewTraceToolRequest{
			SymbolName: preferred,
		},
		References: symbolsOverviewTraceToolRequest{
			SymbolName: preferred,
		},
	}
}

func composeQualifiedCandidate(prefix string, symbolName string) string {
	normalizedPrefix := normalizeTraceQualifier(prefix)
	normalizedName := strings.TrimSpace(symbolName)
	if normalizedPrefix == "" || normalizedName == "" {
		return ""
	}
	return normalizedPrefix + "." + normalizedName
}

func normalizeTraceQualifier(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		switch r {
		case '.', ':', '/', '\\':
			return true
		default:
			return false
		}
	})

	cleanParts := make([]string, 0, len(parts))
	for _, part := range parts {
		normalizedPart := strings.TrimSpace(part)
		if normalizedPart == "" {
			continue
		}
		cleanParts = append(cleanParts, normalizedPart)
	}

	if len(cleanParts) == 0 {
		return ""
	}

	return strings.Join(cleanParts, ".")
}

func isTraceEligibleUnitName(unitName string) bool {
	trimmed := strings.TrimSpace(unitName)
	if trimmed == "" || strings.EqualFold(trimmed, "unknown") {
		return false
	}
	if strings.Contains(trimmed, "://") || strings.Contains(trimmed, "/") || strings.Contains(trimmed, "\\") {
		return false
	}
	return true
}

func reorderAndDeduplicateCandidates(preferred string, candidates ...string) []string {
	seen := make(map[string]struct{}, len(candidates)+1)
	ordered := make([]string, 0, len(candidates)+1)

	appendCandidate := func(candidate string) {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			return
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		ordered = append(ordered, trimmed)
	}

	appendCandidate(preferred)
	for _, candidate := range candidates {
		appendCandidate(candidate)
	}

	if len(ordered) <= 1 {
		return ordered
	}

	rest := append([]string(nil), ordered[1:]...)
	sort.Slice(rest, func(i, j int) bool {
		left := strings.ToLower(rest[i])
		right := strings.ToLower(rest[j])
		if left != right {
			return left < right
		}
		return rest[i] < rest[j]
	})

	return append([]string{ordered[0]}, rest...)
}

func kindSortKey(kind any) string {
	return strings.ToLower(fmt.Sprint(kind))
}

func compareOverviewItems(left symbolsOverviewItem, right symbolsOverviewItem) int {
	leftName := strings.ToLower(strings.TrimSpace(left.Name))
	rightName := strings.ToLower(strings.TrimSpace(right.Name))
	if leftName < rightName {
		return -1
	}
	if leftName > rightName {
		return 1
	}

	leftKind := kindSortKey(left.Kind)
	rightKind := kindSortKey(right.Kind)
	if leftKind < rightKind {
		return -1
	}
	if leftKind > rightKind {
		return 1
	}

	leftContainer := strings.ToLower(strings.TrimSpace(left.ContainerName))
	rightContainer := strings.ToLower(strings.TrimSpace(right.ContainerName))
	if leftContainer < rightContainer {
		return -1
	}
	if leftContainer > rightContainer {
		return 1
	}

	leftPreferred := strings.ToLower(strings.TrimSpace(left.Trace.PreferredSymbolName))
	rightPreferred := strings.ToLower(strings.TrimSpace(right.Trace.PreferredSymbolName))
	if leftPreferred < rightPreferred {
		return -1
	}
	if leftPreferred > rightPreferred {
		return 1
	}

	return 0
}
