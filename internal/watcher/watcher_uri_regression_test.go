package watcher

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

type fakeWatcherLSPClient struct {
	openFiles      map[string]bool
	notifyChange   []string
	watchedChanges []protocol.FileEvent
}

func (f *fakeWatcherLSPClient) IsFileOpen(path string) bool {
	return f.openFiles[path]
}

func (f *fakeWatcherLSPClient) OpenFile(ctx context.Context, path string) error {
	if f.openFiles == nil {
		f.openFiles = make(map[string]bool)
	}
	f.openFiles[path] = true
	return nil
}

func (f *fakeWatcherLSPClient) NotifyChange(ctx context.Context, path string) error {
	f.notifyChange = append(f.notifyChange, path)
	return nil
}

func (f *fakeWatcherLSPClient) DidChangeWatchedFiles(ctx context.Context, params protocol.DidChangeWatchedFilesParams) error {
	f.watchedChanges = append(f.watchedChanges, params.Changes...)
	return nil
}

func TestIsPathWatched_RelativePatternBaseURI_IsCanonical(t *testing.T) {
	client := &fakeWatcherLSPClient{openFiles: make(map[string]bool)}
	w := NewWorkspaceWatcher(client)

	basePath := filepath.Join(`C:\Users\alice`, "workspace with spaces")
	targetPath := filepath.Join(basePath, "main.go")

	w.registrations = []protocol.FileSystemWatcher{
		{
			GlobPattern: protocol.GlobPattern{Value: protocol.RelativePattern{
				BaseURI: protocol.Or_RelativePattern_baseUri{Value: protocol.DocumentUri(protocol.URIFromPath(basePath))},
				Pattern: "*.go",
			}},
		},
	}

	matched, _ := w.isPathWatched(targetPath)
	if !matched {
		t.Fatalf("expected isPathWatched to match %q against base URI %q with canonical URI decoding", targetPath, string(protocol.URIFromPath(basePath)))
	}
}

func TestHandleFileEvent_ChangedOpenFile_UsesCanonicalPath(t *testing.T) {
	filePath := filepath.Join(`C:\Users\alice`, "workspace with spaces", "main.go")
	uri := string(protocol.URIFromPath(filePath))

	client := &fakeWatcherLSPClient{
		openFiles: map[string]bool{filePath: true},
	}
	w := NewWorkspaceWatcher(client)

	w.handleFileEvent(context.Background(), uri, protocol.FileChangeType(protocol.Changed))

	if len(client.notifyChange) != 1 || client.notifyChange[0] != filePath {
		t.Fatalf("expected changed open file to call NotifyChange with canonical path %q, got notify calls: %v", filePath, client.notifyChange)
	}

	if len(client.watchedChanges) != 0 {
		t.Fatalf("expected changed open file to avoid didChangeWatchedFiles fallback, got %d watched event(s)", len(client.watchedChanges))
	}
}
