package tools

import "strings"

// ComplexityResult descreve a complexidade ciclomática de um símbolo.
type ComplexityResult struct {
	SymbolName string
	Score      int
	Rating     string
}

// AnalyzeComplexity conta a complexidade ciclomática de um símbolo pelo source text.
// Retorna nil se a rotina não for encontrada.
func AnalyzeComplexity(src, symbolName string) *ComplexityResult {
	lines := strings.Split(src, "\n")
	lowerSymbol := strings.ToLower(strings.TrimSpace(symbolName))
	if lowerSymbol == "" {
		return nil
	}

	inRoutine := false
	seenBegin := false
	depth := 0
	score := 1

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		if !inRoutine {
			if !isRoutineHeader(lower, lowerSymbol) {
				continue
			}
			inRoutine = true
			continue
		}

		if strings.Contains(lower, "begin") {
			depth += strings.Count(lower, "begin")
			seenBegin = true
		}

		if hasDecisionPoint(lower) {
			score++
		}

		if strings.Contains(lower, "end") {
			depth -= countStandaloneEnd(lower)
			if seenBegin && depth <= 0 {
				break
			}
		}
	}

	if !inRoutine {
		return nil
	}

	return &ComplexityResult{
		SymbolName: symbolName,
		Score:      score,
		Rating:     complexityRating(score),
	}
}

func isRoutineHeader(line, symbolName string) bool {
	prefixes := []string{"procedure ", "function ", "constructor ", "destructor "}
	for _, prefix := range prefixes {
		if strings.HasPrefix(line, prefix) && strings.Contains(line, symbolName) {
			return true
		}
	}
	return false
}

func hasDecisionPoint(line string) bool {
	for _, prefix := range []string{"if ", "if(", "while ", "while(", "for ", "repeat", "except", "case "} {
		if strings.Contains(line, prefix) {
			return true
		}
	}
	return false
}

func countStandaloneEnd(line string) int {
	count := 0
	for _, token := range strings.Fields(line) {
		clean := strings.Trim(token, ";.")
		if clean == "end" {
			count++
		}
	}
	return count
}

func complexityRating(score int) string {
	switch {
	case score >= 15:
		return "critical"
	case score >= 10:
		return "high"
	case score >= 5:
		return "medium"
	default:
		return "low"
	}
}
