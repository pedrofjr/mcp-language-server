package tools

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

func TestReplaceSymbolBody_FindsBeginEnd(t *testing.T) {
	src := `unit Foo;
implementation

procedure TFoo.Bar;
begin
  Writeln('old body');
end;

end.`
	result := findSymbolBodyRange(src, "TFoo.Bar")
	if result == nil {
		t.Fatal("findSymbolBodyRange deve retornar range para TFoo.Bar")
	}
	if result.beginLine < 0 || result.endLine < 0 {
		t.Errorf("range invalido: beginLine=%d endLine=%d", result.beginLine, result.endLine)
	}
}

func TestInsertAfterSymbol_FindsInsertionPoint(t *testing.T) {
	src := `unit Foo;
implementation

procedure TFoo.Alpha;
begin
  Writeln('alpha');
end;

end.`
	line := findSymbolEndLine(src, "TFoo.Alpha")
	if line < 0 {
		t.Fatal("findSymbolEndLine deve retornar linha positiva para TFoo.Alpha")
	}
}

func TestInsertBeforeSymbol_FindsInsertionPoint(t *testing.T) {
	src := `unit Foo;
implementation

procedure TFoo.Beta;
begin
  Writeln('beta');
end;

end.`
	line := findSymbolStartLine(src, "TFoo.Beta")
	if line < 0 {
		t.Fatal("findSymbolStartLine deve retornar linha positiva para TFoo.Beta")
	}
}

func TestReplaceSymbolBody_FindsInnerBodyRange_Simple(t *testing.T) {
	src := `unit Foo;
implementation

procedure TFoo.Bar;
begin
  Writeln('old body');
end;

end.`

	r := findSymbolInnerBodyRange(src, "TFoo.Bar")
	if r == nil {
		t.Fatal("findSymbolInnerBodyRange deve retornar range para TFoo.Bar")
	}
	if r.beginLine != r.endLine {
		t.Fatalf("range interno simples deve cobrir uma unica linha: beginLine=%d endLine=%d", r.beginLine, r.endLine)
	}
	lines := strings.Split(src, "\n")
	if r.beginLine < 1 || r.beginLine > len(lines) {
		t.Fatalf("beginLine fora do limite: beginLine=%d total=%d", r.beginLine, len(lines))
	}
	if strings.TrimSpace(lines[r.beginLine-1]) != "Writeln('old body');" {
		t.Fatalf("linha interna esperada logo apos begin: atual=%q", strings.TrimSpace(lines[r.beginLine-1]))
	}
	if strings.TrimSpace(lines[r.beginLine-2]) != "begin" {
		t.Fatalf("linha anterior ao range interno deve ser begin: atual=%q", strings.TrimSpace(lines[r.beginLine-2]))
	}
	if strings.TrimSpace(lines[r.endLine]) != "end;" {
		t.Fatalf("linha posterior ao range interno deve ser end;: atual=%q", strings.TrimSpace(lines[r.endLine]))
	}
}

func TestReplaceSymbolBody_FindsInnerBodyRange_NestedBeginEnd(t *testing.T) {
	src := `unit Foo;
implementation

procedure TFoo.Baz;
begin
  if True then
  begin
    Writeln('inner');
  end;
  Writeln('outer');
end;

end.`

	r := findSymbolInnerBodyRange(src, "TFoo.Baz")
	if r == nil {
		t.Fatal("findSymbolInnerBodyRange deve retornar range para TFoo.Baz")
	}
	lines := strings.Split(src, "\n")
	if r.beginLine < 1 || r.beginLine > len(lines) {
		t.Fatalf("beginLine fora do limite: beginLine=%d total=%d", r.beginLine, len(lines))
	}
	if r.endLine < 1 || r.endLine > len(lines) {
		t.Fatalf("endLine fora do limite: endLine=%d total=%d", r.endLine, len(lines))
	}
	if strings.TrimSpace(lines[r.beginLine-1]) != "if True then" {
		t.Fatalf("range interno deve iniciar apos begin externo: atual=%q", strings.TrimSpace(lines[r.beginLine-1]))
	}
	if strings.TrimSpace(lines[r.endLine-1]) != "Writeln('outer');" {
		t.Fatalf("range interno deve terminar antes do end externo: atual=%q", strings.TrimSpace(lines[r.endLine-1]))
	}
	if strings.TrimSpace(lines[r.beginLine-2]) != "begin" {
		t.Fatalf("linha anterior ao range interno deve ser begin externo: atual=%q", strings.TrimSpace(lines[r.beginLine-2]))
	}
	if strings.TrimSpace(lines[r.endLine]) != "end;" {
		t.Fatalf("linha posterior ao range interno deve ser end externo: atual=%q", strings.TrimSpace(lines[r.endLine]))
	}
}

