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
	Name string `json:"name"`
	Kind any    `json:"kind"`
}

// GetSymbolsOverview aggregates workspace symbols by URI and returns a compact JSON overview.
func GetSymbolsOverview(ctx context.Context, client *lsp.Client, query string) (string, error) {
	trimmedQuery := strings.TrimSpace(query)

	symbolResult, err := client.Symbol(ctx, protocol.WorkspaceSymbolParams{Query: trimmedQuery})
	if err != nil {
		return "", fmt.Errorf("workspace/symbol request failed: %w", err)
	}

	results, err := symbolResult.Results()
	if err != nil {
		return "", fmt.Errorf("failed to parse workspace symbols: %w", err)
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
		symbolName := strings.TrimSpace(sym.GetName())
		if symbolName == "" {
			continue
		}

		uri := strings.TrimSpace(string(sym.GetLocation().URI))
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
			Name: symbolName,
			Kind: extractOverviewSymbolKind(sym),
		})
	}

	payload.Units = make([]symbolsOverviewUnitEntry, 0, len(unitsByURI))
	for _, unit := range unitsByURI {
		sort.Slice(unit.Symbols, func(i, j int) bool {
			leftName := strings.ToLower(unit.Symbols[i].Name)
			rightName := strings.ToLower(unit.Symbols[j].Name)
			if leftName != rightName {
				return leftName < rightName
			}
			return kindSortKey(unit.Symbols[i].Kind) < kindSortKey(unit.Symbols[j].Kind)
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

func extractOverviewSymbolKind(sym protocol.WorkspaceSymbolResult) any {
	if si, ok := sym.(*protocol.SymbolInformation); ok {
		return si.Kind
	}
	if ws, ok := sym.(*protocol.WorkspaceSymbol); ok {
		return ws.Kind
	}
	return "unknown"
}

func kindSortKey(kind any) string {
	return strings.ToLower(fmt.Sprint(kind))
}
