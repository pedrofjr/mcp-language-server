package main

import (
	"os"
	"strings"
	"testing"
)

func TestRunQuery_PublicContractSingleSource(t *testing.T) {
	needle := strings.ToLower(runQueryPublicContract)
	if !strings.Contains(needle, "fallback") || !strings.Contains(needle, "lsp") {
		t.Fatal("runQueryPublicContract must document degraded fallback and LSP authority")
	}

	src, err := os.ReadFile("tools.go")
	if err != nil {
		t.Fatalf("read tools.go: %v", err)
	}
	if !strings.Contains(string(src), "runQueryPublicContract") {
		t.Fatal("tools.go must reference runQueryPublicContract in run_query registration")
	}

	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	readmeText := strings.ToLower(string(readme))
	if !strings.Contains(readmeText, "fallback") || !strings.Contains(readmeText, "degradad") {
		t.Fatal("README.md must document run_query degraded fallback")
	}
	if !strings.Contains(readmeText, "semântica") && !strings.Contains(readmeText, "semantica") {
		t.Fatal("README.md must state run_query does not replace Delphi semantic analysis")
	}
	if !strings.Contains(readmeText, "lsp") {
		t.Fatal("README.md must reference LSP-backed tools as authoritative")
	}

	claudePath := "CLAUDE.md"
	if _, statErr := os.Stat(claudePath); statErr == nil {
		claude, readErr := os.ReadFile(claudePath)
		if readErr != nil {
			t.Fatalf("read CLAUDE.md: %v", readErr)
		}
		claudeText := strings.ToLower(string(claude))
		if strings.Contains(claudeText, "run_query") {
			if !strings.Contains(claudeText, "fallback") && !strings.Contains(claudeText, "degradad") {
				t.Fatal("CLAUDE.md must document run_query degraded fallback when run_query is mentioned")
			}
		}
	}
}

func TestRunQuery_ToolDescriptionDocumentsDegradedFallback(t *testing.T) {
	TestRunQuery_PublicContractSingleSource(t)
}
