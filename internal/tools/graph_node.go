package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
)

const defaultGraphNodeRelationType = "uses_unit"

// GetGraphNode returns graph node relations for a unit as formatted JSON.
// relationType defaults to "uses_unit".
func GetGraphNode(ctx context.Context, client *lsp.Client, uri string, relationType string) (string, error) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return "", fmt.Errorf("uri must be a non-empty string")
	}

	relationType = strings.TrimSpace(relationType)
	if relationType == "" {
		relationType = defaultGraphNodeRelationType
	}

	params := map[string]any{
		"uri":          uri,
		"relationType": relationType,
	}

	var raw json.RawMessage
	err := client.Call(ctx, "custom/graph/node", params, &raw)
	if err != nil {
		return "", fmt.Errorf("custom/graph/node request failed: %w", err)
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "Graph node not available for the requested file or relation.", nil
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
