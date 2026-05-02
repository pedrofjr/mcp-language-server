package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
)

// GetDependencyTree returns the dependency tree for a Delphi unit.
// direction must be "imports" (shows what the unit depends on) or "importedBy" (reverse).
// The uri parameter should be a file URI (e.g. file:///path/to/Unit1.pas).
func GetDependencyTree(ctx context.Context, client *lsp.Client, uri string, direction string) (string, error) {
	if direction == "" {
		direction = "imports"
	}
	params := map[string]string{"uri": uri, "direction": direction}
	var raw json.RawMessage
	err := client.Call(ctx, "custom/dependencyTree", params, &raw)
	if err != nil {
		return "", fmt.Errorf("custom/dependencyTree request failed: %w", err)
	}

	if len(raw) == 0 || string(raw) == "null" {
		return "Unit not found or not loaded by the LSP server.", nil
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
