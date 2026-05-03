package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
)

const defaultGraphQueryDirection = "both"
const defaultGraphQueryRelationType = "uses_unit"

// GetGraphQuery returns a bounded graph neighborhood for a unit as formatted JSON.
// relationType defaults to "uses_unit", direction defaults to "both" and depth defaults to 1.
func GetGraphQuery(ctx context.Context, client *lsp.Client, uri string, relationType string, direction string, depth int) (string, error) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return "", fmt.Errorf("uri must be a non-empty string")
	}

	relationType = strings.TrimSpace(relationType)
	if relationType == "" {
		relationType = defaultGraphQueryRelationType
	}

	direction = strings.TrimSpace(direction)
	if direction == "" {
		direction = defaultGraphQueryDirection
	}

	if depth < 0 {
		return "", fmt.Errorf("depth must be greater than or equal to 0")
	}

	params := map[string]any{
		"uri":          uri,
		"relationType": relationType,
		"direction":    direction,
		"depth":        depth,
	}

	var raw json.RawMessage
	err := client.Call(ctx, "custom/graph/query", params, &raw)
	if err != nil {
		return "", fmt.Errorf("custom/graph/query request failed: %w", err)
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "Graph query not available for the requested file or relation.", nil
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
