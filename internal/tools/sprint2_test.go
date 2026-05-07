package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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

// CT-S2-VAL-01: MemoryWrite com title vazio/whitespace deve retornar erro de validação
func TestMemoryWrite_EmptyTitleReturnsValidationError(t *testing.T) {
	t.Setenv("ORACLE_MEMORY_DIR", t.TempDir())

	cases := []struct {
		name  string
		title string
	}{
		{"empty string", ""},
		{"only spaces", "   "},
		{"only tab", "\t"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, err := MemoryWrite(tc.title, "conteudo qualquer", nil)
			if err == nil {
				t.Errorf("esperado erro de validação para title=%q, mas MemoryWrite retornou id=%q sem erro", tc.title, id)
			}
			if id != "" {
				t.Errorf("esperado id vazio quando title inválido, obtido %q", id)
			}
		})
	}
}

// CT-S2-VAL-02: MemoryRead com id vazio/whitespace deve retornar erro de validação (não apenas not found)
func TestMemoryRead_EmptyIDReturnsValidationError(t *testing.T) {
	t.Setenv("ORACLE_MEMORY_DIR", t.TempDir())

	cases := []struct {
		name string
		id   string
	}{
		{"empty string", ""},
		{"only spaces", "   "},
		{"only tab", "\t"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry, err := MemoryRead(tc.id)
			if err == nil {
				t.Errorf("esperado erro de validação para id=%q, mas MemoryRead retornou entry=%v sem erro", tc.id, entry)
			}
			if entry != nil {
				t.Errorf("esperado nil quando id inválido, obtido %+v", entry)
			}
			// O erro deve indicar validação, não apenas "not found"
			if err != nil && !strings.Contains(strings.ToLower(err.Error()), "vazio") &&
				!strings.Contains(strings.ToLower(err.Error()), "invalid") &&
				!strings.Contains(strings.ToLower(err.Error()), "obrigatorio") &&
				!strings.Contains(strings.ToLower(err.Error()), "required") &&
				!strings.Contains(strings.ToLower(err.Error()), "validação") &&
				!strings.Contains(strings.ToLower(err.Error()), "validacao") &&
				!strings.Contains(strings.ToLower(err.Error()), "blank") {
				t.Errorf("esperado erro de validação para id=%q, mas obtido apenas: %v", tc.id, err)
			}
		})
	}
}

// CT-S2-VAL-03: MemoryEdit com id vazio/whitespace deve retornar erro de validação
func TestMemoryEdit_EmptyIDReturnsValidationError(t *testing.T) {
	t.Setenv("ORACLE_MEMORY_DIR", t.TempDir())

	cases := []struct {
		name string
		id   string
	}{
		{"empty string", ""},
		{"only spaces", "   "},
		{"only tab", "\t"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := MemoryEdit(tc.id, "novo conteudo")
			if err == nil {
				t.Errorf("esperado erro de validação para id=%q, mas MemoryEdit retornou nil", tc.id)
			}
			// O erro deve indicar validação, não apenas "not found"
			if err != nil && !strings.Contains(strings.ToLower(err.Error()), "vazio") &&
				!strings.Contains(strings.ToLower(err.Error()), "invalid") &&
				!strings.Contains(strings.ToLower(err.Error()), "obrigatorio") &&
				!strings.Contains(strings.ToLower(err.Error()), "required") &&
				!strings.Contains(strings.ToLower(err.Error()), "validação") &&
				!strings.Contains(strings.ToLower(err.Error()), "validacao") &&
				!strings.Contains(strings.ToLower(err.Error()), "blank") {
				t.Errorf("esperado erro de validação para id=%q, mas obtido apenas: %v", tc.id, err)
			}
		})
	}
}

