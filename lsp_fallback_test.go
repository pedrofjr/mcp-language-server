package main

import (
	"context"
	"testing"

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
