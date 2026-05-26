package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestReleaseInstall_VendoredParserPresent asserts the published module root ships
// the Delphi tree-sitter vendor required by go.mod replace (no ../Delphi_Oracle).
func TestReleaseInstall_VendoredParserPresent(t *testing.T) {
	t.Helper()
	root := moduleRoot(t)
	parserC := filepath.Join(root, "third_party", "tree-sitter-delphi6", "src", "parser.c")
	if _, err := os.Stat(parserC); err != nil {
		t.Fatalf("expected vendored parser at %s: %v", parserC, err)
	}
	modPath := filepath.Join(root, "go.mod")
	modBytes, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	mod := string(modBytes)
	if !strings.Contains(mod, "replace github.com/tree-sitter/tree-sitter-delphi6 => ./third_party/tree-sitter-delphi6/bindings/go") {
		t.Fatalf("go.mod must replace tree-sitter-delphi6 with vendored bindings under third_party")
	}
	if strings.Contains(mod, "../Delphi_Oracle") {
		t.Fatalf("go.mod must not depend on sibling Delphi_Oracle checkout")
	}
}

// TestReleaseInstall_GoInstallFromModuleRoot is the supported release path documented in README:
// clone (or CI checkout) + go install . with no monorepo sibling.
// readmeInstallRequiredPhrases are the public install/CLI contract documented in README.md
// and enforced by CI (TestReleaseInstall_*).
var readmeInstallRequiredPhrases = []string{
	"go install .",
	"third_party/tree-sitter-delphi6/",
	"go install github.com/isaacphi/mcp-language-server@latest",
	"v0.1.2",
	"--help",
	"Exit code",
}

// TestReleaseInstall_ReadmeDocumentsSupportedPaths ensures README matches the supported
// release path validated by TestReleaseInstall_GoInstallFromModuleRoot.
func TestReleaseInstall_ReadmeDocumentsSupportedPaths(t *testing.T) {
	root := moduleRoot(t)
	readmeBytes, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	readme := string(readmeBytes)
	for _, phrase := range readmeInstallRequiredPhrases {
		if !strings.Contains(readme, phrase) {
			t.Fatalf("README.md must document supported install/CLI phrase %q", phrase)
		}
	}
	if strings.Contains(readme, "replace github.com/tree-sitter/tree-sitter-delphi6 => ../Delphi_Oracle") {
		t.Fatal("README must not instruct sibling Delphi_Oracle checkout as the primary install path")
	}
}

func TestReleaseInstall_GoInstallFromModuleRoot(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess go install in -short mode")
	}
	root := moduleRoot(t)
	gopath := t.TempDir()
	gobin := filepath.Join(gopath, "bin")
	if err := os.MkdirAll(gobin, 0o755); err != nil {
		t.Fatalf("mkdir gobin: %v", err)
	}
	env := append(os.Environ(),
		"GOPATH="+gopath,
		"GOBIN="+gobin,
		"GO111MODULE=on",
	)
	cmd := exec.Command("go", "install", ".")
	cmd.Dir = root
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go install . failed: %v\nstderr:\n%s", err, stderr.String())
	}
	binary := "mcp-language-server"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	installed := filepath.Join(gobin, binary)
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("expected binary at %s: %v", installed, err)
	}
	help := exec.Command(installed, "--help")
	help.Env = env
	if out, err := help.CombinedOutput(); err != nil {
		t.Fatalf("--help failed: %v\n%s", err, out)
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}
