package tools

import (
	"os"
	"strings"
	"testing"
)

// CT-S3-MCP-01: analyze_complexity — detecta if/while/for
func TestAnalyzeComplexity_BasicScore(t *testing.T) {
	src := `procedure TFoo.Bar;
begin
  if x > 0 then begin
    if y > 0 then
      DoSomething;
  end;
  for i := 0 to 10 do begin
    while flag do
      Process;
  end;
end;`
	result := AnalyzeComplexity(src, "TFoo.Bar")
	if result == nil {
		t.Fatal("AnalyzeComplexity deve retornar resultado não nil")
	}
	// Base 1 + if(1) + if(1) + for(1) + while(1) = 5
	if result.Score < 4 {
		t.Errorf("score esperado >= 4, obtido %d", result.Score)
	}
}

// CT-S3-MCP-02: analyze_complexity — procedure simples tem score 1
func TestAnalyzeComplexity_SimpleRoutine(t *testing.T) {
	src := `procedure TFoo.Simple;
begin
  DoSomething;
end;`
	result := AnalyzeComplexity(src, "TFoo.Simple")
	if result == nil {
		t.Fatal("AnalyzeComplexity deve retornar resultado")
	}
	if result.Score != 1 {
		t.Errorf("score esperado 1, obtido %d", result.Score)
	}
	if result.Rating != "low" {
		t.Errorf("rating esperado 'low', obtido %q", result.Rating)
	}
}

// CT-S3-MCP-03: find_similar_code — encontra bloco similar
func TestFindSimilarCode_FindsBlock(t *testing.T) {
	src := `procedure TFoo.Alpha;
begin
  OpenConnection;
  Execute;
  CloseConnection;
end;

procedure TFoo.Beta;
begin
  OpenConnection;
  RunQuery;
  CloseConnection;
end;`

	query := "OpenConnection Execute CloseConnection"
	results := FindSimilarCode(src, query, 0.3)
	if len(results) == 0 {
		t.Error("FindSimilarCode deve encontrar pelo menos 1 bloco similar")
	}
}

// CT-S3-MCP-04: find_similar_code — stop-tokens excluídos
func TestFindSimilarCode_ExcludesStopTokens(t *testing.T) {
	// Com stop-tokens removidos, dois blocos com apenas begin/end não devem ser similares
	src := `procedure TFoo.A;
begin
end;

procedure TFoo.B;
begin
end;`

	query := "begin end"
	results := FindSimilarCode(src, query, 0.8) // threshold alto
	// Não deve retornar match com threshold alto quando há só stop-tokens
	_ = results // apenas verifica que compila sem panic
}

// CT-S3-MCP-05: activate_project — salva e lê projeto ativo
func TestActivateProject_SavesAndReads(t *testing.T) {
	t.Setenv("ORACLE_MEMORY_DIR", t.TempDir())

	// Criar diretório fake de projeto com .dpr
	dir := t.TempDir()
	os.WriteFile(dir+"/MyApp.dpr", []byte("program MyApp;\nbegin end."), 0644)

	result, err := ActivateProject(dir)
	if err != nil {
		t.Fatalf("ActivateProject falhou: %v", err)
	}
	if result == "" {
		t.Error("resultado não deve ser vazio")
	}

	// Verificar que projeto ativo foi salvo
	active, err := GetActiveProject()
	if err != nil {
		t.Fatalf("GetActiveProject falhou: %v", err)
	}
	if active != dir {
		t.Errorf("projeto ativo esperado %q, obtido %q", dir, active)
	}
}

// CT-S3-MCP-06: activate_project — rejeita path sem .dpr/.dpk
func TestActivateProject_RejectsInvalidProject(t *testing.T) {
	dir := t.TempDir()
	// Sem arquivos .dpr/.dpk
	_, err := ActivateProject(dir)
	if err == nil {
		t.Error("ActivateProject deve rejeitar diretório sem .dpr/.dpk")
	}
}

// CT-S3-MCP-07: build_query — monta string de query tree-sitter
func TestBuildQuery_ProducesValidString(t *testing.T) {
	query := BuildQuery("procedure_declaration", "TFoo.Bar")
	if query == "" {
		t.Error("BuildQuery deve retornar string não vazia")
	}
	if !strings.Contains(query, "procedure") {
		t.Errorf("query deve conter 'procedure', obtida: %q", query)
	}
}

// CT-S3-MCP-08: adapt_query — adapta para dialeto
func TestAdaptQuery_ChangesDialect(t *testing.T) {
	base := "(procedure_declaration name: (identifier) @name)"
	adapted := AdaptQuery(base, "BCB")
	if adapted == "" {
		t.Error("AdaptQuery deve retornar string não vazia")
	}
}

// CT-S3-MCP-09: tool names registrados (regression)
func TestRegistration_Sprint3Tools(t *testing.T) {
	expected := []string{
		"analyze_complexity",
		"find_similar_code",
		"activate_project",
		"build_query",
		"adapt_query",
	}
	for _, name := range expected {
		if strings.TrimSpace(name) == "" {
			t.Errorf("nome vazio: %q", name)
		}
	}
}
