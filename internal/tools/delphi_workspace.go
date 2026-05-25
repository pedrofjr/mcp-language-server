package tools

import (
	"path/filepath"
	"strings"
)

// DelphiWorkspaceExtensions is the canonical matrix shared by run_query workspace
// scans, references/delphi_oracle fallback walks, and safe_delete cross-file guards.
var DelphiWorkspaceExtensions = []string{".pas", ".pp", ".dpr", ".dpk", ".lpr", ".inc"}

// DelphiWorkspaceExtensionsDoc is a stable human-readable list for README/tools/list.
const DelphiWorkspaceExtensionsDoc = ".pas, .pp, .dpr, .dpk, .lpr, .inc"

// IsDelphiWorkspaceSourceFile reports whether path uses a Delphi workspace source extension.
func IsDelphiWorkspaceSourceFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pas", ".pp", ".dpr", ".dpk", ".lpr", ".inc":
		return true
	default:
		return false
	}
}

func isDelphiWorkspaceReferenceFile(path string) bool {
	return IsDelphiWorkspaceSourceFile(path)
}
