package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindImplementations_NilClientFallsBackWithoutPanic(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "sample.pas")
	source := `unit Sample;

interface

type
  IFoo = interface
  end;

  TMyClass = class(TObject, IFoo)
  end;

implementation

end.`

	if err := os.WriteFile(filePath, []byte(source), 0o644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("FindImplementations panicked with nil client: %v", recovered)
		}
	}()

	result, err := FindImplementations(context.Background(), nil, filePath, "IFoo", tempDir)
	if err != nil {
		t.Fatalf("expected fallback scan to succeed, got error: %v", err)
	}

	if !strings.Contains(result, "implements IFoo") {
		t.Fatalf("expected fallback result to include implementation match, got %q", result)
	}
}

func TestFindImplementationsTextScan_MultilineHeritage(t *testing.T) {
	source := `unit Sample;

interface

type
  IFoo = interface
  end;

  TMyClass = class(
    TObject,
    IFoo
  );

implementation

end.`

	results := findImplementationsTextScan(source, "IFoo")
	if len(results) != 1 {
		t.Fatalf("expected one IFoo implementation, got %d: %v", len(results), results)
	}

	if !strings.Contains(results[0], "TMyClass implements IFoo") {
		t.Fatalf("unexpected multiline scan result: %q", results[0])
	}
}

func TestFindSymbolPositionExact_IgnoresCommentAndPartial(t *testing.T) {
	source := strings.Join([]string{
		"unit Sample;",
		"interface",
		"var",
		"  IFoobar: Integer; // IFoo in comment should be ignored",
		"  Value: Pointer;",
		"implementation",
		"begin",
		"  Value := IFoo;",
		"end.",
	}, "\n")

	line, column, err := findSymbolPositionExact(source, "IFoo")
	if err != nil {
		t.Fatalf("expected exact IFoo token to be found, got error: %v", err)
	}

	lines := strings.Split(source, "\n")
	if line < 0 || line >= len(lines) {
		t.Fatalf("line out of bounds: %d", line)
	}
	if column < 0 || column+len("IFoo") > len(lines[line]) {
		t.Fatalf("column out of bounds: %d", column)
	}
	if token := lines[line][column : column+len("IFoo")]; token != "IFoo" {
		t.Fatalf("expected exact token IFoo at located position, got %q", token)
	}

	qualifiedLine, qualifiedColumn, err := findSymbolPositionExact(source, "Sample.IFoo")
	if err != nil {
		t.Fatalf("expected qualified symbol fallback to simple token, got error: %v", err)
	}
	if qualifiedLine != line || qualifiedColumn != column {
		t.Fatalf("expected qualified lookup to resolve same location, got line=%d column=%d", qualifiedLine, qualifiedColumn)
	}

	_, _, err = findSymbolPositionExact("IFoobar // IFoo only in comment", "IFoo")
	if err == nil {
		t.Fatalf("expected no exact token match when only partial or comment text is present")
	}
}
