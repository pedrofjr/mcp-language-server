package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// GetWorkspaceSymbols searches for symbols across the entire workspace matching the given query.
func GetWorkspaceSymbols(ctx context.Context, client *lsp.Client, query string) (string, error) {
	symbolResult, err := client.Symbol(ctx, protocol.WorkspaceSymbolParams{
		Query: query,
	})
	if err != nil {
		return "", fmt.Errorf("workspace/symbol request failed: %w", err)
	}

	results, err := symbolResult.Results()
	if err != nil {
		return "", fmt.Errorf("failed to parse workspace symbols: %w", err)
	}

	if len(results) == 0 {
		return "No symbols found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d symbol(s):\n\n", len(results)))
	for _, sym := range results {
		loc := sym.GetLocation()
		kindStr := ""
		containerStr := ""

		if si, ok := sym.(*protocol.SymbolInformation); ok {
			if name, found := protocol.TableKindMap[si.Kind]; found {
				kindStr = fmt.Sprintf(" [%s]", name)
			}
			if si.ContainerName != "" {
				containerStr = fmt.Sprintf(" (in %s)", si.ContainerName)
			}
		}

		sb.WriteString(fmt.Sprintf("- %s%s%s\n  %s:%d:%d\n",
			sym.GetName(),
			kindStr,
			containerStr,
			loc.URI,
			loc.Range.Start.Line+1,
			loc.Range.Start.Character+1,
		))
	}
	return sb.String(), nil
}
