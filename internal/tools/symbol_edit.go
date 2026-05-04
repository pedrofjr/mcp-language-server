package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
)

const (
	toolReplaceSymbolBody  = "replace_symbol_body"
	toolInsertAfterSymbol  = "insert_after_symbol"
	toolInsertBeforeSymbol = "insert_before_symbol"
)

type symbolBodyRange struct {
	beginLine int // 1-indexed, line with "begin"
	endLine   int // 1-indexed, line with "end;"
}

// findSymbolBodyRange localiza o begin..end de uma rotina Delphi pelo nome qualificado.
// symbolName pode ser "TFoo.Bar" ou simplesmente "Bar".
// Retorna nil se não encontrar.
func findSymbolBodyRange(src, symbolName string) *symbolBodyRange {
	startLine := findSymbolStartLine(src, symbolName)
	if startLine < 0 {
		return nil
	}

	lines := strings.Split(src, "\n")
	depth := 0
	beginFound := false
	beginLine := -1

	for i := startLine - 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(strings.ToLower(lines[i]))
		// Ignorar comentários de linha simples
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "{") {
			continue
		}
		words := strings.Fields(trimmed)
		for _, w := range words {
			w = strings.Trim(w, ";,():")
			switch w {
			case "begin":
				if !beginFound {
					beginFound = true
					beginLine = i + 1 // 1-indexed
				}
				depth++
			case "end":
				if depth > 0 {
					depth--
					if depth == 0 && beginFound {
						return &symbolBodyRange{beginLine: beginLine, endLine: i + 1}
					}
				}
			}
		}
	}
	return nil
}

// findSymbolStartLine retorna a linha (1-indexed) onde a rotina é declarada.
// Retorna -1 se não encontrar.
func findSymbolStartLine(src, symbolName string) int {
	lines := strings.Split(src, "\n")
	simpleName := symbolName
	if idx := strings.LastIndex(symbolName, "."); idx >= 0 {
		simpleName = symbolName[idx+1:]
	}
	lowerSimple := strings.ToLower(simpleName)
	lowerFull := strings.ToLower(symbolName)

	for i, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		if (strings.HasPrefix(lower, "procedure ") ||
			strings.HasPrefix(lower, "function ") ||
			strings.HasPrefix(lower, "constructor ") ||
			strings.HasPrefix(lower, "destructor ")) &&
			(strings.Contains(lower, lowerSimple) || strings.Contains(lower, lowerFull)) {
			return i + 1 // 1-indexed
		}
	}
	return -1
}

// findSymbolEndLine retorna a linha (1-indexed) do "end;" da rotina.
// Retorna -1 se não encontrar.
func findSymbolEndLine(src, symbolName string) int {
	r := findSymbolBodyRange(src, symbolName)
	if r == nil {
		return -1
	}
	return r.endLine
}

// ReplaceSymbolBody lê o arquivo, localiza o corpo do símbolo e substitui pelo newBody.
func ReplaceSymbolBody(ctx context.Context, client *lsp.Client, filePath, symbolName, newBody string) (string, error) {
	normalizedPath, err := normalizeFilePathOrURI(filePath)
	if err != nil {
		return "", fmt.Errorf("invalid file path: %v", err)
	}

	content, err := readSymbolFileContent(normalizedPath)
	if err != nil {
		return "", fmt.Errorf("could not read file: %v", err)
	}

	r := findSymbolBodyRange(content, symbolName)
	if r == nil {
		return "", fmt.Errorf("symbol %q not found or has no begin..end block", symbolName)
	}

	edit := TextEdit{
		StartLine: r.beginLine,
		EndLine:   r.endLine,
		NewText:   newBody,
	}
	return ApplyTextEdits(ctx, client, normalizedPath, []TextEdit{edit})
}

