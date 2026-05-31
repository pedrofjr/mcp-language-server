package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
)

func TestResolveDelphiRoutine_QualifiedBarNotBarEx(t *testing.T) {
	src := `unit Foo;
interface
  TFoo = class
    procedure Bar;
    procedure BarEx;
  end;
implementation

procedure TFoo.Bar;
begin
  Writeln('bar');
end;

procedure TFoo.BarEx;
begin
  Writeln('bar-ex');
end;

end.`

	bounded, err := resolveDelphiRoutineBoundaries(src, "TFoo.Bar")
	if err != nil {
		t.Fatalf("expected TFoo.Bar to resolve, got error: %v", err)
	}

	lines := strings.Split(src, "\n")
	if strings.TrimSpace(lines[bounded.beginLine-1]) != "begin" {
		t.Fatalf("expected outer begin for TFoo.Bar, got line %d: %q", bounded.beginLine, lines[bounded.beginLine-1])
	}
	if !strings.Contains(lines[bounded.beginLine], "bar") && !strings.Contains(lines[bounded.beginLine+1], "bar") {
		t.Fatalf("expected TFoo.Bar body near beginLine=%d", bounded.beginLine)
	}
	if strings.Contains(lines[bounded.beginLine-1], "bar-ex") {
		t.Fatalf("TFoo.Bar must not resolve to BarEx body")
	}
}

func TestResolveDelphiRoutine_UnqualifiedBarIsAmbiguous(t *testing.T) {
	src := `unit Foo;
implementation

procedure TFoo.Bar;
begin
end;

procedure TOther.Bar;
begin
end;

end.`

	_, err := resolveDelphiRoutineBoundaries(src, "Bar")
	if !errors.Is(err, ErrDelphiSymbolAmbiguous) {
		t.Fatalf("expected ambiguous error for unqualified Bar, got: %v", err)
	}
}

func TestResolveDelphiRoutine_BeginEndInsideStringDoesNotExpandBody(t *testing.T) {
	src := `unit Foo;
implementation

procedure TFoo.Bar;
begin
  Msg := 'fake begin end; inside string';
  Writeln('ok');
end;

end.`

	bounded, err := resolveDelphiRoutineBoundaries(src, "TFoo.Bar")
	if err != nil {
		t.Fatalf("expected TFoo.Bar to resolve, got: %v", err)
	}
	if bounded.endLine-bounded.beginLine < 2 {
		t.Fatalf("expected body span to include multiple lines, got begin=%d end=%d", bounded.beginLine, bounded.endLine)
	}

	lines := strings.Split(src, "\n")
	if strings.TrimSpace(lines[bounded.endLine-1]) != "end;" {
		t.Fatalf("expected outer end; at endLine, got %q", lines[bounded.endLine-1])
	}
}

func TestResolveDelphiRoutine_BeginEndInsideCommentDoesNotExpandBody(t *testing.T) {
	src := `unit Foo;
implementation

procedure TFoo.Bar;
begin
  { fake begin end; inside comment }
  Writeln('ok');
end;

end.`

	bounded, err := resolveDelphiRoutineBoundaries(src, "TFoo.Bar")
	if err != nil {
		t.Fatalf("expected TFoo.Bar to resolve, got: %v", err)
	}
	lines := strings.Split(src, "\n")
	if strings.TrimSpace(lines[bounded.endLine-1]) != "end;" {
		t.Fatalf("expected outer end; at endLine, got %q", lines[bounded.endLine-1])
	}
}

func TestResolveDelphiRoutine_DecoyDeclarationInCommentIgnored(t *testing.T) {
	src := `unit Foo;
implementation

// procedure TFoo.BarEx;
procedure TFoo.Bar;
begin
end;

procedure TFoo.BarEx;
begin
end;

end.`

	bounded, err := resolveDelphiRoutineBoundaries(src, "TFoo.Bar")
	if err != nil {
		t.Fatalf("expected TFoo.Bar to resolve, got: %v", err)
	}
	if bounded.headerLine != 5 {
		t.Fatalf("expected TFoo.Bar header at line 5, got %d", bounded.headerLine)
	}
}

func TestReplaceSymbolBody_AmbiguousSymbolDoesNotMutateFile(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "foo.pas")
	original := strings.Join([]string{
		"unit Foo;",
		"implementation",
		"",
		"procedure TFoo.Bar;",
		"begin",
		"  Writeln('bar');",
		"end;",
		"",
		"procedure TOther.Bar;",
		"begin",
		"  Writeln('other');",
		"end;",
		"",
		"end.",
	}, "\n")
	if err := os.WriteFile(filePath, []byte(original), 0o644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	_, err := ReplaceSymbolBody(context.Background(), nil, filePath, "Bar", "  Writeln('new');")
	if err == nil {
		t.Fatal("expected ReplaceSymbolBody to fail for ambiguous symbol")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "ambiguous") {
		t.Fatalf("expected ambiguous error, got: %v", err)
	}

	finalBytes, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("failed to read file after failed replace: %v", readErr)
	}
	if string(finalBytes) != original {
		t.Fatalf("expected file unchanged on ambiguous symbol, got:\n%s", string(finalBytes))
	}
}

