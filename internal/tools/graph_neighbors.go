package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
)

const defaultGraphNeighborsDirection = "imports"
const defaultGraphNeighborsRelationType = "uses_unit"

// GetGraphNeighbors returns first-hop graph neighbors for a unit as formatted JSON.
// relationType defaults to "uses_unit" and direction defaults to "imports".
func GetGraphNeighbors(ctx context.Context, client *lsp.Client, uri string, relationType string, direction string) (string, error) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return "", fmt.Errorf("uri must be a non-empty string")
	}

	relationType = strings.TrimSpace(relationType)
	if relationType == "" {
		relationType = defaultGraphNeighborsRelationType
	}

	direction = strings.TrimSpace(direction)
	if direction == "" {
		direction = defaultGraphNeighborsDirection
	}

	params := map[string]any{
		"uri":          uri,
		"relationType": relationType,
		"direction":    direction,
	}

	var raw json.RawMessage
	err := client.Call(ctx, "custom/graph/neighbors", params, &raw)
	if err != nil {
		return "", fmt.Errorf("custom/graph/neighbors request failed: %w", err)
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "Graph neighbors not available for the requested file or relation.", nil
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