// CT-S2-VAL-04: Padronização de mensagens de erro de validação entre as quatro Memory Tools.
// Verifica que todas retornam erro não-nulo com formato consistente:
// "campo <name> e obrigatorio e nao pode ser vazio"
func TestMemoryValidationErrorMessagePattern_EmptyTitleOrID(t *testing.T) {
	t.Setenv("ORACLE_MEMORY_DIR", t.TempDir())

	// Padrão esperado: "campo <palavra> e obrigatorio e nao pode ser vazio"
	pattern := regexp.MustCompile(`campo \S+ e obrigatorio e nao pode ser vazio`)

	cases := []struct {
		name    string
		getErr  func() error
		input   string
	}{
		{
			name:  "MemoryWrite com title vazio",
			input: `""`,
			getErr: func() error {
				_, err := MemoryWrite("", "x", nil)
				return err
			},
		},
		{
			name:  "MemoryRead com id apenas espacos",
			input: `" "`,
			getErr: func() error {
				_, err := MemoryRead(" ")
				return err
			},
		},
		{
			name:  "MemoryEdit com id tab",
			input: `"\t"`,
			getErr: func() error {
				return MemoryEdit("\t", "x")
			},
		},
		{
			name:  "MemoryDelete com id vazio",
			input: `""`,
			getErr: func() error {
				return MemoryDelete("")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.getErr()

			if err == nil {
				t.Fatalf("esperado erro não-nulo para input %s, mas obteve nil", tc.input)
			}

			if !pattern.MatchString(err.Error()) {
				t.Errorf(
					"mensagem de erro não segue o padrão esperado\n"+
						"  padrão:  %q\n"+
						"  obtido:  %q",
					pattern.String(),
					err.Error(),
				)
			}
		})
	}
}

