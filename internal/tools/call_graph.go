package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
)

const maxCallGraphDepth = 20

// GetCallGraph returns the call graph for a symbol as a formatted JSON string.
// symbolName must be a non-empty symbol identifier accepted by the language server.
// depth defaults to 1 when <= 0 and is capped at maxCallGraphDepth.
func GetCallGraph(ctx context.Context, client *lsp.Client, symbolName string, depth int) (string, error) {
	symbolName = strings.TrimSpace(symbolName)
	if symbolName == "" {
		return "", fmt.Errorf("symbolName must be a non-empty string")
	}

	if depth <= 0 {
		depth = 1
	}
	if depth > maxCallGraphDepth {
		depth = maxCallGraphDepth
	}

	params := map[string]any{
		"symbolName": symbolName,
		"depth":      depth,
	}

	var raw json.RawMessage
	err := client.Call(ctx, "custom/callGraph", params, &raw)
	if err != nil {
		return "", fmt.Errorf("custom/callGraph request failed: %w", err)
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "Call graph not available for the requested file or symbol.", nil
	}

	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return string(raw), nil
	}

	out, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return string(raw), nil
	}

	return string(out), nil
}
