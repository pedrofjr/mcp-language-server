package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
)

const defaultSemanticSearchLimit = 20

// GetSemanticSearch executes custom/semanticSearch and returns a formatted JSON payload.
// query must be non-empty after trim. scope accepts "workspace" or "file" and defaults to "workspace".
// uri is required when scope is "file". limit defaults to 20 when <= 0.
func GetSemanticSearch(ctx context.Context, client *lsp.Client, query string, scope string, uri string, limit int) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("query must be a non-empty string")
	}

	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "workspace"
	}
	if scope != "workspace" && scope != "file" {
		return "", fmt.Errorf("scope must be 'workspace' or 'file'")
	}

	uri = strings.TrimSpace(uri)
	if scope == "file" && uri == "" {
		return "", fmt.Errorf("uri is required when scope='file'")
	}

	if limit <= 0 {
		limit = defaultSemanticSearchLimit
	}

	params := map[string]any{
		"query": query,
		"scope": scope,
		"limit": limit,
	}
	if scope == "file" {
		params["uri"] = uri
	}

	var raw json.RawMessage
	err := client.Call(ctx, "custom/semanticSearch", params, &raw)
	if err != nil {
		return "", fmt.Errorf("custom/semanticSearch request failed: %w", err)
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "No semantic search results found.", nil
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
