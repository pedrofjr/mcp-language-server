package omnipascal_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	otinternal "github.com/isaacphi/mcp-language-server/integrationtests/tests/omnipascal/internal"
	"github.com/isaacphi/mcp-language-server/internal/omnipascal"
	"github.com/isaacphi/mcp-language-server/internal/tools"
)

func TestOmniPascalSmokeTools(t *testing.T) {
	suite := otinternal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 2*time.Minute)
	defer cancel()

	filePath := suite.TargetFile
	line := suite.RefLine
	column := suite.RefColumn
	positionCandidates := buildPositionCandidates(filePath, line, column)
	selection := omnipascal.TextSpan{
		Start: omnipascal.Point{Line: line, Offset: column},
		End:   omnipascal.Point{Line: line, Offset: column},
	}

	t.Run("load_project", func(t *testing.T) {
		if suite.ProjectFile == "" {
			t.Skip("no project file found (.dpr/.dpk/.lpi)")
		}

		result, err := tools.OmniPascalLoadProject(ctx, suite.Client, suite.ProjectFile)
		if err != nil {
			t.Fatalf("loadProject failed: %v", err)
		}
		if strings.TrimSpace(result) == "" {
			t.Fatalf("loadProject returned empty output")
		}
	})

	t.Run("open_change_close", func(t *testing.T) {
		if _, err := tools.OmniPascalOpenFile(ctx, suite.Client, filePath); err != nil {
			t.Fatalf("open failed: %v", err)
		}
		if _, err := tools.OmniPascalChangeFile(ctx, suite.Client, omnipascal.ChangeArgs{
			File:         filePath,
			Line:         line,
			Offset:       column,
			EndLine:      line,
			EndOffset:    column,
			InsertString: "",
		}); err != nil {
			t.Fatalf("change failed: %v", err)
		}
		if _, err := tools.OmniPascalCloseFile(ctx, suite.Client, filePath); err != nil {
			t.Fatalf("close failed: %v", err)
		}
	})

	t.Run("set_config", func(t *testing.T) {
		config := map[string]any{"workspacePaths": suite.WorkspaceDir}
		if v := strings.TrimSpace(os.Getenv("OMNIPASCAL_DELPHI_PATH")); v != "" {
			config["delphiInstallationPath"] = v
		}
		if v := strings.TrimSpace(os.Getenv("OMNIPASCAL_SEARCH_PATH")); v != "" {
			config["searchPath"] = v
		}
		if _, err := tools.OmniPascalSetConfig(ctx, suite.Client, config); err != nil {
			t.Fatalf("set config failed: %v", err)
		}
	})

	t.Run("geterr", func(t *testing.T) {
		result, err := tools.OmniPascalGetDiagnostics(ctx, suite.Client, []string{filePath}, 0, 1000)
		if err != nil {
			t.Fatalf("geterr failed: %v", err)
		}
		if strings.TrimSpace(result) == "" {
			t.Fatalf("geterr returned empty output")
		}
	})

	t.Run("geterr_with_error", func(t *testing.T) {
		baselineResult, err := tools.OmniPascalGetDiagnostics(ctx, suite.Client, []string{filePath}, 0, 3000)
		if err != nil {
			t.Fatalf("failed to capture baseline diagnostics: %v", err)
		}

		originalContent, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read target file: %v", err)
		}
		t.Cleanup(func() {
			if err := os.WriteFile(filePath, originalContent, 0644); err != nil {
				t.Logf("WARNING: failed to restore target file after geterr_with_error: %v", err)
				return
			}
			if err := suite.Client.ReopenFile(suite.Context, filePath); err != nil {
				t.Logf("WARNING: failed to resync file after geterr_with_error restore: %v", err)
			}
		})

		// Introduce a syntax error on line 1 and sync with OmniPascal.
		if _, err := tools.OmniPascalApplyTextEdits(ctx, suite.Client, filePath, []tools.TextEdit{{
			StartLine: 1,
			EndLine:   1,
			NewText:   "INVALID_PASCAL_SYNTAX_ERROR_XYZ",
		}}); err != nil {
			t.Fatalf("inject error edit failed: %v", err)
		}

		result, err := tools.OmniPascalGetDiagnostics(ctx, suite.Client, []string{filePath}, 0, 3000)
		if err != nil {
			t.Fatalf("geterr_with_error failed: %v", err)
		}
		if !strings.Contains(result, "ERROR") {
			t.Fatalf("expected at least one ERROR diagnostic after injecting syntax error, got:\n%s", result)
		}

		if err := os.WriteFile(filePath, originalContent, 0644); err != nil {
			t.Fatalf("failed to restore target file after geterr_with_error: %v", err)
		}
		if err := suite.Client.ReopenFile(ctx, filePath); err != nil {
			t.Fatalf("failed to resync target file after geterr_with_error restore: %v", err)
		}

		restoredResult, err := tools.OmniPascalGetDiagnostics(ctx, suite.Client, []string{filePath}, 300, 5000)
		if err != nil {
			t.Fatalf("failed to fetch diagnostics after restore: %v", err)
		}

		if canonicalDiagnostics(restoredResult) != canonicalDiagnostics(baselineResult) {
			t.Fatalf("diagnostics did not converge after restore\n--- baseline ---\n%s\n--- restored ---\n%s", baselineResult, restoredResult)
		}
	})

	t.Run("completions", func(t *testing.T) {
		result, err := tools.OmniPascalCompletions(ctx, suite.Client, filePath, line, column)
		if err != nil {
			t.Fatalf("completions failed: %v", err)
		}
		if strings.TrimSpace(result) == "" {
			t.Fatalf("completions returned empty output")
		}
	})

	t.Run("definition", func(t *testing.T) {
		result, err := firstSuccessfulPosition(positionCandidates, func(candidate lineColumn) (string, error) {
			opCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			return tools.OmniPascalDefinition(opCtx, suite.Client, filePath, candidate.Line, candidate.Column)
		})
		if err != nil {
			t.Skipf("definition unavailable in sampled positions: %v", err)
		}
		if strings.TrimSpace(result) == "" {
			t.Fatalf("definition returned empty output")
		}
	})

	t.Run("quickinfo", func(t *testing.T) {
		result, err := firstSuccessfulPosition(positionCandidates, func(candidate lineColumn) (string, error) {
			opCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			return tools.OmniPascalQuickInfo(opCtx, suite.Client, filePath, candidate.Line, candidate.Column)
		})
		if err != nil {
			t.Skipf("quickinfo unavailable in sampled positions: %v", err)
		}
		if strings.TrimSpace(result) == "" {
			t.Fatalf("quickinfo returned empty output")
		}
	})

	t.Run("signature_help", func(t *testing.T) {
		result, err := firstSuccessfulPosition(positionCandidates, func(candidate lineColumn) (string, error) {
			opCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			return tools.OmniPascalSignatureHelp(opCtx, suite.Client, filePath, candidate.Line, candidate.Column)
		})
		if err != nil {
			t.Skipf("signature help unavailable in sampled positions: %v", err)
		}
		if strings.TrimSpace(result) == "" {
			t.Fatalf("signature help returned empty output")
		}
	})

	t.Run("get_all_units", func(t *testing.T) {
		result, err := tools.OmniPascalGetAllUnits(ctx, suite.Client, filePath)
		if err != nil {
			t.Fatalf("getAllUnits failed: %v", err)
		}
		if strings.TrimSpace(result) == "" {
			t.Fatalf("getAllUnits returned empty output")
		}
	})

	t.Run("get_possible_uses_sections", func(t *testing.T) {
		result, err := tools.OmniPascalGetPossibleUsesSections(ctx, suite.Client, filePath)
		if err != nil {
			t.Fatalf("getPossibleUsesSections failed: %v", err)
		}
		if strings.TrimSpace(result) == "" {
			t.Fatalf("getPossibleUsesSections returned empty output")
		}
	})

	t.Run("add_uses", func(t *testing.T) {
		sectionsResult, err := tools.OmniPascalGetPossibleUsesSections(ctx, suite.Client, filePath)
		if err != nil {
			t.Fatalf("could not read uses sections: %v", err)
		}
		sections := nonEmptyLines(sectionsResult)
		if len(sections) == 0 {
			t.Skip("no uses section available")
		}

		result, err := tools.OmniPascalAddUses(ctx, suite.Client, filePath, sections[0], "SysUtils", false)
		if err != nil {
			t.Fatalf("addUses failed: %v", err)
		}
		if strings.TrimSpace(result) == "" {
			t.Fatalf("addUses returned empty output")
		}
	})

	t.Run("get_code_actions", func(t *testing.T) {
		result, err := tools.OmniPascalGetCodeActions(ctx, suite.Client, filePath, selection)
		if err != nil {
			t.Fatalf("getCodeActions failed: %v", err)
		}
		if strings.TrimSpace(result) == "" {
			t.Fatalf("getCodeActions returned empty output")
		}
	})

	t.Run("run_code_action", func(t *testing.T) {
		actionsResult, err := tools.OmniPascalGetCodeActions(ctx, suite.Client, filePath, selection)
		if err != nil {
			t.Fatalf("could not read code actions: %v", err)
		}
		identifier := firstCodeActionIdentifier(actionsResult)
		if identifier == "" {
			t.Skip("no code action available")
		}

		result, err := tools.OmniPascalRunCodeAction(ctx, suite.Client, filePath, identifier, selection, true, false)
		if err != nil {
			t.Fatalf("runCodeAction failed: %v", err)
		}
		if strings.TrimSpace(result) == "" {
			t.Fatalf("runCodeAction returned empty output")
		}
	})

	t.Run("edit_file", func(t *testing.T) {
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read target file: %v", err)
		}
		original := string(content)
		lines := strings.Split(original, "\n")
		if len(lines) == 0 {
			t.Skip("target file has no lines")
		}

		const editMarker = "{ test edit }"
		firstLine := strings.TrimSuffix(lines[0], "\r")
		newFirstLine := firstLine + " " + editMarker
		result, err := tools.OmniPascalApplyTextEdits(ctx, suite.Client, filePath, []tools.TextEdit{{
			StartLine: 1,
			EndLine:   1,
			NewText:   newFirstLine,
		}})
		if err != nil {
			t.Fatalf("edit_file failed: %v", err)
		}
		if strings.TrimSpace(result) == "" {
			t.Fatalf("edit_file returned empty output")
		}

		// Verify the edit actually landed on disk.
		diskContent, readErr := os.ReadFile(filePath)
		if readErr != nil {
			t.Fatalf("failed to read file after edit: %v", readErr)
		}
		if !strings.Contains(string(diskContent), editMarker) {
			t.Fatalf("edit_file did not persist to disk: marker %q not found in:\n%s", editMarker, string(diskContent))
		}

		if writeErr := os.WriteFile(filePath, []byte(original), 0644); writeErr != nil {
			t.Fatalf("failed to restore target file: %v", writeErr)
		}
		if reopenErr := suite.Client.ReopenFile(ctx, filePath); reopenErr != nil {
			t.Fatalf("failed to resync target file: %v", reopenErr)
		}
	})

}

