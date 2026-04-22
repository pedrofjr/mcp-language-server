package lsp

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

type discardWriteCloser struct{}

func (discardWriteCloser) Write(p []byte) (int, error) {
	return len(p), nil
}

func (discardWriteCloser) Close() error {
	return nil
}

var _ io.WriteCloser = discardWriteCloser{}

func TestCloseAllFiles_CanonicalizesEncodedWindowsURI(t *testing.T) {
	filePath := filepath.Join(`C:\Users\alice`, "folder with space", "main.go")
	uri := string(protocol.URIFromPath(filePath))

	client := &Client{
		stdin:     discardWriteCloser{},
		openFiles: make(map[string]*OpenFileInfo),
	}
	client.openFiles[uri] = &OpenFileInfo{Version: 1, URI: protocol.DocumentUri(uri)}

	client.CloseAllFiles(context.Background())

	remaining := client.GetOpenFilesSnapshot()
	if len(remaining) != 0 {
		t.Fatalf("expected CloseAllFiles to close encoded Windows URI %q, but %d open file(s) remain: %v", uri, len(remaining), remaining)
	}
}
