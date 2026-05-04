package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// GetCodeActions requests code actions from the LSP server for the given file
// position and returns the result as a JSON-indented string.
//
// Parameters:
//   - filePath: absolute path or URI of the Delphi source file
//   - line: 1-indexed line number for the cursor position
//   - column: 1-indexed column number for the cursor position
//   - only: optional filter for action kinds (e.g. ["quickfix"])
//   - includeDiagnostics: when true, requests current file diagnostics from the
//     server and attaches them to the code-action context; when false the
//     diagnostics array is sent empty (avoids an extra round-trip)
func GetCodeActions(
	ctx context.Context,
	client *lsp.Client,
	filePath string,
	line int,
	column int,
	only []string,
	includeDiagnostics bool,
) (string, error) {
	normalizedPath, err := normalizeFilePathOrURI(filePath)
	if err != nil {
		return "", fmt.Errorf("invalid file path or URI: %w", err)
	}

	if err := client.OpenFile(ctx, normalizedPath); err != nil {
		return "", fmt.Errorf("could not open file: %w", err)
	}

	uri := protocol.URIFromPath(normalizedPath)

	// Zero-indexed LSP position from 1-indexed user input.
	lspLine := uint32(line - 1)
	lspChar := uint32(column - 1)

	codeActionRange := protocol.Range{
		Start: protocol.Position{Line: lspLine, Character: lspChar},
		End:   protocol.Position{Line: lspLine, Character: lspChar + 1},
	}

	var diagnostics []protocol.Diagnostic
	if includeDiagnostics {
		diagnostics = client.GetFileDiagnostics(uri)
	}

	onlyKinds := make([]protocol.CodeActionKind, 0, len(only))
	for _, k := range only {
		onlyKinds = append(onlyKinds, protocol.CodeActionKind(k))
	}

	params := protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
		Range:        codeActionRange,
		Context: protocol.CodeActionContext{
			Diagnostics: diagnostics,
			Only:        onlyKinds,
		},
	}

	var raw json.RawMessage
	if err := client.Call(ctx, "textDocument/codeAction", params, &raw); err != nil {
		return "", fmt.Errorf("textDocument/codeAction request failed: %w", err)
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