func nonEmptyLines(value string) []string {
	parts := strings.Split(value, "\n")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func firstCodeActionIdentifier(actionsOutput string) string {
	for _, line := range nonEmptyLines(actionsOutput) {
		if strings.EqualFold(line, "No code actions available.") {
			return ""
		}
		sep := strings.LastIndex(line, " | ")
		if sep == -1 || sep+3 >= len(line) {
			continue
		}
		identifier := strings.TrimSpace(line[sep+3:])
		if identifier != "" {
			return identifier
		}
	}
	return ""
}

type lineColumn struct {
	Line   int
	Column int
}

func buildPositionCandidates(filePath string, defaultLine, defaultColumn int) []lineColumn {
	// Stable fallbacks for known Delphi symbols in typical forms units.
	candidates := []lineColumn{
		{Line: 15, Column: 20},
		{Line: 7, Column: 30},
		{Line: defaultLine, Column: defaultColumn},
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return candidates
	}

	for _, token := range []string{"SysUtils", "TForm", "class"} {
		if line, column, ok := findTokenPosition(string(content), token); ok {
			candidates = append(candidates, lineColumn{Line: line, Column: column})
		}
	}

	// Deduplicate preserving order.
	seen := map[string]struct{}{}
	result := make([]lineColumn, 0, len(candidates))
	for _, candidate := range candidates {
		key := fmt.Sprintf("%d:%d", candidate.Line, candidate.Column)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, candidate)
	}

	return result
}

func findTokenPosition(content, token string) (int, int, bool) {
	lines := strings.Split(content, "\n")
	for lineIndex, line := range lines {
		column := strings.Index(line, token)
		if column >= 0 {
			return lineIndex + 1, column + 1, true
		}
	}
	return 0, 0, false
}

func firstSuccessfulPosition(candidates []lineColumn, run func(candidate lineColumn) (string, error)) (string, error) {
	var lastErr error
	for _, candidate := range candidates {
		result, err := run(candidate)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no candidate positions available")
	}
	return "", lastErr
}

func canonicalDiagnostics(value string) string {
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	return strings.TrimSpace(normalized)
}