func TestReplaceSymbolBody_FindsInnerBodyRange_SymbolNotFound(t *testing.T) {
	src := `unit Foo;
implementation

procedure TFoo.Existente;
begin
  Writeln('ok');
end;

end.`

	r := findSymbolInnerBodyRange(src, "TFoo.Inexistente")
	if r != nil {
		t.Fatalf("simbolo inexistente deve retornar nil: beginLine=%d endLine=%d", r.beginLine, r.endLine)
	}
}

func TestToolsRegistration_SymbolEditTools(t *testing.T) {
	// Verifica que os nomes das tools estão definidos como constantes
	names := []string{toolReplaceSymbolBody, toolInsertAfterSymbol, toolInsertBeforeSymbol}
	for _, n := range names {
		if strings.TrimSpace(n) == "" {
			t.Errorf("constante de tool vazia: %q", n)
		}
	}
}

func TestReplaceSymbolBody_AllowsInnerBodyPayloadAndPreservesDelimiters(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "foo.pas")
	original := "procedure TFoo.Bar;\n" +
		"begin\n" +
		"  Writeln('old body');\n" +
		"end;\n"
	if err := os.WriteFile(filePath, []byte(original), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	t.Setenv(editFileFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessEditFileFakeLSP")
	if err != nil {
		t.Fatalf("failed to start fake LSP: %v", err)
	}
	defer func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		t.Fatalf("failed to initialize fake LSP client: %v", err)
	}

	if _, err := ReplaceSymbolBody(ctx, client, filePath, "TFoo.Bar", "  Writeln('new body');"); err != nil {
		t.Fatalf("expected ReplaceSymbolBody to allow inner-body payload without begin/end, got error: %v", err)
	}

	updated, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}

	updatedText := string(updated)
	if !strings.Contains(updatedText, "begin") {
		t.Fatalf("expected routine delimiters to preserve begin in final file, got: %q", updatedText)
	}
	if !strings.Contains(updatedText, "end;") {
		t.Fatalf("expected routine delimiters to preserve end; in final file, got: %q", updatedText)
	}
	if !strings.Contains(updatedText, "Writeln('new body');") {
		t.Fatalf("expected inner body to be replaced with new statement, got: %q", updatedText)
	}
}

func TestReplaceSymbolBody_RollsBackOriginalContentOnApplyFailureAfterPartialMutation(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "foo.pas")
	originalContent := "unit Foo;\n" +
		"implementation\n\n" +
		"procedure TFoo.Bar;\n" +
		"begin\n" +
		"  Writeln('old');\n" +
		"end;\n\n" +
		"end.\n"
	if err := os.WriteFile(filePath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	originalApplyHook := applySymbolBodyTextEdits
	applySymbolBodyTextEdits = func(_ context.Context, _ *lsp.Client, targetFilePath string, _ []TextEdit) (string, error) {
		mutatedContent := "unit Foo;\n" +
			"implementation\n\n" +
			"procedure TFoo.Bar;\n" +
			"begin\n" +
			"  Writeln('mutated');\n" +
			"end;\n\n" +
			"end.\n"
		if err := os.WriteFile(targetFilePath, []byte(mutatedContent), 0o644); err != nil {
			return "", err
		}
		return "", fmt.Errorf("simulated apply failure")
	}
	t.Cleanup(func() {
		applySymbolBodyTextEdits = originalApplyHook
	})

	_, err := ReplaceSymbolBody(context.Background(), nil, filePath, "TFoo.Bar", "  Writeln('new');")
	if err == nil {
		t.Fatal("expected ReplaceSymbolBody to fail when apply hook returns error")
	}

	finalBytes, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("failed to read file after ReplaceSymbolBody failure: %v", readErr)
	}
	if string(finalBytes) != originalContent {
		t.Fatalf("expected local rollback to restore exact original content after apply failure; expected=%q got=%q", originalContent, string(finalBytes))
	}
}

