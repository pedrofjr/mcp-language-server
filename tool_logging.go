package main

import (
	"context"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/mark3labs/mcp-go/mcp"
)

// withLSPGuard verifica se o client LSP está disponível antes de executar o handler.
// Se lspClient for nil, retorna um ToolResultError consistente sem chamar o handler interno.
func withLSPGuard(client *lsp.Client, h func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if client == nil {
			return mcp.NewToolResultError("language server not available"), nil
		}
		return h(ctx, req)
	}
}

// withToolLogging envolve um handler de tool MCP adicionando logs estruturados
// automáticos de início, fim e erro, com duração em ms.
// Formato: tool=<name> action=start|end|error duration_ms=<N> [err=<msg>]
func withToolLogging(name string, h func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		coreLogger.Debug("tool=%s action=start", name)
		start := time.Now()
		result, err := h(ctx, req)
		durationMs := time.Since(start).Milliseconds()
		if err != nil || (result != nil && result.IsError) {
			coreLogger.Error("tool=%s action=error duration_ms=%d err=%v", name, durationMs, err)
		} else {
			coreLogger.Debug("tool=%s action=end duration_ms=%d", name, durationMs)
		}
		return result, err
	}
}
