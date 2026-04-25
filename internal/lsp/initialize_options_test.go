package lsp

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

func TestNormalizeInitializeOptions_WorkspaceSettings_FileSchemeCaseInsensitiveAndEncodedSpace(t *testing.T) {
	options := InitializeOptions{
		WorkspaceSettings: map[string]WorkspaceInitializationOptions{
			"FILE:///C:/Program%20Files/My%20Repo": {
				SearchPaths: []string{"Source"},
			},
		},
	}

	normalized, err := NormalizeInitializeOptions(options)
	if err != nil {
		t.Fatalf("NormalizeInitializeOptions returned error: %v", err)
	}

	if len(normalized.WorkspaceSettings) != 1 {
		t.Fatalf("expected exactly one workspace setting, got %d", len(normalized.WorkspaceSettings))
	}

	expectedRoot := string(protocol.URIFromPath(`C:\Program Files\My Repo`))
	settings, ok := normalized.WorkspaceSettings[expectedRoot]
	if !ok {
		t.Fatalf("expected normalized workspace key %q in workspaceSettings", expectedRoot)
	}

	if got, want := len(settings.SearchPaths), 1; got != want {
		t.Fatalf("unexpected workspace searchPaths length: got=%d want=%d", got, want)
	}

	if got, want := settings.SearchPaths[0], "Source"; got != want {
		t.Fatalf("unexpected workspace search path: got=%q want=%q", got, want)
	}
}

func TestNormalizeInitializeOptions_DelphiInstallationPath_FileURIConvertedToLocalPath(t *testing.T) {
	options := InitializeOptions{
		DelphiInstallationPath: "file:///C:/Delphi6",
	}

	normalized, err := NormalizeInitializeOptions(options)
	if err != nil {
		t.Fatalf("NormalizeInitializeOptions returned error: %v", err)
	}

	expectedPath := protocol.DocumentUri("file:///C:/Delphi6").Path()
	if got, want := normalized.DelphiInstallationPath, expectedPath; got != want {
		t.Fatalf("unexpected normalized delphiInstallationPath: got=%q want=%q", got, want)
	}

	if runtime.GOOS == "windows" {
		if got, want := filepath.VolumeName(normalized.DelphiInstallationPath), "C:"; got != want {
			t.Fatalf("unexpected drive letter in delphiInstallationPath: got=%q want=%q", got, want)
		}
	}
}

func TestNormalizeInitializeOptions_SearchPathsRelativosPreservados(t *testing.T) {
	options := InitializeOptions{
		SearchPaths: []string{"  ./src  ", "..\\shared", "vendor/pkg"},
	}

	normalized, err := NormalizeInitializeOptions(options)
	if err != nil {
		t.Fatalf("NormalizeInitializeOptions returned error: %v", err)
	}

	if got, want := len(normalized.SearchPaths), 3; got != want {
		t.Fatalf("unexpected searchPaths length: got=%d want=%d", got, want)
	}

	if got, want := normalized.SearchPaths[0], "./src"; got != want {
		t.Fatalf("unexpected first search path: got=%q want=%q", got, want)
	}

	if got, want := normalized.SearchPaths[1], "..\\shared"; got != want {
		t.Fatalf("unexpected second search path: got=%q want=%q", got, want)
	}

	if got, want := normalized.SearchPaths[2], "vendor/pkg"; got != want {
		t.Fatalf("unexpected third search path: got=%q want=%q", got, want)
	}
}