func TestInsertAfterSymbol_RollsBackOriginalContentOnApplyFailureAfterPartialMutation(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "foo.pas")
	originalContent := "unit Foo;\n" +
		"implementation\n\n" +
		"procedure TFoo.Bar;\n" +
		"begin\n" +
		"  Writeln('old');\n" +
		"end;\n\n" +
		"end.\n"
	if err := os.WriteFile(filePath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	originalApplyHook := applySymbolBodyTextEdits
	applySymbolBodyTextEdits = func(_ context.Context, _ *lsp.Client, targetFilePath string, _ []TextEdit) (string, error) {
		mutatedContent := "unit Foo;\n" +
			"implementation\n\n" +
			"procedure TFoo.Bar;\n" +
			"begin\n" +
			"  Writeln('old');\n" +
			"end;\n" +
			"procedure TFoo.Inserted; begin end;\n\n" +
			"end.\n"
		if err := os.WriteFile(targetFilePath, []byte(mutatedContent), 0o644); err != nil {
			return "", err
		}
		return "", fmt.Errorf("simulated apply failure")
	}
	t.Cleanup(func() {
		applySymbolBodyTextEdits = originalApplyHook
	})

	_, err := InsertAfterSymbol(context.Background(), nil, filePath, "TFoo.Bar", "procedure TFoo.Inserted; begin end;")
	if err == nil {
		t.Fatal("expected InsertAfterSymbol to fail when apply hook returns error")
	}
	if !strings.Contains(err.Error(), "simulated apply failure") {
		t.Fatalf("expected InsertAfterSymbol error to contain simulated apply failure, got: %v", err)
	}

	finalBytes, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("failed to read file after InsertAfterSymbol failure: %v", readErr)
	}
	if string(finalBytes) != originalContent {
		t.Fatalf("expected local rollback to restore exact original content after apply failure; expected=%q got=%q", originalContent, string(finalBytes))
	}
}

func TestInsertBeforeSymbol_RollsBackOriginalContentOnApplyFailureAfterPartialMutation(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "foo.pas")
	originalContent := "unit Foo;\n" +
		"implementation\n\n" +
		"procedure TFoo.Bar;\n" +
		"begin\n" +
		"  Writeln('old');\n" +
		"end;\n\n" +
		"end.\n"
	if err := os.WriteFile(filePath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	originalApplyHook := applySymbolBodyTextEdits
	applySymbolBodyTextEdits = func(_ context.Context, _ *lsp.Client, targetFilePath string, _ []TextEdit) (string, error) {
		mutatedContent := "unit Foo;\n" +
			"implementation\n\n" +
			"procedure TFoo.Inserted; begin end;\n" +
			"procedure TFoo.Bar;\n" +
			"begin\n" +
			"  Writeln('old');\n" +
			"end;\n\n" +
			"end.\n"
		if err := os.WriteFile(targetFilePath, []byte(mutatedContent), 0o644); err != nil {
			return "", err
		}
		return "", fmt.Errorf("simulated apply failure")
	}
	t.Cleanup(func() {
		applySymbolBodyTextEdits = originalApplyHook
	})

	_, err := InsertBeforeSymbol(context.Background(), nil, filePath, "TFoo.Bar", "procedure TFoo.Inserted; begin end;")
	if err == nil {
		t.Fatal("expected InsertBeforeSymbol to fail when apply hook returns error")
	}
	if !strings.Contains(err.Error(), "simulated apply failure") {
		t.Fatalf("expected InsertBeforeSymbol error to contain simulated apply failure, got: %v", err)
	}

	finalBytes, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("failed to read file after InsertBeforeSymbol failure: %v", readErr)
	}
	if string(finalBytes) != originalContent {
		t.Fatalf("expected local rollback to restore exact original content after apply failure; expected=%q got=%q", originalContent, string(finalBytes))
	}
}

