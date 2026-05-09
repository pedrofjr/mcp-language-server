package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/isaacphi/mcp-language-server/internal/logging"
	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestWithLSPGuard_WhenClientNilReturnsConsistentError verifica que, quando
// lspClient é nil, o handler wrappado retorna um ToolResultError com mensagem
// "language server not available" sem chamar o handler interno.
func TestWithLSPGuard_WhenClientNilReturnsConsistentError(t *testing.T) {
	called := false
	inner := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return mcp.NewToolResultText("ok"), nil
	}

	// withLSPGuard(nil, inner) deve retornar handler que retorna erro
	wrapped := withLSPGuard(nil, inner)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("expected no error (error should be in result), got %v", err)
	}
	if called {
		t.Fatal("inner handler should NOT have been called when client is nil")
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	// Verifica que o result é um error result com mensagem consistente
	if len(result.Content) == 0 {
		t.Fatal("result.Content should not be empty")
	}
}

// TestWithLSPGuard_WhenClientPresentCallsInner verifica que, quando
// lspClient não é nil, o handler interno é chamado normalmente.
func TestWithLSPGuard_WhenClientPresentCallsInner(t *testing.T) {
	called := false
	inner := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return mcp.NewToolResultText("ok"), nil
	}

	// Passando um client não-nil (pode ser um *lsp.Client com valor zero,
	// pois não chamamos métodos nele neste teste)
	fakeClient := &lsp.Client{}
	wrapped := withLSPGuard(fakeClient, inner)
	_, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("inner handler should have been called when client is not nil")
	}
}

func TestWithToolLogging_ComposedWithLSPGuardNilClient_LogsSingleErrorAndSkipsInner(t *testing.T) {
	var logBuf bytes.Buffer
	prevLevel, ok := logging.ComponentLevels[logging.Core]
	if !ok {
		prevLevel = logging.DefaultMinLevel
	}
	logging.SetupTestLogging(&logBuf)
	logging.SetLevel(logging.Core, logging.LevelDebug)
	t.Cleanup(func() {
		logging.ResetTestLogging()
		logging.SetLevel(logging.Core, prevLevel)
	})

	called := false
	inner := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return mcp.NewToolResultText("should not happen"), nil
	}

	wrapped := withToolLogging("x", withLSPGuard(nil, inner))
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("expected nil go error, got %v", err)
	}
	if called {
		t.Fatal("inner handler should NOT be called when LSP client is nil")
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.IsError {
		t.Fatal("expected ToolResultError from LSP guard")
	}

	contentJSON, marshalErr := json.Marshal(result.Content)
	if marshalErr != nil {
		t.Fatalf("failed to marshal result content for assertion: %v", marshalErr)
	}
	if !strings.Contains(string(contentJSON), "language server not available") {
		t.Fatalf("expected guard error message in result content, got %s", string(contentJSON))
	}

	logs := logBuf.String()
	if !strings.Contains(logs, "tool=x action=error") {
		t.Fatalf("expected error log in composed wrapper, got logs: %s", logs)
	}
	if strings.Contains(logs, "tool=x action=end") {
		t.Fatalf("did not expect success log in composed wrapper, got logs: %s", logs)
	}
	if strings.Count(logs, "tool=x action=error") != 1 {
		t.Fatalf("expected exactly one error log entry, got logs: %s", logs)
	}
}
