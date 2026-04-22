package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

func TestExtractTextFromLocation_ParsesWindowsStyleFileURI(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-specific regression test")
	}

	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "sample.txt")
	if err := os.WriteFile(filePath, []byte("alpha beta\ngamma\n"), 0o644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	loc := protocol.Location{
		URI: protocol.URIFromPath(filePath),
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 6},
			End:   protocol.Position{Line: 0, Character: 10},
		},
	}

	text, err := ExtractTextFromLocation(loc)
	if err != nil {
		t.Fatalf("expected ExtractTextFromLocation to parse canonical file URI, got error: %v", err)
	}

	if text != "beta" {
		t.Fatalf("unexpected extracted text: got %q, want %q", text, "beta")
	}
}