func TestReplaceSymbolBody_OverloadSameNameFailsWithoutMutation(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "foo.pas")
	original := strings.Join([]string{
		"unit Foo;",
		"implementation",
		"",
		"procedure TFoo.Alpha(const A: Integer);",
		"begin",
		"  Writeln('int');",
		"end;",
		"",
		"procedure TFoo.Alpha(const A: string);",
		"begin",
		"  Writeln('str');",
		"end;",
		"",
		"end.",
	}, "\n")
	if err := os.WriteFile(filePath, []byte(original), 0o644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	_, err := ReplaceSymbolBody(context.Background(), nil, filePath, "TFoo.Alpha", "  Writeln('new');")
	if err == nil {
		t.Fatal("expected ReplaceSymbolBody to fail for ambiguous overloads")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "ambiguous") {
		t.Fatalf("expected ambiguous error for overloads, got: %v", err)
	}

	finalBytes, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("failed to read file: %v", readErr)
	}
	if string(finalBytes) != original {
		t.Fatalf("expected file unchanged for ambiguous overload")
	}
}

func TestReplaceSymbolBody_QualifiedBarExPicksTargetRoutine(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "foo.pas")
	original := strings.Join([]string{
		"unit Foo;",
		"implementation",
		"",
		"procedure TFoo.Bar;",
		"begin",
		"  Writeln('bar');",
		"end;",
		"",
		"procedure TFoo.BarEx;",
		"begin",
		"  Writeln('bar-ex');",
		"end;",
		"",
		"end.",
	}, "\n")
	if err := os.WriteFile(filePath, []byte(original), 0o644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	applySymbolBodyTextEdits = func(_ context.Context, _ *lsp.Client, _ string, edits []TextEdit) (string, error) {
		if len(edits) != 1 {
			t.Fatalf("expected one edit, got %d", len(edits))
		}
		if edits[0].StartLine != 11 || edits[0].EndLine != 11 {
			t.Fatalf("expected inner body edit on BarEx lines 11..11, got %d..%d", edits[0].StartLine, edits[0].EndLine)
		}
		return "ok", nil
	}
	t.Cleanup(func() {
		applySymbolBodyTextEdits = ApplyTextEdits
	})

	_, err := ReplaceSymbolBody(context.Background(), nil, filePath, "TFoo.BarEx", "  Writeln('new');")
	if err != nil {
		t.Fatalf("expected ReplaceSymbolBody to succeed for TFoo.BarEx, got: %v", err)
	}
}

func TestResolveDelphiRoutine_HarnessTMutateTarget(t *testing.T) {
	src := strings.Join([]string{
		"unit SymbolMutateHarness;",
		"implementation",
		"",
		"procedure TMutate.Target;",
		"begin",
		"end;",
		"",
		"end.",
	}, "\n")
	bounded, err := resolveDelphiRoutineBoundaries(src, "TMutate.Target")
	if err != nil {
		t.Fatalf("resolveDelphiRoutineBoundaries: %v", err)
	}
	if bounded.beginLine <= 0 || bounded.endLine <= 0 {
		t.Fatalf("unexpected bounds: %+v", bounded)
	}
}

func TestReplaceSymbolBody_HarnessFixtureQualifiedTarget(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "symbol_mutate.pas")
	original := strings.Join([]string{
		"unit SymbolMutateHarness;",
		"implementation",
		"",
		"procedure TMutate.Target;",
		"begin",
		"  Writeln('seed');",
		"end;",
		"",
		"procedure DeleteMe;",
		"begin",
		"  Writeln('delete-me');",
		"end;",
		"",
		"end.",
	}, "\n")
	if err := os.WriteFile(filePath, []byte(original), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	applySymbolBodyTextEdits = func(_ context.Context, _ *lsp.Client, _ string, edits []TextEdit) (string, error) {
		if len(edits) != 1 {
			t.Fatalf("expected one edit, got %d", len(edits))
		}
		return "ok", nil
	}
	t.Cleanup(func() {
		applySymbolBodyTextEdits = ApplyTextEdits
	})

	_, err := ReplaceSymbolBody(
		context.Background(),
		nil,
		filePath,
		"TMutate.Target",
		"begin\n  // HARNESS_BODY_REPLACED\nend;",
	)
	if err != nil {
		t.Fatalf("ReplaceSymbolBody harness fixture: %v", err)
	}
}
