package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDelphiWorkspaceExtensions_MatchesReferenceMatrix(t *testing.T) {
	expected := map[string]struct{}{
		".pas": {}, ".pp": {}, ".dpr": {}, ".dpk": {}, ".lpr": {}, ".inc": {},
	}
	assert.Len(t, DelphiWorkspaceExtensions, len(expected))
	for _, ext := range DelphiWorkspaceExtensions {
		_, ok := expected[ext]
		assert.True(t, ok, "unexpected extension %q in DelphiWorkspaceExtensions", ext)
	}
}

func TestIsDelphiWorkspaceSourceFile_AcceptsCanonicalExtensions(t *testing.T) {
	for _, path := range []string{
		"Unit1.pas",
		"legacy.pp",
		"App.dpr",
		"Pkg.dpk",
		"Console.lpr",
		"Shared.inc",
	} {
		assert.True(t, IsDelphiWorkspaceSourceFile(path), "expected %q to be Delphi workspace source", path)
	}
}

func TestIsDelphiWorkspaceSourceFile_RejectsNonDelphi(t *testing.T) {
	for _, path := range []string{"readme.md", "main.go", "data.json"} {
		assert.False(t, IsDelphiWorkspaceSourceFile(path), "expected %q to be rejected", path)
	}
}

func TestIsDelphiWorkspaceReferenceFile_AliasesCanonicalMatrix(t *testing.T) {
	assert.True(t, isDelphiWorkspaceReferenceFile("frag.inc"))
	assert.False(t, isDelphiWorkspaceReferenceFile("notes.txt"))
}