func TestSafeDeleteSymbol_RollsBackOriginalContentOnApplyFailureAfterPartialMutation(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "foo.pas")
	originalContent := "unit Foo;\n" +
		"implementation\n\n" +
		"procedure TFoo.Bar;\n" +
		"begin\n" +
		"  Writeln('old');\n" +
		"end;\n\n" +
		"end.\n"
	if err := os.WriteFile(filePath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	originalApplyHook := applySymbolBodyTextEdits
	applySymbolBodyTextEdits = func(_ context.Context, _ *lsp.Client, targetFilePath string, _ []TextEdit) (string, error) {
		mutatedContent := "unit Foo;\n" +
			"implementation\n\n" +
			"end.\n"
		if err := os.WriteFile(targetFilePath, []byte(mutatedContent), 0o644); err != nil {
			return "", err
		}
		return "", fmt.Errorf("simulated apply failure")
	}
	t.Cleanup(func() {
		applySymbolBodyTextEdits = originalApplyHook
	})

	_, err := SafeDeleteSymbol(context.Background(), nil, filePath, "TFoo.Bar", true)
	if err == nil {
		t.Fatal("expected SafeDeleteSymbol to fail when apply hook returns error")
	}
	if !strings.Contains(err.Error(), "simulated apply failure") {
		t.Fatalf("expected SafeDeleteSymbol error to contain simulated apply failure, got: %v", err)
	}

	finalBytes, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("failed to read file after SafeDeleteSymbol failure: %v", readErr)
	}
	if string(finalBytes) != originalContent {
		t.Fatalf("expected local rollback to restore exact original content after apply failure; expected=%q got=%q", originalContent, string(finalBytes))
	}
}

func TestSafeDeleteSymbol_BlocksWhenCrossFileReferenceExists(t *testing.T) {
	workspaceDir := t.TempDir()
	pathUnitA := filepath.Join(workspaceDir, "UnitA.pas")
	pathUnitB := filepath.Join(workspaceDir, "UnitB.pas")

	originalUnitA := "unit UnitA;\n" +
		"implementation\n\n" +
		"procedure TFoo.Bar;\n" +
		"begin\n" +
		"end;\n\n" +
		"end.\n"
	unitB := "unit UnitB;\n" +
		"implementation\n\n" +
		"procedure UseBar;\n" +
		"begin\n" +
		"  TFoo.Bar;\n" +
		"end;\n\n" +
		"end.\n"

	if err := os.WriteFile(pathUnitA, []byte(originalUnitA), 0o644); err != nil {
		t.Fatalf("failed to write UnitA fixture: %v", err)
	}
	if err := os.WriteFile(pathUnitB, []byte(unitB), 0o644); err != nil {
		t.Fatalf("failed to write UnitB fixture: %v", err)
	}

	originalApplyHook := applySymbolBodyTextEdits
	applySymbolBodyTextEdits = func(_ context.Context, _ *lsp.Client, _ string, _ []TextEdit) (string, error) {
		return "", nil
	}
	t.Cleanup(func() {
		applySymbolBodyTextEdits = originalApplyHook
	})

	_, err := SafeDeleteSymbol(context.Background(), nil, pathUnitA, "TFoo.Bar", false)
	if err == nil {
		t.Fatal("expected SafeDeleteSymbol to block deletion when cross-file textual reference exists")
	}

	errMsg := strings.ToLower(err.Error())
	if !strings.Contains(errMsg, "referenc") {
		t.Fatalf("expected blocking error to mention references, got: %v", err)
	}

	finalUnitA, readErr := os.ReadFile(pathUnitA)
	if readErr != nil {
		t.Fatalf("failed to read UnitA after SafeDeleteSymbol: %v", readErr)
	}
	if string(finalUnitA) != originalUnitA {
		t.Fatalf("expected UnitA to remain unchanged when deletion is blocked; expected=%q got=%q", originalUnitA, string(finalUnitA))
	}
}

