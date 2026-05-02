package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
)

// GetAstSummary returns the interface-section structure of a Delphi unit as a formatted JSON string.
// The uri parameter should be a file URI (e.g. file:///path/to/Unit1.pas).
func GetAstSummary(ctx context.Context, client *lsp.Client, uri string) (string, error) {
	params := map[string]string{"uri": uri}
	var raw json.RawMessage
	err := client.Call(ctx, "custom/astSummary", params, &raw)
	if err != nil {
		return "", fmt.Errorf("custom/astSummary request failed: %w", err)
	}

	if len(raw) == 0 || string(raw) == "null" {
		return "File not found or not loaded by the LSP server.", nil
	}

	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw), nil
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(raw), nil
	}
	return string(out), nil
}
