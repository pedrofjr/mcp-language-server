package tools

import (
	"strings"
	"testing"
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

func TestToolsRegistration_SymbolEditTools(t *testing.T) {
	// Verifica que os nomes das tools estão definidos como constantes
	names := []string{toolReplaceSymbolBody, toolInsertAfterSymbol, toolInsertBeforeSymbol}
	for _, n := range names {
		if strings.TrimSpace(n) == "" {
			t.Errorf("constante de tool vazia: %q", n)
		}
	}
}