func TestSafeDeleteSymbol_BlocksWhenSemanticCrossFileReferenceExists(t *testing.T) {
	workspaceDir := t.TempDir()
	pathUnitA := filepath.Join(workspaceDir, "UnitA.pas")
	pathUnitB := filepath.Join(workspaceDir, "UnitB.pas")

	originalUnitA := "unit UnitA;\n" +
		"implementation\n\n" +
		"procedure TFoo.Bar;\n" +
		"begin\n" +
		"end;\n\n" +
		"end.\n"
	unitB := "unit UnitB;\n" +
		"implementation\n\n" +
		"procedure UseBar;\n" +
		"begin\n" +
		"  TFoo.Bar;\n" +
		"end;\n\n" +
		"end.\n"

	if err := os.WriteFile(pathUnitA, []byte(originalUnitA), 0o644); err != nil {
		t.Fatalf("failed to write UnitA fixture: %v", err)
	}
	if err := os.WriteFile(pathUnitB, []byte(unitB), 0o644); err != nil {
		t.Fatalf("failed to write UnitB fixture: %v", err)
	}

	originalResolveHook := resolveSymbolReferencesForDelete
	resolveSymbolReferencesForDelete = func(_ context.Context, _ *lsp.Client, _ string, _ string) ([]protocol.Location, error) {
		uri := protocol.DocumentUri("file:///" + filepath.ToSlash(pathUnitB))
		return []protocol.Location{{
			URI: uri,
			Range: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 2},
				End:   protocol.Position{Line: 5, Character: 10},
			},
		}}, nil
	}
	t.Cleanup(func() {
		resolveSymbolReferencesForDelete = originalResolveHook
	})

	originalApplyHook := applySymbolBodyTextEdits
	applySymbolBodyTextEdits = func(_ context.Context, _ *lsp.Client, _ string, _ []TextEdit) (string, error) {
		return "", nil
	}
	t.Cleanup(func() {
		applySymbolBodyTextEdits = originalApplyHook
	})

	client := &lsp.Client{}
	_, err := SafeDeleteSymbol(context.Background(), client, pathUnitA, "TFoo.Bar", false)
	if err == nil {
		t.Fatal("expected SafeDeleteSymbol to block deletion when semantic cross-file references exist")
	}

	errMsg := strings.ToLower(err.Error())
	if !strings.Contains(errMsg, "cross") && !strings.Contains(errMsg, "referenc") {
		t.Fatalf("expected blocking error to mention semantic cross-file references, got: %v", err)
	}

	finalUnitA, readErr := os.ReadFile(pathUnitA)
	if readErr != nil {
		t.Fatalf("failed to read UnitA after SafeDeleteSymbol: %v", readErr)
	}
	if string(finalUnitA) != originalUnitA {
		t.Fatalf("expected UnitA to remain unchanged when semantic cross-file deletion is blocked; expected=%q got=%q", originalUnitA, string(finalUnitA))
	}
}

