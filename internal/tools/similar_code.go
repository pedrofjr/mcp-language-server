package tools

import "strings"

// SimilarBlock representa um bloco com tokens semanticamente semelhantes ao trecho consultado.
type SimilarBlock struct {
	StartLine int
	EndLine   int
	Score     float64
	Snippet   string
}

var delphiStopTokens = map[string]bool{
	"begin": true, "end": true, "var": true, "procedure": true,
	"function": true, "type": true, "const": true, "uses": true,
	"implementation": true, "interface": true, "unit": true, "program": true,
}

const windowSize = 10

// FindSimilarCode procura janelas de código semelhantes usando similaridade de Jaccard.
func FindSimilarCode(src, query string, threshold float64) []SimilarBlock {
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return nil
	}

	lines := strings.Split(src, "\n")
	if len(lines) < windowSize {
		return evaluateWindow(lines, queryTokens, threshold, 0)
	}

	var blocks []SimilarBlock
	for start := 0; start+windowSize <= len(lines); start++ {
		blocks = append(blocks, evaluateWindow(lines[start:start+windowSize], queryTokens, threshold, start)...)
	}
	return blocks
}

func evaluateWindow(lines []string, queryTokens []string, threshold float64, start int) []SimilarBlock {
	window := strings.Join(lines, "\n")
	windowTokens := tokenize(window)
	score := jaccardSimilarity(queryTokens, windowTokens)
	if score < threshold {
		return nil
	}

	return []SimilarBlock{{
		StartLine: start,
		EndLine:   start + len(lines) - 1,
		Score:     score,
		Snippet:   window,
	}}
}

func tokenize(text string) []string {
	var builder strings.Builder
	for _, ch := range text {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			builder.WriteRune(ch)
			continue
		}
		builder.WriteRune(' ')
	}

	parts := strings.Fields(builder.String())
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		lower := strings.ToLower(part)
		if delphiStopTokens[lower] || len(lower) <= 1 {
			continue
		}
		tokens = append(tokens, lower)
	}
	return tokens
}

func jaccardSimilarity(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}

	setA := make(map[string]struct{}, len(a))
	setB := make(map[string]struct{}, len(b))
	for _, token := range a {
		setA[token] = struct{}{}
	}
	for _, token := range b {
		setB[token] = struct{}{}
	}

	intersection := 0
	for token := range setA {
		if _, ok := setB[token]; ok {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
