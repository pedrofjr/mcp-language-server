package tools

import (
	"fmt"
	"strings"
)

// BuildQuery monta uma query tree-sitter para o tipo de nó e símbolo informados.
func BuildQuery(nodeType, symbol string) string {
	if symbol == "" {
		return fmt.Sprintf("(%s) @capture", nodeType)
	}
	return fmt.Sprintf(`(%s name: (identifier) @name (#eq? @name %q)) @capture`, nodeType, symbol)
}

// AdaptQuery ajusta uma query base para um dialeto Pascal específico.
func AdaptQuery(base, dialect string) string {
	replacements := map[string]map[string]string{
		"pascal": {
			"procedure_declaration": "procedure_definition",
			"function_declaration":  "function_definition",
		},
		"fpc": {
			"procedure_declaration": "proc_decl",
			"function_declaration":  "func_decl",
			"class_type":            "class_decl",
		},
		"bcb": {
			"procedure_declaration": "procedure_declaration",
			"function_declaration":  "function_declaration",
			"class_type":            "class_type",
		},
		"delphi6": {},
	}

	result := base
	if replacementsForDialect, ok := replacements[strings.ToLower(dialect)]; ok {
		for from, to := range replacementsForDialect {
			result = strings.ReplaceAll(result, from, to)
		}
	}
	return result
}
