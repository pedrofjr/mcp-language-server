package main

import (
	"context"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/mark3labs/mcp-go/mcp"
)

// Prefixos operacionais estáveis para caminhos críticos de erro.
const (
	OpLSPUnavailable     = "OP_LSP_UNAVAILABLE"
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
			return mcp.NewToolResultError(opErrMsgWithRecovery(OpLSPUnavailable, "language server not available | action: initialize language server client")), nil
		}
		return h(ctx, req)
	}
}

// withPreToolValidation executa pre antes do handler interno.
// Quando pre retorna done=true, o resultado de pre é devolvido imediatamente (ex.: validação local).
func withPreToolValidation(pre func(mcp.CallToolRequest) (*mcp.CallToolResult, bool), h func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if result, done := pre(req); done {
			return result, nil
		}
		return h(ctx, req)
	}
}

// withToolLogging envolve um handler de tool MCP com log estruturado v1.1+ (P3):
// tool, outcome, duration_ms, request_id, project_path, timestamp.
func withToolLogging(name string, h func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()
		requestID := formatRequestID(req)
		projectPath := extractProjectPath(req, workspaceDirFromContext(ctx))

		result, err := h(ctx, req)
		durationMs := time.Since(start).Milliseconds()
		outcome := classifyToolOutcome(result, err)

		logStructuredToolEvent(name, outcome, durationMs, requestID, projectPath)

		if result != nil && result.IsError {
			result = enrichToolResultWithRecovery(result)
		}

		return result, err
	}
}
