package main

import (
	"os"
	"strings"
	"testing"
)

func TestRunQuery_ToolDescriptionDocumentsDegradedFallback(t *testing.T) {
	src, err := os.ReadFile("tools.go")
	if err != nil {
		t.Fatalf("read tools.go: %v", err)
	}
	text := strings.ToLower(string(src))
	if !strings.Contains(text, `mcp.newtool("run_query"`) {
		t.Fatal("run_query tool registration missing")
	}
	if !strings.Contains(text, "fallback") && !strings.Contains(text, "degradad") {
		t.Fatal("run_query description must document degraded fallback")
	}
	if !strings.Contains(text, "lsp-backed") && !strings.Contains(text, "fonte autoritativa") {
		t.Fatal("run_query description must reference LSP as authoritative")
	}
}
