package main

import (
	"os"
	"path/filepath"
	"testing"

	tree_sitter_delphi6 "github.com/tree-sitter/tree-sitter-delphi6"
)

func TestTreeSitterDelphi6_VendoredModuleResolves(t *testing.T) {
	parserPath := filepath.Join("third_party", "tree-sitter-delphi6", "src", "parser.c")
	if _, err := os.Stat(parserPath); err != nil {
		t.Fatalf("expected vendored parser at %s: %v", parserPath, err)
	}
	if tree_sitter_delphi6.Language() == nil {
		t.Fatal("expected non-nil tree-sitter Language() from vendored delphi6 module")
	}
}
