package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
)

// NodeContext contem informacoes sobre o no textual em uma posicao.
type NodeContext struct {
	Line    string `json:"line"`
	Token   string `json:"token"`
	Before  string `json:"before"`
	After   string `json:"after"`
	LineNum int    `json:"line_number"`
	Col     int    `json:"column"`
}

// getNodeAtPositionFromSource extrai contexto textual de uma posicao (0-indexed line/col).
func getNodeAtPositionFromSource(src string, line, col int) *NodeContext {
	lines := strings.Split(src, "\n")
	if line < 0 || line >= len(lines) {
		return nil
	}

	lineText := lines[line]
	if col < 0 {
		col = 0
	}
	if col > len(lineText) {
		col = len(lineText)
	}

	start := col
	for start > 0 && isIdentChar(lineText[start-1]) {
		start--
	}

	end := col
	for end < len(lineText) && isIdentChar(lineText[end]) {
		end++
	}

	token := lineText[start:end]
	before := strings.TrimSpace(lineText[:start])
	after := ""
	if end < len(lineText) {
		after = strings.TrimSpace(lineText[end:])
	}

	return &NodeContext{
		Line:    lineText,
		Token:   token,
		Before:  before,
		After:   after,
		LineNum: line + 1,
		Col:     col,
	}
}

func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') || b == '_'
}

// GetNodeAtPosition retorna contexto do no na posicao dada.
func GetNodeAtPosition(ctx context.Context, client *lsp.Client, filePath string, line, col int) (string, error) {
	_ = client
	if err := ctx.Err(); err != nil {
		return "", err
	}

	content, err := readSymbolFileContent(filePath)
	if err != nil {
		return "", fmt.Errorf("erro ao ler arquivo: %v", err)
	}

	nodeCtx := getNodeAtPositionFromSource(content, line-1, col-1)
	if nodeCtx == nil {
		return fmt.Sprintf("Posicao (%d, %d) fora dos limites do arquivo", line, col), nil
	}

	return fmt.Sprintf("Token: %q\nLinha %d: %s\nAntes: %s\nDepois: %s",
		nodeCtx.Token, nodeCtx.LineNum, nodeCtx.Line, nodeCtx.Before, nodeCtx.After), nil
}

var delphiNodeTypes = []string{
	"unit", "program", "library", "package",
	"interface_section", "implementation_section", "initialization_section", "finalization_section",
	"uses_clause", "type_section", "var_section", "const_section",
	"class_declaration", "interface_declaration", "record_declaration",
	"procedure_declaration", "function_declaration", "constructor_declaration", "destructor_declaration",
	"method_declaration", "property_declaration",
	"begin_end_block", "try_block", "case_statement", "if_statement", "for_statement",
	"while_statement", "repeat_until_statement", "with_statement",
	"assignment_statement", "call_statement", "raise_statement",
	"identifier", "qualified_identifier", "string_literal", "numeric_literal", "boolean_literal",
	"binary_expression", "unary_expression", "parenthesized_expression",
	"parameter_list", "argument_list",
	"compiler_directive", "comment",
}

// GetNodeTypes retorna a lista de tipos de nos do Delphi suportados.
func GetNodeTypes(ctx context.Context, client *lsp.Client) (string, error) {
	_ = client
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return strings.Join(delphiNodeTypes, "\n"), nil
}