func TestSymbolEditApplyWithRollback_RestoresOriginalOnFailure(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "foo.pas")
	originalContent := "unit Foo;\n" +
		"interface\n\n" +
		"implementation\n\n" +
		"end.\n"
	if err := os.WriteFile(filePath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	edits := []TextEdit{{
		StartLine: 1,
		EndLine:   1,
		NewText:   "unit Changed;",
	}}

	_, err := applySymbolEditWithRollback(
		context.Background(),
		nil,
		filePath,
		edits,
		originalContent,
		func(_ context.Context, _ *lsp.Client, targetFilePath string, _ []TextEdit) (string, error) {
			mutatedContent := "unit Mutated;\n" +
				"interface\n\n" +
				"implementation\n\n" +
				"end.\n"
			if writeErr := os.WriteFile(targetFilePath, []byte(mutatedContent), 0o644); writeErr != nil {
				return "", writeErr
			}
			return "", fmt.Errorf("simulated apply failure")
		},
	)
	if err == nil {
		t.Fatal("expected helper to fail when apply hook returns error")
	}

	finalBytes, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("failed to read file after helper failure: %v", readErr)
	}
	if string(finalBytes) != originalContent {
		t.Fatalf("expected helper to restore exact original content after apply failure; expected=%q got=%q", originalContent, string(finalBytes))
	}
}

func TestReplaceSymbolBody_PreconditionSymbolNotFound_DoesNotMutateFile(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "foo.pas")
	originalContent := "unit Foo;\n" +
		"implementation\n\n" +
		"procedure TFoo.Existente;\n" +
		"begin\n" +
		"  Writeln('ok');\n" +
		"end;\n\n" +
		"end.\n"
	if err := os.WriteFile(filePath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	_, err := ReplaceSymbolBody(context.Background(), nil, filePath, "TFoo.Inexistente", "  Writeln('x');")
	if err == nil {
		t.Fatal("expected ReplaceSymbolBody to return error when symbol not found")
	}

	finalBytes, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("failed to read file after ReplaceSymbolBody: %v", readErr)
	}
	if string(finalBytes) != originalContent {
		t.Fatalf("expected file to remain unchanged when symbol not found; expected=%q got=%q", originalContent, string(finalBytes))
	}
}

func TestSafeDeleteSymbol_PreconditionSymbolNotFound_DoesNotMutateFile(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "foo.pas")
	originalContent := "unit Foo;\n" +
		"implementation\n\n" +
		"procedure TFoo.Existente;\n" +
		"begin\n" +
		"  Writeln('ok');\n" +
		"end;\n\n" +
		"end.\n"
	if err := os.WriteFile(filePath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	_, err := SafeDeleteSymbol(context.Background(), nil, filePath, "TFoo.Inexistente", false)
	if err == nil {
		t.Fatal("expected SafeDeleteSymbol to return error when symbol not found")
	}

	finalBytes, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("failed to read file after SafeDeleteSymbol: %v", readErr)
	}
	if string(finalBytes) != originalContent {
		t.Fatalf("expected file to remain unchanged when symbol not found; expected=%q got=%q", originalContent, string(finalBytes))
	}
}

func TestSymbolEditApplyWithRollback_ReturnsComposedErrorWhenApplyAndRollbackFail(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "foo.pas")
	originalContent := "unit Foo;\nimplementation\n\nend.\n"
	if err := os.WriteFile(filePath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	// Sobrescreve o hook de escrita de rollback para sempre falhar.
	originalWriteHook := writeSymbolFileForRollback
	writeSymbolFileForRollback = func(_ string, _ []byte, _ fs.FileMode) error {
		return fmt.Errorf("simulated rollback failure")
	}
	t.Cleanup(func() {
		writeSymbolFileForRollback = originalWriteHook
	})

	edits := []TextEdit{{StartLine: 1, EndLine: 1, NewText: "unit Changed;"}}
	_, err := applySymbolEditWithRollback(
		context.Background(),
		nil,
		filePath,
		edits,
		originalContent,
		func(_ context.Context, _ *lsp.Client, _ string, _ []TextEdit) (string, error) {
			return "", fmt.Errorf("simulated apply failure")
		},
	)

	if err == nil {
		t.Fatal("expected applySymbolEditWithRollback to return error when both apply and rollback fail")
	}
	if !strings.Contains(err.Error(), "simulated apply failure") {
		t.Errorf("expected error to reference apply failure, got: %v", err)
	}
	if !strings.Contains(err.Error(), "simulated rollback failure") {
		t.Errorf("expected error to reference rollback failure, got: %v", err)
	}
}
