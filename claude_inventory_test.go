package main

import (
	"os"
	"strings"
	"testing"
)

func TestClaude_DoesNotCiteObsoleteToolCount(t *testing.T) {
	text, err := os.ReadFile("CLAUDE.md")
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	body := string(text)
	if strings.Contains(body, "6 ferramentas") || strings.Contains(body, "six tools") {
		t.Fatal("CLAUDE.md still cites obsolete fixed tool count")
	}
}

func TestClaude_MentionsLSPBridge(t *testing.T) {
	text, err := os.ReadFile("CLAUDE.md")
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(text), "LSP") || !strings.Contains(string(text), "MCP") {
		t.Fatal("CLAUDE.md must describe MCP/LSP bridge")
	}
}