// InsertAfterSymbol insere texto logo após a linha "end;" do símbolo.
func InsertAfterSymbol(ctx context.Context, client *lsp.Client, filePath, symbolName, text string) (string, error) {
	normalizedPath, err := normalizeFilePathOrURI(filePath)
	if err != nil {
		return "", fmt.Errorf("invalid file path: %v", err)
	}

	content, err := readSymbolFileContent(normalizedPath)
	if err != nil {
		return "", fmt.Errorf("could not read file: %v", err)
	}

	endLine := findSymbolEndLine(content, symbolName)
	if endLine < 0 {
		return "", fmt.Errorf("symbol %q not found", symbolName)
	}

	lines := strings.Split(content, "\n")
	insertAt := endLine + 1
	if endLine >= len(lines) {
		insertAt = len(lines)
	}
	edit := TextEdit{
		StartLine: insertAt,
		EndLine:   insertAt,
		NewText:   text + "\n",
	}
	return ApplyTextEdits(ctx, client, normalizedPath, []TextEdit{edit})
}

// InsertBeforeSymbol insere texto logo antes da linha de declaração do símbolo.
func InsertBeforeSymbol(ctx context.Context, client *lsp.Client, filePath, symbolName, text string) (string, error) {
	normalizedPath, err := normalizeFilePathOrURI(filePath)
	if err != nil {
		return "", fmt.Errorf("invalid file path: %v", err)
	}

	content, err := readSymbolFileContent(normalizedPath)
	if err != nil {
		return "", fmt.Errorf("could not read file: %v", err)
	}

	startLine := findSymbolStartLine(content, symbolName)
	if startLine < 0 {
		return "", fmt.Errorf("symbol %q not found", symbolName)
	}

	edit := TextEdit{
		StartLine: startLine,
		EndLine:   startLine,
		NewText:   text + "\n" + symbolLineAt(content, startLine),
	}
	return ApplyTextEdits(ctx, client, normalizedPath, []TextEdit{edit})
}

// readSymbolFileContent lê o conteúdo de um arquivo local.
func readSymbolFileContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func symbolLineAt(src string, line int) string {
	lines := strings.Split(src, "\n")
	if line < 1 || line > len(lines) {
		return ""
	}
	return lines[line-1]
}

// SafeDeleteSymbol remove um simbolo se nao tiver referencias externas (ou forcadamente).
// Parametros: filePath, symbolName, force (se true, remove mesmo com referencias).
func SafeDeleteSymbol(ctx context.Context, client *lsp.Client, filePath, symbolName string, force bool) (string, error) {
	normalizedPath, err := normalizeFilePathOrURI(filePath)
	if err != nil {
		return "", fmt.Errorf("caminho invalido: %v", err)
	}

	content, err := readSymbolFileContent(normalizedPath)
	if err != nil {
		return "", fmt.Errorf("erro ao ler arquivo: %v", err)
	}

	startLine := findSymbolStartLine(content, symbolName)
	if startLine < 0 {
		return "", fmt.Errorf("simbolo %q nao encontrado em %s", symbolName, filePath)
	}

	if !force {
		lowerSymbol := strings.ToLower(symbolName)
		if idx := strings.LastIndex(lowerSymbol, "."); idx >= 0 {
			lowerSymbol = lowerSymbol[idx+1:]
		}

		count := 0
		for lineIndex, line := range strings.Split(content, "\n") {
			if lineIndex+1 == startLine {
				continue
			}
			if strings.Contains(strings.ToLower(line), lowerSymbol) {
				count++
			}
		}

		if count > 0 {
			return "", fmt.Errorf("simbolo %q tem %d referencias no arquivo. Use force=true para forcar a remocao", symbolName, count)
		}
	}

	r := findSymbolBodyRange(content, symbolName)
	var edit TextEdit
	if r != nil {
		edit = TextEdit{StartLine: startLine, EndLine: r.endLine, NewText: ""}
	} else {
		edit = TextEdit{StartLine: startLine, EndLine: startLine, NewText: ""}
	}

	return ApplyTextEdits(ctx, client, normalizedPath, []TextEdit{edit})
}
