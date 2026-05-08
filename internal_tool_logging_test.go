package main

import (
	"context"
	"errors"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestWithToolLogging_WrapperExistsAndCallsUnderlyingHandler verifica que
// withToolLogging retorna um handler válido que delega ao handler interno.
func TestWithToolLogging_WrapperExistsAndCallsUnderlyingHandler(t *testing.T) {
	called := false
	inner := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return mcp.NewToolResultText("ok"), nil
	}
	wrapped := withToolLogging("test_tool", inner)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("inner handler was not called")
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

// TestWithToolLogging_PropagatesError verifica que erros do handler interno
// são propagados corretamente pelo wrapper.
func TestWithToolLogging_PropagatesError(t *testing.T) {
	expectedErr := errors.New("test error")
	inner := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, expectedErr
	}
	wrapped := withToolLogging("failing_tool", inner)
	_, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}
