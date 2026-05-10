package main

import (
	"context"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/mark3labs/mcp-go/mcp"
)

// Prefixos operacionais estáveis para caminhos críticos de erro.
const (
	OpLSPUnavailable    = "OP_LSP_UNAVAILABLE"
	OpDefinitionCanceled = "OP_DEFINITION_CANCELED"
	OpDefinitionDeadline = "OP_DEFINITION_DEADLINE"
	OpReferencesCanceled = "OP_REFERENCES_CANCELED"
	OpReferencesDeadline = "OP_REFERENCES_DEADLINE"
	OpRunQueryCanceled   = "OP_RUN_QUERY_CANCELED"
	OpRunQueryDeadline   = "OP_RUN_QUERY_DEADLINE"
)

// opErrMsg formata uma mensagem de erro operacional com prefixo estável.
// Formato: TOKEN | <mensagem legada>
func opErrMsg(token, msg string) string {
	return token + " | " + msg
}

// withLSPGuard verifica se o client LSP está disponível antes de executar o handler.
// Se lspClient for nil, retorna um ToolResultError consistente sem chamar o handler interno.
func withLSPGuard(client *lsp.Client, h func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if client == nil {
			return mcp.NewToolResultError(opErrMsg(OpLSPUnavailable, "language server not available | action: initialize language server client")), nil
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
