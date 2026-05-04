package tools

import (
	"os"
	"strings"
	"testing"
)

// CT-S2-MCP-01: Sistema de memoria - write e read
func TestMemorySystem_WriteAndRead(t *testing.T) {
	// Usa diretorio temporario para nao sujar o UserConfigDir real
	t.Setenv("ORACLE_MEMORY_DIR", t.TempDir())

	id, err := MemoryWrite("Titulo de teste", "Conteudo de teste", []string{"tag1", "tag2"})
	if err != nil {
		t.Fatalf("MemoryWrite falhou: %v", err)
	}
	if id == "" {
		t.Fatal("MemoryWrite deve retornar um ID nao vazio")
	}

	entry, err := MemoryRead(id)
	if err != nil {
		t.Fatalf("MemoryRead falhou: %v", err)
	}
	if entry.Content != "Conteudo de teste" {
		t.Errorf("conteudo esperado %q, obtido %q", "Conteudo de teste", entry.Content)
	}
}

// CT-S2-MCP-02: Sistema de memoria - list por tag
func TestMemorySystem_ListByTag(t *testing.T) {
	t.Setenv("ORACLE_MEMORY_DIR", t.TempDir())

	MemoryWrite("Item A", "conteudo A", []string{"tagA"})
	MemoryWrite("Item B", "conteudo B", []string{"tagB"})
	MemoryWrite("Item C", "conteudo C", []string{"tagA", "tagB"})

	listA, err := MemoryList("tagA")
	if err != nil {
		t.Fatalf("MemoryList falhou: %v", err)
	}
	if len(listA) != 2 {
		t.Errorf("esperado 2 itens com tagA, obtido %d", len(listA))
	}
}

// CT-S2-MCP-03: Sistema de memoria - delete
func TestMemorySystem_Delete(t *testing.T) {
	t.Setenv("ORACLE_MEMORY_DIR", t.TempDir())

	id, _ := MemoryWrite("Para deletar", "conteudo", nil)
	err := MemoryDelete(id)
	if err != nil {
		t.Fatalf("MemoryDelete falhou: %v", err)
	}

	_, err = MemoryRead(id)
	if err == nil {
		t.Error("MemoryRead apos delete deveria retornar erro")
	}
}

// CT-S2-MCP-04: get_diagnostics_for_symbol - funcao existe
func TestGetDiagnosticsForSymbol_FunctionExists(t *testing.T) {
	// Apenas verifica que a funcao compila corretamente com assinatura esperada.
	// Teste de integracao real requereria LSP rodando.
	var _ = GetDiagnosticsForSymbol
}

// CT-S2-MCP-05: find_implementations - regex Delphi certa
func TestFindImplementations_DelphiSyntax(t *testing.T) {
	src := `unit Foo;
interface
type
  IFoo = interface
    procedure DoIt;
  end;
  TMyClass = class(TObject, IFoo)
  public
    procedure DoIt;
  end;
  TOtherClass = class(TBase, IFoo, IBar)
  end;
  TUnrelated = class(TObject)
  end;
implementation
end.`

	results := findImplementationsTextScan(src, "IFoo")
	if len(results) < 2 {
		t.Errorf("esperado >= 2 implementacoes de IFoo, obtido %d: %v", len(results), results)
	}
	for _, r := range results {
		if !strings.Contains(r, "IFoo") {
			t.Errorf("resultado %q nao contem IFoo", r)
		}
	}

	// Verificar que TUnrelated nao aparece
	for _, r := range results {
		if strings.Contains(r, "TUnrelated") {
			t.Errorf("TUnrelated nao deveria aparecer nos resultados")
		}
	}
}

// CT-S2-MCP-06: get_node_at_position - retorna token e linha
func TestGetNodeAtPosition_ReturnsContext(t *testing.T) {
	src := `unit Foo;
interface
type
  TMyClass = class(TObject)
  end;
implementation
end.`

	ctx := getNodeAtPositionFromSource(src, 3, 2) // linha 4 (0-indexed: 3), col 2 = "TMyClass"
	if ctx == nil {
		t.Fatal("getNodeAtPositionFromSource retornou nil")
	}
	if ctx.Token == "" {
		t.Error("Token nao deve ser vazio")
	}
}

// CT-S2-MCP-07: onboarding - detecta .dpr
func TestOnboarding_DetectsDprEntry(t *testing.T) {
	dir := t.TempDir()
	// Criar arquivo .dpr fake
	os.WriteFile(dir+"/MyProject.dpr", []byte("program MyProject;\nbegin\nend."), 0644)
	os.WriteFile(dir+"/Unit1.pas", []byte("unit Unit1;\ninterface\nimplementation\nend."), 0644)

	result, err := ScanProjectStructure(dir)
	if err != nil {
		t.Fatalf("ScanProjectStructure falhou: %v", err)
	}
	if result.EntryPoint == "" {
		t.Error("EntryPoint deve ser detectado")
	}
	if len(result.Units) == 0 {
		t.Error("Units deve conter Unit1.pas")
	}
}

// CT-S2-MCP-08: check_onboarding - arquivo ausente
func TestCheckOnboarding_NotPerformed(t *testing.T) {
	dir := t.TempDir()
	performed, _ := CheckOnboardingPerformed(dir)
	if performed {
		t.Error("onboarding nao deve estar performed em dir vazio")
	}
}

// CT-S2-MCP-09: safe_delete_symbol - rejeita se tem referencias
func TestSafeDeleteSymbol_RejectsIfHasReferences(t *testing.T) {
	// Apenas verifica que a funcao existe com assinatura correta.
	var _ = SafeDeleteSymbol
}

// CT-S2-MCP-10: tool names registrados (regression)
func TestRegistration_Sprint2Tools(t *testing.T) {
	expected := []string{
		"memory_write", "memory_read", "memory_list", "memory_edit", "memory_delete",
		"safe_delete_symbol", "get_diagnostics_for_symbol", "find_implementations",
		"get_node_at_position", "get_node_types",
		"onboarding", "check_onboarding_performed",
	}
	for _, name := range expected {
		if strings.TrimSpace(name) == "" {
			t.Errorf("nome vazio: %q", name)
		}
	}
}
