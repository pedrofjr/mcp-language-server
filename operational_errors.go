package main

import (
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// Tokens operacionais compartilhados entre tools (contrato v1.1+ P2).
const (
	OpValidation = "OP_VALIDATION"
	OpToolFailed = "OP_TOOL_FAILED"
)

// OpValidationError retorna erro MCP padronizado para argumentos inválidos.
func OpValidationError(detail string) (*mcp.CallToolResult, error) {
	msg := opErrMsgWithRecovery(OpValidation, detail+" | action: verify required arguments and types, then retry")
	return mcp.NewToolResultError(msg), nil
}

// OpToolFailedError retorna erro MCP padronizado para falha de execução da tool.
func OpToolFailedError(tool, detail, action string) (*mcp.CallToolResult, error) {
	prefix := fmt.Sprintf("%s failed", tool)
	if action != "" {
		prefix += " | action: " + action
	}
	msg := opErrMsgWithRecovery(OpToolFailed, prefix+": "+detail)
	return mcp.NewToolResultError(msg), nil
}

// OpErrorFromParseArg converte erros de parse de argumentos em OP_VALIDATION.
func OpErrorFromParseArg(err error) (*mcp.CallToolResult, error) {
	if err == nil {
		return nil, nil
	}
	return OpValidationError(err.Error())
}

// OpErrorFromMemory classifica erros das Memory Tools em validação vs falha operacional.
func OpErrorFromMemory(tool string, err error) (*mcp.CallToolResult, error) {
	if err == nil {
		return nil, nil
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "obrigatorio"),
		strings.Contains(lower, "duplicado"),
		strings.Contains(lower, "vazio"):
		return OpValidationError(msg)
	case strings.Contains(lower, "nao encontrada"):
		return OpToolFailedError(tool, msg, "verify memory id/title and retry")
	default:
		return OpToolFailedError(tool, msg, "retry or inspect ORACLE_MEMORY_DIR storage")
	}
}

// OpErrorFromDomain envolve erro de domínio genérico (onboarding, symbol edit, etc.).
func OpErrorFromDomain(tool string, err error, action string) (*mcp.CallToolResult, error) {
	if err == nil {
		return nil, nil
	}
	if action == "" {
		action = "review arguments and retry"
	}
	return OpToolFailedError(tool, err.Error(), action)
}

func opErrMsgWithRecovery(token, msg string) string {
	recovery := opRecoveryForToken(token)
	if recovery == "" {
		return opErrMsg(token, msg)
	}
	return opErrMsg(token, msg) + " | recovery: " + recovery
}

// opRecoveryForToken retorna passos guiados de retomada para tokens operacionais conhecidos.
func opRecoveryForToken(token string) string {
	switch token {
	case OpValidation:
		return "1) compare arguments with tools/list schema; 2) fix types/required fields; 3) retry the same tool call"
	case OpToolFailed:
		return "1) inspect the action hint above; 2) reduce scope (single file/symbol); 3) retry; 4) if persistent, restart MCP and LSP"
	case OpLSPUnavailable:
		return "1) confirm --workspace and LSP binary; 2) restart MCP server; 3) wait for LSP ready; 4) retry the tool"
	case OpDefinitionCanceled, OpReferencesCanceled, OpRunQueryCanceled:
		return "1) avoid canceling in-flight requests; 2) retry with a fresh tools/call; 3) narrow file/symbol scope if needed"
	case OpDefinitionDeadline, OpReferencesDeadline, OpRunQueryDeadline:
		return "1) retry on a smaller scope (single file/symbol); 2) check LSP load; 3) increase client deadline only if policy allows"
	default:
		if strings.HasSuffix(token, "_CANCELED") {
			return "1) avoid canceling in-flight requests; 2) retry with a fresh tools/call; 3) narrow file/symbol scope if needed"
		}
		if strings.HasSuffix(token, "_DEADLINE") {
			return "1) retry on a smaller scope (single file/symbol); 2) check LSP load; 3) increase client deadline only if policy allows"
		}
		return ""
	}
}

func extractOperationalToken(text string) string {
	if idx := strings.Index(text, " | "); idx > 0 {
		return strings.TrimSpace(text[:idx])
	}
	return ""
}

// enrichToolResultWithRecovery acrescenta passos de retomada quando ausentes no payload de erro.
func enrichToolResultWithRecovery(result *mcp.CallToolResult) *mcp.CallToolResult {
	if result == nil || !result.IsError || len(result.Content) == 0 {
		return result
	}
	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		return result
	}
	text := textContent.Text
	if strings.Contains(text, "recovery:") {
		return result
	}
	token := extractOperationalToken(text)
	recovery := opRecoveryForToken(token)
	if recovery == "" {
		return result
	}
	textContent.Text = text + " | recovery: " + recovery
	result.Content[0] = textContent
	return result
}
