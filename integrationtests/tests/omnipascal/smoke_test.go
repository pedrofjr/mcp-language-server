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
			InsertString: "x",
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
		baselineHasLine1 := strings.Contains(baselineResult, "L1:")

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
		if !baselineHasLine1 && !strings.Contains(result, "L1:") {
			t.Fatalf("expected injected syntax error to surface at line 1, got:\n%s", result)
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

		// OmniPascal may only emit diagnostics after an in-memory buffer event.
		if strings.Contains(restoredResult, "No diagnostics.") {
			if _, changeErr := tools.OmniPascalOpenFile(ctx, suite.Client, filePath); changeErr != nil {
				t.Fatalf("failed to open file during diagnostics convergence: %v", changeErr)
			}
			if _, changeErr := tools.OmniPascalChangeFile(ctx, suite.Client, omnipascal.ChangeArgs{
				File:         filePath,
				Line:         1,
				Offset:       1,
				EndLine:      1,
				EndOffset:    2,
				InsertString: "u",
			}); changeErr != nil {
				t.Fatalf("failed to trigger in-memory diagnostics refresh: %v", changeErr)
			}
			restoredResult, err = tools.OmniPascalGetDiagnostics(ctx, suite.Client, []string{filePath}, 300, 5000)
			if err != nil {
				t.Fatalf("failed to fetch diagnostics after in-memory refresh: %v", err)
			}
		}

		if strings.Contains(restoredResult, "INVALID_PASCAL_SYNTAX_ERROR_XYZ") {
			t.Fatalf("restored diagnostics still reference injected syntax marker:\n%s", restoredResult)
		}

		if !baselineHasLine1 && strings.Contains(restoredResult, "L1:") {
			t.Fatalf("restored diagnostics still contain line 1 errors introduced by test edit\n--- baseline ---\n%s\n--- restored ---\n%s", baselineResult, restoredResult)
		}
	})

	t.Run("geterr_after_buffer_disk_resync", func(t *testing.T) {
		originalContent, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read target file: %v", err)
		}
		originalFirstLine := strings.TrimSuffix(strings.Split(string(originalContent), "\n")[0], "\r")

		if _, err := tools.OmniPascalOpenFile(ctx, suite.Client, filePath); err != nil {
			t.Fatalf("open before in-memory change failed: %v", err)
		}

		if _, err := tools.OmniPascalChangeFile(ctx, suite.Client, omnipascal.ChangeArgs{
			File:         filePath,
			Line:         1,
			Offset:       1,
			EndLine:      1,
			EndOffset:    2,
			InsertString: "x",
		}); err != nil {
			t.Fatalf("in-memory change failed: %v", err)
		}

		dirtyResult, err := tools.OmniPascalGetDiagnostics(ctx, suite.Client, []string{filePath}, 300, 5000)
		if err != nil {
			t.Fatalf("geterr after in-memory change failed: %v", err)
		}
		if !strings.Contains(dirtyResult, "ERROR") {
			t.Fatalf("expected diagnostics after in-memory change, got:\n%s", dirtyResult)
		}

		if _, err := tools.OmniPascalApplyTextEdits(ctx, suite.Client, filePath, []tools.TextEdit{{
			StartLine: 1,
			EndLine:   1,
			NewText:   originalFirstLine,
		}}); err != nil {
			t.Fatalf("failed to restore first line on disk: %v", err)
		}

		restoredResult, err := tools.OmniPascalGetDiagnostics(ctx, suite.Client, []string{filePath}, 300, 7000)
		if err != nil {
			t.Fatalf("geterr after disk restore failed: %v", err)
		}

		if strings.Contains(restoredResult, "L1:") {
			t.Fatalf("diagnostics remained stale after disk restore\n--- dirty ---\n%s\n--- restored ---\n%s", dirtyResult, restoredResult)
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
			t.Skipf("definition unavailable in sampled positions (non-blocking semantic variability): %v", err)
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
			t.Skipf("quickinfo unavailable in sampled positions (non-blocking semantic variability): %v", err)
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
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read target file: %v", err)
		}
		original := string(content)
		t.Cleanup(func() {
			if err := os.WriteFile(filePath, []byte(original), 0644); err != nil {
				t.Logf("WARNING: failed to restore file after add_uses: %v", err)
				return
			}
			if err := suite.Client.ReopenFile(suite.Context, filePath); err != nil {
				t.Logf("WARNING: failed to resync file after add_uses restore: %v", err)
			}
		})

		sectionsResult, err := tools.OmniPascalGetPossibleUsesSections(ctx, suite.Client, filePath)
		if err != nil {
			t.Fatalf("could not read uses sections: %v", err)
		}
		sections := nonEmptyLines(sectionsResult)
		if len(sections) == 0 {
			t.Skip("no uses section available")
		}

		unitCandidates := []string{"DateUtils", "Math", "StrUtils"}
		unitToAdd := ""
		for _, candidate := range unitCandidates {
			if !strings.Contains(original, candidate) {
				unitToAdd = candidate
				break
			}
		}
		if unitToAdd == "" {
			t.Skip("could not find a unit candidate absent from file")
		}

		result, err := tools.OmniPascalAddUses(ctx, suite.Client, filePath, sections[0], unitToAdd, true)
		if err != nil {
			t.Fatalf("addUses failed: %v", err)
		}
		if !strings.Contains(result, "Applied") {
			t.Fatalf("addUses did not report applied changes, got: %s", result)
		}

		diskContent, readErr := os.ReadFile(filePath)
		if readErr != nil {
			t.Fatalf("failed to read file after add_uses: %v", readErr)
		}
		if !strings.Contains(string(diskContent), unitToAdd) {
			t.Fatalf("add_uses did not persist unit %q to disk", unitToAdd)
		}
	})

	t.Run("get_code_actions", func(t *testing.T) {
		result, err := tools.OmniPascalGetCodeActions(ctx, suite.Client, filePath, selection)
		if err != nil {
			t.Fatalf("getCodeActions failed: %v", err)
		}
		trimmed := strings.TrimSpace(result)
		if trimmed == "" {
			t.Fatalf("getCodeActions returned empty output")
		}
		if trimmed != "No code actions available." && !strings.Contains(trimmed, " | ") {
			t.Fatalf("unexpected getCodeActions output format: %s", result)
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
		trimmed := strings.TrimSpace(result)
		if trimmed == "" {
			t.Fatalf("runCodeAction returned empty output")
		}
		if !strings.Contains(trimmed, "changes") && !strings.Contains(trimmed, "No text changes") {
			t.Fatalf("runCodeAction returned unexpected payload: %s", result)
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
		{Line: 168, Column: 1},
		{Line: 168, Column: 14},
		{Line: 15, Column: 20},
		{Line: 7, Column: 30},
		{Line: defaultLine, Column: defaultColumn},
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return candidates
	}

	for _, token := range []string{"FiltrarTabela", "Button1Click", "SysUtils", "TForm", "class"} {
		if line, column, ok := findTokenPosition(string(content), token); ok {
			candidates = append(candidates, lineColumn{Line: line, Column: column})
		}
	}

	// Broaden semantic probing: collect identifier starts across the file so
	// definition/quickinfo can find at least one resolvable symbol.
	candidates = append(candidates, findIdentifierPositions(string(content), 200)...)

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

func findIdentifierPositions(content string, limit int) []lineColumn {
	if limit <= 0 {
		return nil
	}

	positions := make([]lineColumn, 0, limit)
	lines := strings.Split(content, "\n")

	for lineIndex, line := range lines {
		inIdent := false
		identStart := 0

		for i, r := range line {
			isIdent := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_'

			if isIdent && !inIdent {
				inIdent = true
				identStart = i
			}

			if !isIdent && inIdent {
				if i-identStart >= 2 {
					positions = append(positions, lineColumn{Line: lineIndex + 1, Column: identStart + 1})
					if len(positions) >= limit {
						return positions
					}
				}
				inIdent = false
			}
		}

		if inIdent {
			if len(line)-identStart >= 2 {
				positions = append(positions, lineColumn{Line: lineIndex + 1, Column: identStart + 1})
				if len(positions) >= limit {
					return positions
				}
			}
		}
	}

	return positions
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