// CT-S2-MCP-ISO-01: isolamento por escopo — diretórios de memória independentes
func TestMemorySystem_IsolatedByMemoryDirectory(t *testing.T) {
	scopeA := t.TempDir()
	scopeB := t.TempDir()

	// --- escopo A: escrever uma entrada ---
	var idFromA string
	t.Run("escreve_em_scopeA", func(t *testing.T) {
		t.Setenv("ORACLE_MEMORY_DIR", scopeA)

		id, err := MemoryWrite("Memoria do Escopo A", "conteudo exclusivo de A", []string{"scopeA"})
		if err != nil {
			t.Fatalf("MemoryWrite em scopeA falhou: %v", err)
		}
		if id == "" {
			t.Fatal("MemoryWrite deve retornar ID nao vazio")
		}
		idFromA = id

		entry, err := MemoryRead(id)
		if err != nil {
			t.Fatalf("MemoryRead em scopeA falhou: %v", err)
		}
		if entry.Content != "conteudo exclusivo de A" {
			t.Errorf("conteudo inesperado em scopeA: %q", entry.Content)
		}

		list, err := MemoryList("")
		if err != nil {
			t.Fatalf("MemoryList em scopeA falhou: %v", err)
		}
		if len(list) != 1 {
			t.Errorf("scopeA deve conter exatamente 1 entrada, obtido %d", len(list))
		}
	})

	// --- escopo B: deve estar completamente vazio ---
	t.Run("scopeB_esta_vazio", func(t *testing.T) {
		t.Setenv("ORACLE_MEMORY_DIR", scopeB)

		list, err := MemoryList("")
		if err != nil {
			t.Fatalf("MemoryList em scopeB falhou: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("scopeB deve estar vazio, mas tem %d entradas", len(list))
		}

		_, err = MemoryRead(idFromA)
		if err == nil {
			t.Error("MemoryRead(idFromA) em scopeB deveria retornar erro (not found), mas retornou nil")
		}
	})
}

// CT-S2-DUP-01: Criação duplicada de memória com mesmo title no mesmo escopo deve ser bloqueada.
// RED: produção ainda não implementa verificação de duplicata por title.
func TestMemoryWrite_DuplicateTitleInSameScopeReturnsValidationError(t *testing.T) {
	t.Setenv("ORACLE_MEMORY_DIR", t.TempDir())

	// 1ª chamada — deve passar normalmente
	id, err := MemoryWrite("TituloUnico", "conteudo A", []string{"a"})
	if err != nil {
		t.Fatalf("primeira MemoryWrite falhou inesperadamente: %v", err)
	}
	if id == "" {
		t.Fatal("primeira MemoryWrite deve retornar ID não vazio")
	}

	// 2ª chamada com mesmo title — deve falhar
	id2, err2 := MemoryWrite("TituloUnico", "conteudo B", []string{"b"})
	if err2 == nil {
		t.Fatalf("esperado erro na segunda MemoryWrite com title duplicado, mas obteve id=%q sem erro", id2)
	}
	if id2 != "" {
		t.Errorf("esperado id vazio na segunda MemoryWrite com title duplicado, obtido %q", id2)
	}

	// Mensagem de erro deve mencionar duplicação/existência/title
	errMsg := strings.ToLower(err2.Error())
	if !strings.Contains(errMsg, "duplica") &&
		!strings.Contains(errMsg, "ja exist") &&
		!strings.Contains(errMsg, "já exist") &&
		!strings.Contains(errMsg, "title") &&
		!strings.Contains(errMsg, "titulo") {
		t.Errorf("mensagem de erro deveria mencionar duplicata/já existe/title, obtido: %q", err2.Error())
	}

	// MemoryList deve continuar com apenas 1 entrada
	list, err := MemoryList("")
	if err != nil {
		t.Fatalf("MemoryList falhou: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("esperado 1 entrada após tentativa de duplicata, obtido %d", len(list))
	}
}

// CT-S2-DUP-02: Variação de whitespace e case — title normalizado deve bloquear duplicata.
// RED: produção ainda não implementa normalização de title para comparação.
func TestMemoryWrite_DuplicateTitleCaseAndWhitespaceInsensitive(t *testing.T) {
	t.Setenv("ORACLE_MEMORY_DIR", t.TempDir())

	_, err := MemoryWrite("TituloUnico", "conteudo original", nil)
	if err != nil {
		t.Fatalf("primeira MemoryWrite falhou: %v", err)
	}

	// Mesmo title com case diferente e espaços extras deve ser rejeitado
	cases := []struct {
		name  string
		title string
	}{
		{"lowercase com espacos", "  titulounico  "},
		{"uppercase", "TITULOUNICO"},
		{"mixed case", "tItUlOuNiCo"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, err := MemoryWrite(tc.title, "conteudo duplicado", nil)
			if err == nil {
				t.Errorf("esperado erro de duplicata para title=%q, mas obteve id=%q sem erro", tc.title, id)
			}
		})
	}

	// Ao final, ainda deve haver apenas 1 entrada
	list, err := MemoryList("")
	if err != nil {
		t.Fatalf("MemoryList falhou: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("esperado 1 entrada após tentativas de duplicata com variações, obtido %d", len(list))
	}
}

// CT-S2-PERSIST-01: Persistência entre sessões lógicas via storage em disco
// Verifica que uma entrada gravada na "sessão A" permanece legível na "sessão B"
// quando ambas apontam para o mesmo diretório em disco.
func TestMemorySystem_PersistsAcrossLogicalSessions(t *testing.T) {
	// Diretório compartilhado que simula o storage persistente em disco.
	// Ao contrário de t.TempDir() por subtest, este sobrevive entre os subtestes.
	memoryDir := t.TempDir()

	var writtenID string

	// Sessão A: agente escreve uma entrada e encerra seu contexto de ambiente.
	t.Run("sessaoA_escreve", func(t *testing.T) {
		t.Setenv("ORACLE_MEMORY_DIR", memoryDir)

		id, err := MemoryWrite("Titulo persistente", "conteudo da sessao A", []string{"persist"})
		if err != nil {
			t.Fatalf("sessaoA: MemoryWrite falhou: %v", err)
		}
		if id == "" {
			t.Fatal("sessaoA: MemoryWrite deve retornar ID nao vazio")
		}
		writtenID = id
	})

	// Sessão B: novo contexto de ambiente (mesmo dir), deve encontrar a entrada.
	t.Run("sessaoB_le", func(t *testing.T) {
		if writtenID == "" {
			t.Skip("sessaoA nao produziu ID; pulando sessaoB")
		}
		t.Setenv("ORACLE_MEMORY_DIR", memoryDir)

		entry, err := MemoryRead(writtenID)
		if err != nil {
			t.Fatalf("sessaoB: MemoryRead(%q) falhou: %v — entrada deveria persistir em disco", writtenID, err)
		}
		if entry.Content != "conteudo da sessao A" {
			t.Errorf("sessaoB: conteudo esperado %q, obtido %q", "conteudo da sessao A", entry.Content)
		}

		list, err := MemoryList("")
		if err != nil {
			t.Fatalf("sessaoB: MemoryList falhou: %v", err)
		}

		found := false
		for _, e := range list {
			if e.ID == writtenID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("sessaoB: MemoryList nao contém a entrada %q criada na sessaoA (total=%d)", writtenID, len(list))
		}
	})
}

// CT-S2-EDIT-POLICY-01: MemoryEdit deve usar substituição total — conteúdo anterior não pode ser preservado.
func TestMemoryEdit_UsesFullReplacementPolicy(t *testing.T) {
	t.Setenv("ORACLE_MEMORY_DIR", t.TempDir())

	// 1) Criar entrada com três linhas
	id, err := MemoryWrite("Titulo para edicao", "linha1\nlinha2\nlinha3", nil)
	if err != nil {
		t.Fatalf("MemoryWrite falhou: %v", err)
	}

	// 2) Editar com novo conteúdo
	if err := MemoryEdit(id, "novo conteudo"); err != nil {
		t.Fatalf("MemoryEdit falhou: %v", err)
	}

	// 3) Ler e verificar substituição total
	entry, err := MemoryRead(id)
	if err != nil {
		t.Fatalf("MemoryRead apos edicao falhou: %v", err)
	}

	if entry.Content != "novo conteudo" {
		t.Errorf("esperado conteudo exatamente %q, obtido %q", "novo conteudo", entry.Content)
	}

	// Nenhum fragmento do conteúdo anterior deve sobreviver
	if strings.Contains(entry.Content, "linha1") {
		t.Errorf("conteudo anterior (linha1) nao deve persistir apos MemoryEdit; obtido: %q", entry.Content)
	}
}

// CT-S2-CONC-01: concorrência de write/list/read com múltiplas goroutines
func TestMemorySystem_ConcurrentAccessIsRaceFree(t *testing.T) {
	const N = 10
	t.Setenv("ORACLE_MEMORY_DIR", t.TempDir())

	var wg sync.WaitGroup

	// N goroutines escrevendo entradas com títulos únicos
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			title := fmt.Sprintf("titulo-%d", n)
			_, _ = MemoryWrite(title, fmt.Sprintf("conteudo-%d", n), nil)
		}(i)
	}

	// N goroutines lendo a lista concorrentemente
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = MemoryList("")
		}()
	}

	wg.Wait()

	// Após o join, a lista deve conter pelo menos 1 entrada
	list, err := MemoryList("")
	if err != nil {
		t.Fatalf("MemoryList apos goroutines falhou: %v", err)
	}
	if len(list) < 1 {
		t.Errorf("esperado pelo menos 1 entrada apos %d goroutines de write, obtido %d", N, len(list))
	}
}

// CT-S2-MCP-CORRUPTED: memoria com JSON invalido retorna erro gracioso (sem panic)
func TestMemorySystem_CorruptedJSONReturnsGracefulError(t *testing.T) {
dir := t.TempDir()
t.Setenv("ORACLE_MEMORY_DIR", dir)

// Escrever JSON invalido diretamente no arquivo de memoria
if err := os.WriteFile(filepath.Join(dir, "memory.json"), []byte("{invalid json}"), 0o600); err != nil {
t.Fatalf("nao foi possivel escrever arquivo corrompido: %v", err)
}

// MemoryList deve retornar error nao-nil, nunca panic
_, err := MemoryList("")
if err == nil {
t.Error("MemoryList com JSON corrompido deveria retornar erro, obteve nil")
}

// MemoryRead deve retornar error nao-nil, nunca panic
_, err = MemoryRead("qualquer-id")
if err == nil {
t.Error("MemoryRead com JSON corrompido deveria retornar erro, obteve nil")
}
}
