package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

const (
	fakeLSPEnv                                      = "MCP_FAKE_LSP_DELPHI_ORACLE"
	fakeLSPWorkspaceSymbolProviderEnv               = "MCP_FAKE_LSP_DELPHI_ORACLE_WORKSPACE_SYMBOL_PROVIDER"
	fakeLSPWorkspaceSymbolEmptyEnv                  = "MCP_FAKE_LSP_DELPHI_ORACLE_WORKSPACE_SYMBOL_EMPTY_RESULT"
	fakeLSPWorkspaceSymbolIrrelevantEnv             = "MCP_FAKE_LSP_DELPHI_ORACLE_WORKSPACE_SYMBOL_IRRELEVANT_RESULT"
	fakeLSPWorkspaceSymbolMatchedButUnsustainedEnv  = "MCP_FAKE_LSP_DELPHI_ORACLE_WORKSPACE_SYMBOL_MATCHED_BUT_UNSUSTAINED_RESULT"
	fakeLSPWorkspaceSymbolQualifiedTypeAmbiguousEnv = "MCP_FAKE_LSP_DELPHI_ORACLE_WORKSPACE_SYMBOL_QUALIFIED_TYPE_AMBIGUOUS_RESULT"
	fakeLSPWorkspaceSymbolQualifiedTypeNoStrongEnv  = "MCP_FAKE_LSP_DELPHI_ORACLE_WORKSPACE_SYMBOL_QUALIFIED_TYPE_NO_STRONG_RESULT"
	fakeLSPReferencesPayloadEnv                     = "MCP_FAKE_LSP_DELPHI_ORACLE_REFERENCES_PAYLOAD"
)

var fakeLSPReferenceLinePattern = regexp.MustCompile(`L(\d+):C\d+`)

type fakeDelphiOracleReferenceRange struct {
	StartLine      int `json:"startLine"`
	StartCharacter int `json:"startCharacter"`
	EndLine        int `json:"endLine"`
	EndCharacter   int `json:"endCharacter"`
}

func TestHelperProcessDelphiOracleFakeLSP(t *testing.T) {
	if os.Getenv(fakeLSPEnv) != "1" {
		return
	}

	runDelphiOracleFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestDelphiOracle_ReadDefinition_FallbackWhenWorkspaceSymbolUnavailable(t *testing.T) {
	client, filePath, cleanup := setupDelphiOracleFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture para definition: %v", err)
	}

	result, err := ReadDefinition(ctx, client, "FindCustomer")
	if err != nil {
		t.Fatalf("esperado fallback em definition sem workspace/symbol, mas recebeu erro: %v", err)
	}

	if !strings.Contains(result, "FindCustomer") {
		t.Fatalf("esperado resultado de definition contendo o simbolo FindCustomer, obtido: %s", result)
	}
}

func TestDelphiOracle_ReadDefinition_QualifiedQueryFallsBackWhenWorkspaceSymbolResultsAreIrrelevant(t *testing.T) {
	t.Setenv(fakeLSPWorkspaceSymbolProviderEnv, "1")
	t.Setenv(fakeLSPWorkspaceSymbolIrrelevantEnv, "1")

	testCases := []struct {
		name        string
		fileName    string
		symbol      string
		content     string
		mustContain string
	}{
		{
			name:     "synautil_timezonebias",
			fileName: "synautil_case.pas",
			symbol:   "synautil.TimeZoneBias",
			content: strings.Join([]string{
				"function TimeZoneBias: Integer;",
				"begin",
				"  Result := synautil.TimeZoneBias;",
				"end;",
				"",
			}, "\n"),
			mustContain: "Symbol: synautil.TimeZoneBias",
		},
		{
			name:     "asn1util_asnencoiditem",
			fileName: "asn1util_case.pas",
			symbol:   "asn1util.ASNEncOIDItem",
			content: strings.Join([]string{
				"function ASNEncOIDItem: Integer;",
				"begin",
				"  Result := asn1util.ASNEncOIDItem;",
				"end;",
				"",
			}, "\n"),
			mustContain: "Symbol: asn1util.ASNEncOIDItem",
		},
		{
			name:     "clamsend_create",
			fileName: "clamsend_case.pas",
			symbol:   "clamsend.TClamSend.Create",
			content: strings.Join([]string{
				"constructor TClamSend.Create;",
				"begin",
				"  Client := clamsend.TClamSend.Create;",
				"end;",
				"",
			}, "\n"),
			mustContain: "Symbol: clamsend.TClamSend.Create",
		},
		{
			name:     "synedittextbuffer_stringlist",
			fileName: "synedit_case.pas",
			symbol:   "SynEditTextBuffer.TSynEditStringList",
			content: strings.Join([]string{
				"type TSynEditStringList = class;",
				"begin",
				"  Buffer := SynEditTextBuffer.TSynEditStringList.Create;",
				"end;",
				"",
			}, "\n"),
			mustContain: "Symbol: SynEditTextBuffer.TSynEditStringList",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, map[string]string{
				testCase.fileName: testCase.content,
			})
			defer cleanup()

			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()

			filePath := filePaths[testCase.fileName]
			if err := client.OpenFile(ctx, filePath); err != nil {
				t.Fatalf("falha ao abrir fixture qualificada para definition: %v", err)
			}

			result, err := ReadDefinition(ctx, client, testCase.symbol)
			if err != nil {
				t.Fatalf("ReadDefinition nao deveria falhar quando workspace/symbol retorna resultado irrelevante: %v", err)
			}

			if strings.Contains(result, "not found") {
				t.Fatalf("esperado fallback Delphi apos filtragem de workspace/symbol irrelevante para %s; obtido: %s", testCase.symbol, result)
			}

			if !strings.Contains(result, testCase.mustContain) {
				t.Fatalf("esperado definition conter %q apos fallback Delphi; obtido: %s", testCase.mustContain, result)
			}
		})
	}
}

func TestDelphiOracle_ReadDefinition_QualifiedQueryFallsBackWhenMatchedWorkspaceSymbolDoesNotSustainRequestedSymbol(t *testing.T) {
	t.Setenv(fakeLSPWorkspaceSymbolProviderEnv, "1")
	t.Setenv(fakeLSPWorkspaceSymbolMatchedButUnsustainedEnv, "1")

	testCases := []struct {
		name        string
		fileName    string
		symbol      string
		content     string
		mustContain string
	}{
		{
			name:     "synautil_timezonebias",
			fileName: "synautil.pas",
			symbol:   "synautil.TimeZoneBias",
			content: strings.Join([]string{
				"unit synautil;",
				"",
				"interface",
				"",
				"function TimeZoneBias: Integer;",
				"",
				"implementation",
				"",
				"end.",
			}, "\n"),
			mustContain: "function TimeZoneBias: Integer;",
		},
		{
			name:     "asn1util_asnencoiditem",
			fileName: "asn1util.pas",
			symbol:   "asn1util.ASNEncOIDItem",
			content: strings.Join([]string{
				"unit asn1util;",
				"",
				"interface",
				"",
				"function ASNEncOIDItem: Integer;",
				"",
				"implementation",
				"",
				"end.",
			}, "\n"),
			mustContain: "function ASNEncOIDItem: Integer;",
		},
		{
			name:     "clamsend_create",
			fileName: "clamsend.pas",
			symbol:   "clamsend.TClamSend.Create",
			content: strings.Join([]string{
				"unit clamsend;",
				"",
				"interface",
				"",
				"type",
				"  TClamSend = class",
				"  public",
				"    constructor Create;",
				"  end;",
				"",
				"implementation",
				"",
				"constructor TClamSend.Create;",
				"begin",
				"end;",
				"",
				"end.",
			}, "\n"),
			mustContain: "constructor TClamSend.Create;",
		},
		{
			name:     "synedittextbuffer_stringlist",
			fileName: "SynEditTextBuffer.pas",
			symbol:   "SynEditTextBuffer.TSynEditStringList",
			content: strings.Join([]string{
				"unit SynEditTextBuffer;",
				"",
				"interface",
				"",
				"type",
				"  TSynEditStringList = class",
				"  end;",
				"",
				"implementation",
				"",
				"end.",
			}, "\n"),
			mustContain: "TSynEditStringList = class",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, map[string]string{
				testCase.fileName: testCase.content,
			})
			defer cleanup()

			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()

			result, err := ReadDefinition(ctx, client, testCase.symbol)
			if err != nil {
				t.Fatalf("ReadDefinition nao deveria falhar quando o fallback Delphi final precisa recuperar uma query qualificada matchada por workspace/symbol: %v", err)
			}

			if strings.Contains(result, "not found") {
				t.Fatalf("esperado fallback Delphi final apos workspace/symbol matchado nao sustentar %s; obtido: %s", testCase.symbol, result)
			}

			providerPath := filePaths[testCase.fileName]
			if !strings.Contains(result, providerPath) {
				t.Fatalf("esperado definition cair para o provider Delphi %s apos match workspace/symbol nao sustentado; obtido: %s", providerPath, result)
			}

			if !strings.Contains(result, "Symbol: "+testCase.symbol) {
				t.Fatalf("esperado definition preservar o simbolo qualificado %s no fallback Delphi final; obtido: %s", testCase.symbol, result)
			}

			if !strings.Contains(result, testCase.mustContain) {
				t.Fatalf("esperado definition incluir a declaracao %q apos fallback Delphi final; obtido: %s", testCase.mustContain, result)
			}
		})
	}
}

func TestDelphiOracle_ReadDefinition_QualifiedTypePrefersClassDeclarationOverConstructorWorkspaceSymbol(t *testing.T) {
	t.Setenv(fakeLSPWorkspaceSymbolProviderEnv, "1")
	t.Setenv(fakeLSPWorkspaceSymbolQualifiedTypeAmbiguousEnv, "1")

	fixtures := map[string]string{
		"blcksock.pas": strings.Join([]string{
			"unit blcksock;",
			"",
			"interface",
			"",
			"type",
			"  TBlockSocket = class",
			"  public",
			"    constructor Create;",
			"  end;",
			"",
			"implementation",
			"",
			"constructor TBlockSocket.Create;",
			"begin",
			"  Sock := 1;",
			"end;",
			"",
			"end.",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	filePath := filePaths["blcksock.pas"]
	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture blcksock para selection de definition qualificada por tipo: %v", err)
	}

	result, err := ReadDefinition(ctx, client, "blcksock.TBlockSocket")
	if err != nil {
		t.Fatalf("ReadDefinition nao deveria falhar com workspace/symbol ambiguo para tipo qualificado: %v", err)
	}

	if !strings.Contains(result, "TBlockSocket = class") {
		t.Fatalf("esperado definition qualificada de tipo retornar declaracao de classe TBlockSocket, obtido: %s", result)
	}

	if strings.Contains(result, "constructor TBlockSocket.Create;") {
		t.Fatalf("consulta qualificada de tipo blcksock.TBlockSocket nao deve selecionar construtor Create vindo de workspace/symbol ambiguo; obtido: %s", result)
	}
}

func TestDelphiOracle_ReadDefinition_QualifiedTypeFallbackWithoutWorkspaceSymbolPrefersClassDeclarationOverConstructor(t *testing.T) {
	fixtures := map[string]string{
		"blcksock.pas": strings.Join([]string{
			"unit blcksock;",
			"",
			"interface",
			"",
			"type",
			"  TBlockSocket = class",
			"  public",
			"    constructor Create;",
			"  end;",
			"",
			"implementation",
			"",
			"constructor TBlockSocket.Create;",
			"begin",
			"  Sock := 1;",
			"end;",
			"",
			"end.",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	filePath := filePaths["blcksock.pas"]
	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture blcksock para fallback de definition sem workspace/symbol: %v", err)
	}

	result, err := ReadDefinition(ctx, client, "blcksock.TBlockSocket")
	if err != nil {
		t.Fatalf("ReadDefinition nao deveria falhar no fallback Delphi sem workspace/symbol para tipo qualificado: %v", err)
	}

	if !strings.Contains(result, "TBlockSocket = class") {
		t.Fatalf("esperado fallback Delphi sem workspace/symbol retornar declaracao de classe TBlockSocket, obtido: %s", result)
	}

	if strings.Contains(result, "constructor TBlockSocket.Create;") {
		t.Fatalf("fallback Delphi sem workspace/symbol para blcksock.TBlockSocket nao deve selecionar construtor Create; obtido: %s", result)
	}
}

func TestDelphiOracle_ReadDefinition_QualifiedMemberQueryContinuesResolvingMethod(t *testing.T) {
	t.Setenv(fakeLSPWorkspaceSymbolProviderEnv, "1")
	t.Setenv(fakeLSPWorkspaceSymbolQualifiedTypeAmbiguousEnv, "1")

	fixtures := map[string]string{
		"blcksock.pas": strings.Join([]string{
			"unit blcksock;",
			"",
			"interface",
			"",
			"type",
			"  TBlockSocket = class",
			"  public",
			"    constructor Create;",
			"  end;",
			"",
			"implementation",
			"",
			"constructor TBlockSocket.Create;",
			"begin",
			"  Sock := 1;",
			"end;",
			"",
			"end.",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	filePath := filePaths["blcksock.pas"]
	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture blcksock para definition qualificada de metodo: %v", err)
	}

	result, err := ReadDefinition(ctx, client, "blcksock.TBlockSocket.Create")
	if err != nil {
		t.Fatalf("ReadDefinition nao deveria falhar para query qualificada de metodo Unit.Type.Method: %v", err)
	}

	if !strings.Contains(result, "constructor TBlockSocket.Create;") {
		t.Fatalf("esperado definition qualificada Unit.Type.Method continuar resolvendo o metodo Create, obtido: %s", result)
	}
}

func TestDelphiOracle_ReadDefinition_QualifiedTypeQueryWithoutStrongTypeFallsBackToBestAvailableKind(t *testing.T) {
	t.Setenv(fakeLSPWorkspaceSymbolProviderEnv, "1")
	t.Setenv(fakeLSPWorkspaceSymbolQualifiedTypeNoStrongEnv, "1")

	fixtures := map[string]string{
		"blcksock.pas": strings.Join([]string{
			"unit blcksock;",
			"",
			"interface",
			"",
			"type",
			"  TBlockSocket = class",
			"  public",
			"    constructor Create;",
			"  end;",
			"",
			"implementation",
			"",
			"constructor TBlockSocket.Create;",
			"begin",
			"  Sock := 1;",
			"end;",
			"",
			"end.",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	filePath := filePaths["blcksock.pas"]
	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture blcksock para definition qualificada sem tipo forte: %v", err)
	}

	result, err := ReadDefinition(ctx, client, "blcksock.TBlockSocket")
	if err != nil {
		t.Fatalf("ReadDefinition nao deveria falhar quando A.B nao tem kind de tipo forte no workspace/symbol: %v", err)
	}

	if strings.Contains(result, "not found") {
		t.Fatalf("esperado fallback de ranking para melhor candidato disponivel quando tipo forte nao existe para A.B; obtido: %s", result)
	}

	if !strings.Contains(result, "constructor TBlockSocket.Create;") {
		t.Fatalf("esperado A.B sem tipo forte manter fallback compativel e resolver candidato de membro disponivel, obtido: %s", result)
	}
}

func TestDelphiOracle_ReadDefinition_QualifiedQueryFallsBackToProviderDeclarationWithoutQualifiedLiteral(t *testing.T) {
	t.Setenv(fakeLSPWorkspaceSymbolProviderEnv, "1")
	t.Setenv(fakeLSPWorkspaceSymbolIrrelevantEnv, "1")

	fixtures := map[string]string{
		"synautil.pas": strings.Join([]string{
			"unit synautil;",
			"",
			"interface",
			"",
			"function TimeZoneBias: Integer;",
			"",
			"implementation",
			"",
			"function TimeZoneBias: Integer;",
			"begin",
			"  Result := 180;",
			"end;",
			"",
			"end.",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	providerPath := filePaths["synautil.pas"]
	if err := client.OpenFile(ctx, providerPath); err != nil {
		t.Fatalf("falha ao abrir provider synautil para definition qualificada: %v", err)
	}

	result, err := ReadDefinition(ctx, client, "synautil.TimeZoneBias")
	if err != nil {
		t.Fatalf("ReadDefinition nao deveria falhar ao cair para o provider Delphi leaf-only: %v", err)
	}

	if strings.Contains(result, "not found") {
		t.Fatalf("esperado localizar synautil.TimeZoneBias via declaracao leaf no provider aberto; obtido: %s", result)
	}

	if !strings.Contains(result, providerPath) {
		t.Fatalf("esperado definition apontar para o provider %s; obtido: %s", providerPath, result)
	}

	if !strings.Contains(result, "function TimeZoneBias: Integer;") {
		t.Fatalf("esperado definition incluir a declaracao leaf do provider para synautil.TimeZoneBias; obtido: %s", result)
	}
}

func TestDelphiOracle_InferSymbolLocationFromOpenFiles_QualifiedCreatePrefersOwnedDeclarationOverIrrelevantUse(t *testing.T) {
	fixtures := map[string]string{
		"main.pas": strings.Join([]string{
			"procedure Demo;",
			"begin",
			"  TOther.Create;",
			"end;",
			"",
			"constructor TTarget.Create;",
			"begin",
			"end;",
			"",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := client.OpenFile(ctx, filePaths["main.pas"]); err != nil {
		t.Fatalf("falha ao abrir fixture para fallback qualificado: %v", err)
	}

	location, found := inferSymbolLocationFromOpenFiles(client, "TTarget.Create")
	if !found {
		t.Fatal("esperado localizar TTarget.Create em arquivo aberto, mas nenhum candidato foi encontrado")
	}

	const expectedLine = 6
	if gotLine := int(location.Range.Start.Line) + 1; gotLine != expectedLine {
		t.Fatalf("esperado fallback localizar a declaracao qualificada TTarget.Create na linha %d, mas escolheu L%d:C%d", expectedLine, gotLine, location.Range.Start.Character+1)
	}
}

func TestDelphiOracle_InferDelphiSymbolLocation_RealSynautilTimeZoneBiasInOpenedExternalFile(t *testing.T) {
	realPath := resolveRealSynautilPath()
	if realPath == "" {
		t.Skip("synautil.pas real nao encontrado; defina MCP_REAL_SYNAUTIL_PATH para habilitar este characterization test")
	}

	content, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatalf("falha ao ler synautil.pas real %s: %v", realPath, err)
	}

	if !strings.Contains(strings.ToLower(string(content)), "timezonebias") {
		t.Fatalf("o arquivo real %s nao contem TimeZoneBias; ajuste MCP_REAL_SYNAUTIL_PATH para apontar para a unidade esperada", realPath)
	}

	client, _, cleanup := setupDelphiOracleFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := client.OpenFile(ctx, realPath); err != nil {
		t.Fatalf("falha ao abrir synautil.pas real no fake LSP: %v", err)
	}

	location, found := inferDelphiSymbolLocation(client, "synautil.TimeZoneBias")
	if !found {
		t.Fatalf("inferDelphiSymbolLocation nao localizou synautil.TimeZoneBias no arquivo real aberto %s; openFiles=%v", realPath, client.GetOpenFilesSnapshot())
	}

	if gotPath := filepath.Clean(location.URI.Path()); !strings.EqualFold(gotPath, filepath.Clean(realPath)) {
		t.Fatalf("esperado localizacao no arquivo real aberto %s, mas o helper retornou %s", realPath, gotPath)
	}

	lines := strings.Split(string(content), "\n")
	lineIndex := int(location.Range.Start.Line)
	if lineIndex < 0 || lineIndex >= len(lines) {
		t.Fatalf("helper retornou linha fora do arquivo real: L%d para %s com %d linhas", lineIndex+1, realPath, len(lines))
	}

	matchedLine := strings.TrimSpace(lines[lineIndex])
	if !strings.Contains(strings.ToLower(matchedLine), "timezonebias") {
		t.Fatalf("esperado linha caracterizada conter TimeZoneBias, mas obteve L%d: %q", lineIndex+1, matchedLine)
	}

	if !locationMatchesDelphiSymbol(location, "synautil.TimeZoneBias") {
		t.Fatalf("a localizacao inferida nao sustenta synautil.TimeZoneBias em %s L%d:C%d: %q", realPath, lineIndex+1, location.Range.Start.Character+1, matchedLine)
	}
}

func TestDelphiOracle_ReadDefinition_RealSynautilTimeZoneBiasInOpenedExternalFile_FallsBackWithoutWorkspaceSymbolProvider(t *testing.T) {
	realPath := resolveRealSynautilPath()
	if realPath == "" {
		t.Skip("synautil.pas real nao encontrado; defina MCP_REAL_SYNAUTIL_PATH para habilitar esta regressao")
	}

	content, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatalf("falha ao ler synautil.pas real %s: %v", realPath, err)
	}

	if !strings.Contains(strings.ToLower(string(content)), "timezonebias") {
		t.Fatalf("o arquivo real %s nao contem TimeZoneBias; ajuste MCP_REAL_SYNAUTIL_PATH para apontar para a unidade esperada", realPath)
	}

	client, _, cleanup := setupDelphiOracleFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := client.OpenFile(ctx, realPath); err != nil {
		t.Fatalf("falha ao abrir synautil.pas real no fake LSP: %v", err)
	}

	location, found := inferDelphiSymbolLocation(client, "synautil.TimeZoneBias")
	if !found {
		t.Fatalf("precondicao invalida: inferDelphiSymbolLocation nao localizou synautil.TimeZoneBias no arquivo real aberto %s; openFiles=%v", realPath, client.GetOpenFilesSnapshot())
	}

	if gotPath := filepath.Clean(location.URI.Path()); !strings.EqualFold(gotPath, filepath.Clean(realPath)) {
		t.Fatalf("precondicao invalida: esperado localizacao no arquivo real aberto %s, mas o helper retornou %s", realPath, gotPath)
	}

	result, err := ReadDefinition(ctx, client, "synautil.TimeZoneBias")
	if err != nil {
		t.Fatalf("ReadDefinition nao deveria falhar ao sustentar synautil.TimeZoneBias via fallback Delphi sem workspace/symbol: %v", err)
	}

	if strings.Contains(result, "synautil.TimeZoneBias not found") {
		t.Fatalf("esperado fallback Delphi sustentar synautil.TimeZoneBias quando synautil.pas real esta aberto sem workspace/symbol; obtido: %s", result)
	}

	if !strings.Contains(result, realPath) {
		t.Fatalf("esperado definition apontar para o arquivo real aberto %s; obtido: %s", realPath, result)
	}

	if !strings.Contains(strings.ToLower(result), "timezonebias") {
		t.Fatalf("esperado definition incluir TimeZoneBias ao cair no fallback Delphi do arquivo real aberto; obtido: %s", result)
	}
}

func TestDelphiOracle_FindReferences_RealWorkspace_CharacterizesRawPayloadAndFilteredResult(t *testing.T) {
	backendPath := resolveRealOracleLSPPath()
	if backendPath == "" {
		t.Skip("oracle-lsp.exe real nao encontrado; defina MCP_REAL_ORACLE_LSP_PATH para habilitar este characterization test")
	}

	workspaceRoot := resolveRealDelphiWorkspaceRoot()
	if workspaceRoot == "" {
		t.Skip("workspace real nao encontrado; defina MCP_REAL_DELPHI_WORKSPACE para habilitar este characterization test")
	}

	testCases := []struct {
		name               string
		symbol             string
		anchorRelativePath string
		anchorMustContain  string
	}{
		{
			name:               "square",
			symbol:             "square",
			anchorRelativePath: filepath.Join("jvcl", "jvcl", "examples", "JvInterpreterDemos", "JvInterpreterTest", "samples", "sample - long loop.pas"),
			anchorMustContain:  "function square(P: Integer): Integer;",
		},
		{
			name:               "qualified_tblocksocket_create",
			symbol:             "TBlockSocket.Create",
			anchorRelativePath: filepath.Join("synapse", "blcksock.pas"),
			anchorMustContain:  "constructor TBlockSocket.Create;",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			anchorPath := filepath.Join(workspaceRoot, testCase.anchorRelativePath)
			ensureRealDelphiAnchorFile(t, anchorPath, testCase.anchorMustContain)

			client, cleanup := setupDelphiOracleRealClient(t, backendPath, workspaceRoot)
			defer cleanup()

			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()

			if err := client.OpenFile(ctx, anchorPath); err != nil {
				t.Fatalf("falha ao abrir arquivo ancora real %s para %s: %v", anchorPath, testCase.symbol, err)
			}

			symbolLocations, rawLocations, err := captureRawReferencePayload(ctx, client, testCase.symbol)
			if err != nil {
				t.Fatalf("falha ao capturar payload cru de textDocument/references para %s: %v", testCase.symbol, err)
			}

			result, err := FindReferences(ctx, client, testCase.symbol)
			if err != nil {
				t.Fatalf("FindReferences nao deveria falhar no workspace real para %s: %v", testCase.symbol, err)
			}

			filteredRawLocations := postProcessDelphiReferenceLocations(rawLocations)

			t.Logf("symbol=%s anchor=%s symbol_locations=%d %s", testCase.symbol, anchorPath, len(symbolLocations), summarizeReferenceLocations(symbolLocations, 4, 4))
			t.Logf("symbol=%s raw_payload_locations=%d %s", testCase.symbol, len(rawLocations), summarizeReferenceLocations(rawLocations, 8, 6))
			t.Logf("symbol=%s raw_payload_filtered=%d %s", testCase.symbol, len(filteredRawLocations), summarizeReferenceLocations(filteredRawLocations, 8, 6))
			t.Logf("symbol=%s filtered_result=%s", testCase.symbol, summarizeFilteredReferenceResult(result))

			if len(symbolLocations) == 0 {
				t.Fatalf("esperado localizar ao menos uma declaracao alvo para %s no workspace real %s", testCase.symbol, workspaceRoot)
			}

			if len(filteredRawLocations) > 0 && strings.Contains(result, "No references found") {
				t.Fatalf("payload cru filtrado ainda contem %d locations para %s, mas o resultado final ficou vazio: %s", len(filteredRawLocations), testCase.symbol, result)
			}
		})
	}
}

func TestDelphiOracle_ReadDefinition_QualifiedQueryWithoutOwnedDeclaration_ReturnsNotFoundInsteadOfLeafFalsePositive(t *testing.T) {
	fixtures := map[string]string{
		"main.pas": strings.Join([]string{
			"constructor TFont.Create;",
			"begin",
			"end;",
			"",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := client.OpenFile(ctx, filePaths["main.pas"]); err != nil {
		t.Fatalf("falha ao abrir fixture para false positive de definition qualificado: %v", err)
	}

	result, err := ReadDefinition(ctx, client, "TBlockSocket.Create")
	if err != nil {
		t.Fatalf("ReadDefinition nao deveria falhar quando o fallback nao encontra owner qualificado: %v", err)
	}

	if !strings.Contains(result, "TBlockSocket.Create not found") {
		t.Fatalf("consulta qualificada sem declaracao propria deve retornar not found em vez de casar o leaf Create errado; obtido: %s", result)
	}
}

func TestDelphiOracle_FindReferences_FallbackWhenWorkspaceSymbolUnavailable(t *testing.T) {
	client, filePath, cleanup := setupDelphiOracleFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture para references: %v", err)
	}

	result, err := FindReferences(ctx, client, "FindCustomer")
	if err != nil {
		t.Fatalf("esperado fallback em references sem workspace/symbol, mas recebeu erro: %v", err)
	}

	if !strings.Contains(result, "References in File") {
		t.Fatalf("esperado resultado de references com bloco de arquivo, obtido: %s", result)
	}
}

func TestDelphiOracle_FindReferences_QualifiedQueryFallsBackWhenWorkspaceSymbolResultsAreIrrelevant(t *testing.T) {
	t.Setenv(fakeLSPWorkspaceSymbolProviderEnv, "1")
	t.Setenv(fakeLSPWorkspaceSymbolIrrelevantEnv, "1")

	fixtures := map[string]string{
		"synautil_references.pas": strings.Join([]string{
			"function TimeZoneBias: Integer;",
			"begin",
			"  Result := synautil.TimeZoneBias;",
			"end;",
			"",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	filePath := filePaths["synautil_references.pas"]
	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture qualificada para references: %v", err)
	}

	result, err := FindReferences(ctx, client, "synautil.TimeZoneBias")
	if err != nil {
		t.Fatalf("FindReferences nao deveria falhar quando workspace/symbol retorna resultado qualificado irrelevante: %v", err)
	}

	if strings.Contains(result, "No references found") {
		t.Fatalf("esperado fallback Delphi apos filtragem de workspace/symbol irrelevante para synautil.TimeZoneBias; obtido: %s", result)
	}

	if !strings.Contains(result, "References in File") {
		t.Fatalf("esperado bloco de arquivo apos fallback Delphi para synautil.TimeZoneBias; obtido: %s", result)
	}

	if !strings.Contains(result, filePath) {
		t.Fatalf("esperado references apontar para a fixture aberta %s; obtido: %s", filePath, result)
	}
}

func TestDelphiOracle_FindReferences_QualifiedQueryFallsBackToProviderDeclarationWithoutQualifiedLiteral(t *testing.T) {
	t.Setenv(fakeLSPWorkspaceSymbolProviderEnv, "1")
	t.Setenv(fakeLSPWorkspaceSymbolIrrelevantEnv, "1")

	fixtures := map[string]string{
		"synautil.pas": strings.Join([]string{
			"unit synautil;",
			"",
			"interface",
			"",
			"function TimeZoneBias: Integer;",
			"",
			"implementation",
			"",
			"function TimeZoneBias: Integer;",
			"begin",
			"  Result := 180;",
			"end;",
			"",
			"end.",
		}, "\n"),
		"consumer.pas": strings.Join([]string{
			"unit SynautilConsumer;",
			"",
			"interface",
			"",
			"uses synautil;",
			"",
			"procedure Run;",
			"",
			"implementation",
			"",
			"procedure Run;",
			"var",
			"  Value: Integer;",
			"begin",
			"  Value := synautil.TimeZoneBias;",
			"end;",
			"",
			"end.",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	providerPath := filePaths["synautil.pas"]
	consumerPath := filePaths["consumer.pas"]
	if err := client.OpenFile(ctx, providerPath); err != nil {
		t.Fatalf("falha ao abrir provider synautil para references qualificadas: %v", err)
	}

	result, err := FindReferences(ctx, client, "synautil.TimeZoneBias")
	if err != nil {
		t.Fatalf("FindReferences nao deveria falhar ao cair para provider Delphi leaf-only: %v", err)
	}

	if strings.Contains(result, "No references found") {
		t.Fatalf("esperado references localizar provider+consumer para synautil.TimeZoneBias sem owner.member literal na declaracao; obtido: %s", result)
	}

	if !strings.Contains(result, providerPath) {
		t.Fatalf("esperado references incluir o provider %s; obtido: %s", providerPath, result)
	}

	if !strings.Contains(result, consumerPath) {
		t.Fatalf("esperado references incluir o consumer %s; obtido: %s", consumerPath, result)
	}
}

func TestDelphiOracle_FindReferences_SimpleSquareInOpenUnit_ReturnsNonEmptyResult(t *testing.T) {
	fixtures := map[string]string{
		"math_unit.pas": strings.Join([]string{
			"unit MathUnit;",
			"",
			"interface",
			"",
			"function square(Value: Integer): Integer;",
			"",
			"implementation",
			"",
			"function square(Value: Integer): Integer;",
			"begin",
			"  Result := Value * Value;",
			"end;",
			"",
			"procedure Demo;",
			"var",
			"  Current: Integer;",
			"begin",
			"  Current := square(3);",
			"end;",
			"",
			"end.",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	filePath := filePaths["math_unit.pas"]
	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir unit Delphi com square para references: %v", err)
	}

	result, err := FindReferences(ctx, client, "square")
	if err != nil {
		t.Fatalf("FindReferences nao deveria falhar para simbolo simples square em unit Delphi aberta: %v", err)
	}

	if strings.Contains(result, "No references found") {
		t.Fatalf("esperado references nao vazio para simbolo simples square em unit Delphi aberta; resultado: %s", result)
	}

	if !strings.Contains(result, "References in File") {
		t.Fatalf("esperado bloco de arquivo no resultado de references para square; obtido: %s", result)
	}

	if !strings.Contains(result, filePath) {
		t.Fatalf("esperado references apontar para a unit Delphi aberta %s; obtido: %s", filePath, result)
	}
}

func TestDelphiOracle_FindReferences_SimpleSquareInOpenUnit_FallsBackWhenWorkspaceSymbolReturnsEmpty(t *testing.T) {
	t.Setenv(fakeLSPWorkspaceSymbolProviderEnv, "1")
	t.Setenv(fakeLSPWorkspaceSymbolEmptyEnv, "1")

	fixtures := map[string]string{
		"math_unit.pas": strings.Join([]string{
			"unit MathUnit;",
			"",
			"interface",
			"",
			"function square(Value: Integer): Integer;",
			"",
			"implementation",
			"",
			"function square(Value: Integer): Integer;",
			"begin",
			"  Result := Value * Value;",
			"end;",
			"",
			"procedure Demo;",
			"var",
			"  Current: Integer;",
			"begin",
			"  Current := square(3);",
			"end;",
			"",
			"end.",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	filePath := filePaths["math_unit.pas"]
	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir unit Delphi com square para fallback de references: %v", err)
	}

	result, err := FindReferences(ctx, client, "square")
	if err != nil {
		t.Fatalf("FindReferences nao deveria falhar quando workspace/symbol retorna vazio para square: %v", err)
	}

	if strings.Contains(result, "No references found") {
		t.Fatalf("esperado fallback por arquivo aberto quando workspace/symbol retorna vazio para square; resultado: %s", result)
	}

	if !strings.Contains(result, "References in File") {
		t.Fatalf("esperado bloco de arquivo apos fallback de references para square; obtido: %s", result)
	}

	if !strings.Contains(result, filePath) {
		t.Fatalf("esperado references apontar para a unit Delphi aberta %s apos fallback; obtido: %s", filePath, result)
	}
}

func TestDelphiOracle_FindReferences_SquareInLongLoopSample_FallsBackWhenWorkspaceSymbolResultsAreIrrelevant(t *testing.T) {
	t.Setenv(fakeLSPWorkspaceSymbolProviderEnv, "1")
	t.Setenv(fakeLSPWorkspaceSymbolIrrelevantEnv, "1")

	fixtures := map[string]string{
		"sample - long loop.pas": strings.Join([]string{
			"function square(Value: Integer): Integer;",
			"begin",
			"  Result := square(Value - 1);",
			"end;",
			"",
			"procedure RunLongLoop;",
			"var",
			"  Index: Integer;",
			"begin",
			"  for Index := 1 to 100 do",
			"    WriteLn(square(Index));",
			"end;",
			"",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	filePath := filePaths["sample - long loop.pas"]
	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture do sample realista para square: %v", err)
	}

	result, err := FindReferences(ctx, client, "square")
	if err != nil {
		t.Fatalf("FindReferences nao deveria falhar para square quando workspace/symbol retorna resultados irrelevantes: %v", err)
	}

	if strings.Contains(result, "No references found") {
		t.Fatalf("esperado fallback Delphi para square quando workspace/symbol retorna resultados filtrados; obtido: %s", result)
	}

	if !strings.Contains(result, "References in File") {
		t.Fatalf("esperado bloco de arquivo no fallback de references para square; obtido: %s", result)
	}

	if !strings.Contains(result, filePath) {
		t.Fatalf("esperado references apontar para o sample Delphi aberto %s; obtido: %s", filePath, result)
	}
}

func TestDelphiOracle_FindReferences_SquareInLongLoopSampleWithoutOpenFiles_PrefersSampleAndIgnoresCommentNoise(t *testing.T) {
	t.Setenv(fakeLSPWorkspaceSymbolProviderEnv, "1")
	t.Setenv(fakeLSPWorkspaceSymbolIrrelevantEnv, "1")

	fixtures := map[string]string{
		"sample - long loop.pas": strings.Join([]string{
			"function square(Value: Integer): Integer;",
			"begin",
			"  Result := Value * Value;",
			"end;",
			"",
			"procedure RunLongLoop;",
			"var",
			"  Index: Integer;",
			"begin",
			"  for Index := 1 to 100 do",
			"    WriteLn(square(Index));",
			"end;",
		}, "\n"),
		"noise-comments.pas": strings.Join([]string{
			"unit NoiseComments;",
			"",
			"interface",
			"",
			"implementation",
			"",
			"{ square textures should not count as a Delphi reference }",
			"(* square cache entries also should not count *)",
			"",
			"end.",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	samplePath := filePaths["sample - long loop.pas"]
	noisePath := filePaths["noise-comments.pas"]

	result, err := FindReferences(ctx, client, "square")
	if err != nil {
		t.Fatalf("FindReferences nao deveria falhar para square sem arquivos abertos no sample Delphi: %v", err)
	}

	if strings.Contains(result, "No references found") {
		t.Fatalf("esperado references localizar declaracao+uso de square no sample Delphi mesmo sem arquivo aberto; obtido: %s", result)
	}

	if !strings.Contains(result, samplePath) {
		t.Fatalf("esperado references priorizar o sample Delphi %s; obtido: %s", samplePath, result)
	}

	if !strings.Contains(result, "L1:C10") || !strings.Contains(result, "L11:C13") {
		t.Fatalf("esperado references incluir ao menos declaracao e uso de square no sample; obtido: %s", result)
	}

	if strings.Contains(result, noisePath) || strings.Contains(result, "square textures") || strings.Contains(result, "square cache entries") {
		t.Fatalf("comentarios textuais irrelevantes nao devem contar como sucesso para square; obtido: %s", result)
	}
}

func TestDelphiOracle_FindReferences_LSPPayloadFiltersCommentDuplicatesAndRanksExecutableForSquare(t *testing.T) {
	t.Setenv("LSP_CONTEXT_LINES", "0")
	setFakeDelphiOracleReferencePayload(t, []fakeDelphiOracleReferenceRange{
		{StartLine: 4, StartCharacter: 9, EndLine: 4, EndCharacter: 15},
		{StartLine: 8, StartCharacter: 2, EndLine: 8, EndCharacter: 8},
		{StartLine: 9, StartCharacter: 3, EndLine: 9, EndCharacter: 9},
		{StartLine: 10, StartCharacter: 3, EndLine: 10, EndCharacter: 9},
		{StartLine: 21, StartCharacter: 29, EndLine: 21, EndCharacter: 35},
		{StartLine: 21, StartCharacter: 17, EndLine: 21, EndCharacter: 23},
	})

	fixtures := map[string]string{
		"math_unit.pas": strings.Join([]string{
			"unit MathUnit;",
			"",
			"interface",
			"",
			"function square(Value: Integer): Integer;",
			"",
			"implementation",
			"",
			"{ square in brace comment should be ignored }",
			"(* square in paren comment should also be ignored *)",
			"// square in slash comment should also be ignored",
			"",
			"function square(Value: Integer): Integer;",
			"begin",
			"  Result := Value * Value;",
			"end;",
			"",
			"procedure Demo;",
			"var",
			"  Accumulator: Integer;",
			"begin",
			"  Accumulator := square(3) + square(4);",
			"end;",
			"",
			"end.",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	filePath := filePaths["math_unit.pas"]
	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture square para payload cru de references: %v", err)
	}

	result, err := FindReferences(ctx, client, "square")
	if err != nil {
		t.Fatalf("FindReferences nao deveria falhar para payload cru do LSP com square: %v", err)
	}

	if !strings.Contains(result, "References in File: 2") {
		t.Fatalf("esperado manter apenas declaracao e linha executavel apos filtro/dedup do payload cru; obtido: %s", result)
	}

	if strings.Contains(result, "brace comment should be ignored") || strings.Contains(result, "paren comment should also be ignored") || strings.Contains(result, "slash comment should also be ignored") {
		t.Fatalf("comentarios Delphi retornados pelo LSP nao devem sobreviver ao resultado final; obtido: %s", result)
	}

	if gotLines := extractReferenceLinesFromResult(t, result); !sameIntSlice(gotLines, []int{22, 5}) {
		t.Fatalf("esperado ranquear linha executavel antes da declaracao e deduplicar a mesma linha em square; linhas obtidas: %v; resultado: %s", gotLines, result)
	}
}

func TestDelphiOracle_FindReferences_LSPPayloadFiltersCommentDuplicatesAndRanksExecutableForQualifiedTShapeSquare(t *testing.T) {
	t.Setenv("LSP_CONTEXT_LINES", "0")
	setFakeDelphiOracleReferencePayload(t, []fakeDelphiOracleReferenceRange{
		{StartLine: 12, StartCharacter: 16, EndLine: 12, EndCharacter: 22},
		{StartLine: 25, StartCharacter: 11, EndLine: 25, EndCharacter: 17},
		{StartLine: 26, StartCharacter: 3, EndLine: 26, EndCharacter: 16},
		{StartLine: 22, StartCharacter: 33, EndLine: 22, EndCharacter: 39},
		{StartLine: 22, StartCharacter: 16, EndLine: 22, EndCharacter: 22},
	})

	fixtures := map[string]string{
		"shapes.pas": strings.Join([]string{
			"unit Shapes;",
			"",
			"interface",
			"",
			"type",
			"  TShape = class",
			"  public",
			"    function square(Value: Integer): Integer;",
			"  end;",
			"",
			"implementation",
			"",
			"function TShape.square(Value: Integer): Integer;",
			"begin",
			"  Result := Value * Value;",
			"end;",
			"",
			"procedure Demo;",
			"var",
			"  Shape: TShape;",
			"begin",
			"  Shape := TShape.Create;",
			"  WriteLn(Shape.square(5) + Shape.square(6));",
			"end;",
			"",
			"{ square comment noise should not survive for TShape.square }",
			"// TOtherShape.square should not leak back into the output",
			"",
			"end.",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	filePath := filePaths["shapes.pas"]
	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture TShape.square para payload cru de references: %v", err)
	}

	result, err := FindReferences(ctx, client, "TShape.square")
	if err != nil {
		t.Fatalf("FindReferences nao deveria falhar para payload cru qualificado do LSP: %v", err)
	}

	if !strings.Contains(result, "References in File: 2") {
		t.Fatalf("esperado manter apenas implementacao e linha executavel qualificadas apos filtro/dedup; obtido: %s", result)
	}

	if strings.Contains(result, "comment noise should not survive") || strings.Contains(result, "TOtherShape.square should not leak") {
		t.Fatalf("comentarios Delphi nao devem reabrir ruido no caso qualificado TShape.square; obtido: %s", result)
	}

	if gotLines := extractReferenceLinesFromResult(t, result); !sameIntSlice(gotLines, []int{23, 13}) {
		t.Fatalf("esperado ranquear a linha executavel antes da implementacao qualificada e deduplicar a mesma linha em TShape.square; linhas obtidas: %v; resultado: %s", gotLines, result)
	}
}

func TestDelphiOracle_GetFullDefinition_FallbackWhenDocumentSymbolUnavailable(t *testing.T) {
	client, filePath, cleanup := setupDelphiOracleFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture para GetFullDefinition: %v", err)
	}

	loc := protocol.Location{
		URI: protocol.URIFromPath(filePath),
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 11},
			End:   protocol.Position{Line: 0, Character: 23},
		},
	}

	definition, _, err := GetFullDefinition(ctx, client, loc)
	if err != nil {
		t.Fatalf("esperado fallback de GetFullDefinition sem documentSymbol, mas recebeu erro: %v", err)
	}

	if !strings.Contains(definition, "FindCustomer") {
		t.Fatalf("esperado definition contendo FindCustomer, obtido: %s", definition)
	}
}

func TestDelphiOracle_GetFullDefinition_InterfaceFreeRoutinePrefersImplementationBlock(t *testing.T) {
	fixtures := map[string]string{
		"synautil.pas": strings.Join([]string{
			"unit synautil;",
			"",
			"interface",
			"",
			"function TimeZoneBias: Integer;",
			"",
			"implementation",
			"",
			"function TimeZoneBias: Integer;",
			"begin",
			"  Result := 180;",
			"end;",
			"",
			"end.",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	filePath := filePaths["synautil.pas"]
	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture synautil para GetFullDefinition: %v", err)
	}

	loc := protocol.Location{
		URI: protocol.URIFromPath(filePath),
		Range: protocol.Range{
			Start: protocol.Position{Line: 4, Character: 9},
			End:   protocol.Position{Line: 4, Character: 21},
		},
	}

	definition, fullLocation, err := GetFullDefinition(ctx, client, loc)
	if err != nil {
		t.Fatalf("GetFullDefinition nao deveria falhar ao partir da declaracao de interface TimeZoneBias: %v", err)
	}

	if gotLine := int(fullLocation.Range.Start.Line) + 1; gotLine != 9 {
		t.Fatalf("esperado GetFullDefinition reposicionar para o header da implementacao TimeZoneBias na linha 9, mas retornou L%d com trecho: %s", gotLine, definition)
	}

	if !strings.Contains(definition, "function TimeZoneBias: Integer;") || !strings.Contains(definition, "Result := 180;") || !strings.Contains(definition, "end;") {
		t.Fatalf("esperado bloco completo da implementacao TimeZoneBias, com header e corpo, obtido: %s", definition)
	}
}

func TestDelphiOracle_GetFullDefinition_InterfaceClassConstructorPrefersQualifiedImplementationBlock(t *testing.T) {
	fixtures := map[string]string{
		"clamsend.pas": strings.Join([]string{
			"unit clamsend;",
			"",
			"interface",
			"",
			"type",
			"  TClamSend = class",
			"  public",
			"    constructor Create;",
			"  end;",
			"",
			"implementation",
			"",
			"constructor TClamSend.Create;",
			"begin",
			"  Value := 1;",
			"end;",
			"",
			"end.",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	filePath := filePaths["clamsend.pas"]
	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture clamsend para GetFullDefinition: %v", err)
	}

	loc := protocol.Location{
		URI: protocol.URIFromPath(filePath),
		Range: protocol.Range{
			Start: protocol.Position{Line: 7, Character: 16},
			End:   protocol.Position{Line: 7, Character: 22},
		},
	}

	definition, fullLocation, err := GetFullDefinition(ctx, client, loc)
	if err != nil {
		t.Fatalf("GetFullDefinition nao deveria falhar ao partir da declaracao de interface do construtor Create: %v", err)
	}

	if gotLine := int(fullLocation.Range.Start.Line) + 1; gotLine != 13 {
		t.Fatalf("esperado GetFullDefinition reposicionar para o header qualificado da implementacao na linha 13, mas retornou L%d com trecho: %s", gotLine, definition)
	}

	if !strings.Contains(definition, "constructor TClamSend.Create;") || !strings.Contains(definition, "Value := 1;") || !strings.Contains(definition, "end;") {
		t.Fatalf("esperado bloco completo da implementacao qualificada do construtor Create, obtido: %s", definition)
	}
}

func TestDelphiOracle_GetFullDefinition_ImplementationFreeRoutineHeaderExpandsToCompleteBlock(t *testing.T) {
	fixtures := map[string]string{
		"synautil.pas": strings.Join([]string{
			"unit synautil;",
			"",
			"interface",
			"",
			"function TimeZoneBias: Integer;",
			"",
			"implementation",
			"",
			"function TimeZoneBias: Integer;",
			"begin",
			"  Result := 180;",
			"end;",
			"",
			"end.",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	filePath := filePaths["synautil.pas"]
	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture synautil para GetFullDefinition no header da implementacao: %v", err)
	}

	loc := protocol.Location{
		URI: protocol.URIFromPath(filePath),
		Range: protocol.Range{
			Start: protocol.Position{Line: 8, Character: 9},
			End:   protocol.Position{Line: 8, Character: 21},
		},
	}

	definition, fullLocation, err := GetFullDefinition(ctx, client, loc)
	if err != nil {
		t.Fatalf("GetFullDefinition nao deveria falhar ao partir do header da implementacao TimeZoneBias: %v", err)
	}

	if gotLine := int(fullLocation.Range.Start.Line) + 1; gotLine != 9 {
		t.Fatalf("esperado GetFullDefinition permanecer no header da implementacao TimeZoneBias na linha 9, mas retornou L%d com trecho: %s", gotLine, definition)
	}

	if gotEndLine := int(fullLocation.Range.End.Line) + 1; gotEndLine <= 9 {
		t.Fatalf("esperado GetFullDefinition expandir o header da implementacao TimeZoneBias para o corpo completo, mas o range terminou em L%d com trecho: %s", gotEndLine, definition)
	}

	if !strings.Contains(definition, "function TimeZoneBias: Integer;") || !strings.Contains(definition, "Result := 180;") || !strings.Contains(definition, "end;") {
		t.Fatalf("esperado bloco completo da implementacao TimeZoneBias ao partir do proprio header, obtido: %s", definition)
	}
}

func TestDelphiOracle_GetFullDefinition_ImplementationQualifiedConstructorHeaderExpandsToCompleteBlock(t *testing.T) {
	fixtures := map[string]string{
		"blcksock.pas": strings.Join([]string{
			"unit blcksock;",
			"",
			"interface",
			"",
			"type",
			"  TBlockSocket = class",
			"  public",
			"    constructor Create;",
			"  end;",
			"",
			"implementation",
			"",
			"constructor TBlockSocket.Create;",
			"begin",
			"  Sock := 1;",
			"end;",
			"",
			"end.",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	filePath := filePaths["blcksock.pas"]
	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture blcksock para GetFullDefinition no header da implementacao qualificada: %v", err)
	}

	loc := protocol.Location{
		URI: protocol.URIFromPath(filePath),
		Range: protocol.Range{
			Start: protocol.Position{Line: 12, Character: 25},
			End:   protocol.Position{Line: 12, Character: 31},
		},
	}

	definition, fullLocation, err := GetFullDefinition(ctx, client, loc)
	if err != nil {
		t.Fatalf("GetFullDefinition nao deveria falhar ao partir do header da implementacao qualificada TBlockSocket.Create: %v", err)
	}

	if gotLine := int(fullLocation.Range.Start.Line) + 1; gotLine != 13 {
		t.Fatalf("esperado GetFullDefinition permanecer no header qualificado da implementacao TBlockSocket.Create na linha 13, mas retornou L%d com trecho: %s", gotLine, definition)
	}

	if gotEndLine := int(fullLocation.Range.End.Line) + 1; gotEndLine <= 13 {
		t.Fatalf("esperado GetFullDefinition expandir o header qualificado TBlockSocket.Create para o corpo completo, mas o range terminou em L%d com trecho: %s", gotEndLine, definition)
	}

	if !strings.Contains(definition, "constructor TBlockSocket.Create;") || !strings.Contains(definition, "Sock := 1;") || !strings.Contains(definition, "end;") {
		t.Fatalf("esperado bloco completo da implementacao qualificada TBlockSocket.Create ao partir do proprio header, obtido: %s", definition)
	}
}

func TestDelphiOracle_GetFullDefinition_InterfaceFreeRoutineWithPreprocessorBranchesPrefersImplementationBlock(t *testing.T) {
	fixtures := map[string]string{
		"synautil.pas": strings.Join([]string{
			"unit synautil;",
			"",
			"interface",
			"",
			"function TimeZoneBias: Integer;",
			"",
			"implementation",
			"",
			"function TimeZoneBias: Integer;",
			"{$IFNDEF MSWINDOWS}",
			"{$IFNDEF FPC}",
			"var",
			"  Bias: Integer;",
			"begin",
			"  Bias := 180;",
			"  Result := Bias;",
			"{$ELSE}",
			"begin",
			"  Result := 240;",
			"{$ENDIF}",
			"{$ELSE}",
			"var",
			"  AltBias: Integer;",
			"begin",
			"  case AltBias of",
			"    0: Result := 0;",
			"  else",
			"    Result := -60;",
			"  end;",
			"{$ENDIF}",
			"end;",
			"",
			"end.",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	filePath := filePaths["synautil.pas"]
	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture synautil com preprocessor para GetFullDefinition: %v", err)
	}

	loc := protocol.Location{
		URI: protocol.URIFromPath(filePath),
		Range: protocol.Range{
			Start: protocol.Position{Line: 4, Character: 9},
			End:   protocol.Position{Line: 4, Character: 21},
		},
	}

	definition, fullLocation, err := GetFullDefinition(ctx, client, loc)
	if err != nil {
		t.Fatalf("GetFullDefinition nao deveria falhar ao promover TimeZoneBias da interface para a implementacao com branches condicionais: %v", err)
	}

	if gotLine := int(fullLocation.Range.Start.Line) + 1; gotLine != 9 {
		t.Fatalf("esperado GetFullDefinition promover TimeZoneBias para o header da implementacao na linha 9, mas retornou L%d com trecho: %s", gotLine, definition)
	}

	if gotEndLine := int(fullLocation.Range.End.Line) + 1; gotEndLine <= 9 {
		t.Fatalf("esperado GetFullDefinition expandir TimeZoneBias com preprocessor ate o fim do bloco de implementacao, mas o range terminou em L%d com trecho: %s", gotEndLine, definition)
	}

	if !strings.Contains(definition, "function TimeZoneBias: Integer;") || !strings.Contains(definition, "Bias := 180;") || !strings.Contains(definition, "Result := 240;") || !strings.Contains(definition, "case AltBias of") || !strings.Contains(definition, "end;") {
		t.Fatalf("esperado bloco completo da implementacao TimeZoneBias com branches condicionais, obtido: %s", definition)
	}
}

func TestDelphiOracle_RenameSymbol_ErrorClaroQuandoRenameProviderAusente(t *testing.T) {
	client, filePath, cleanup := setupDelphiOracleFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	_, err := RenameSymbol(ctx, client, filePath, 1, 11, "FindCustomerNew")
	if err == nil {
		t.Fatal("esperado erro claro para rename sem renameProvider, mas nao houve erro")
	}

	if !strings.Contains(strings.ToLower(err.Error()), "renameprovider") {
		t.Fatalf("esperado erro mencionar renameProvider ausente, obtido: %v", err)
	}
}

func TestDelphiOracle_Diagnostics_UsaCachePublishDiagnosticsQuandoDocumentDiagnosticFalha(t *testing.T) {
	client, filePath, cleanup := setupDelphiOracleFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture para diagnostics: %v", err)
	}

	result, err := GetDiagnosticsForFile(ctx, client, filePath, 1, true)
	if err != nil {
		t.Fatalf("GetDiagnosticsForFile nao deveria falhar com cache publishDiagnostics: %v", err)
	}

	if !strings.Contains(result, "Undeclared identifier") {
		t.Fatalf("esperado diagnostico vindo do cache publishDiagnostics, obtido: %s", result)
	}
}

func setupDelphiOracleFakeClient(t *testing.T) (*lsp.Client, string, func()) {
	t.Helper()

	fixtures := map[string]string{
		"main.pas": strings.Join([]string{
			"procedure FindCustomer;",
			"begin",
			"  MissingIdentifier := 1;",
			"end;",
			"",
		}, "\n"),
	}

	client, filePaths, cleanup := setupDelphiOracleFakeClientWithFixtures(t, fixtures)
	return client, filePaths["main.pas"], cleanup
}

func resolveRealSynautilPath() string {
	candidates := make([]string, 0, 3)

	if configuredPath := strings.TrimSpace(os.Getenv("MCP_REAL_SYNAUTIL_PATH")); configuredPath != "" {
		candidates = append(candidates, configuredPath)
	}

	if userProfile := strings.TrimSpace(os.Getenv("USERPROFILE")); userProfile != "" {
		candidates = append(candidates,
			filepath.Join(userProfile, "Downloads", "Projetos Teste LSP", "synapse", "synautil.pas"),
			filepath.Join(userProfile, "Downloads", "synapse", "synautil.pas"),
		)
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return filepath.Clean(candidate)
		}
	}

	return ""
}

func resolveRealOracleLSPPath() string {
	candidates := make([]string, 0, 2)

	if configuredPath := strings.TrimSpace(os.Getenv("MCP_REAL_ORACLE_LSP_PATH")); configuredPath != "" {
		candidates = append(candidates, configuredPath)
	}

	candidates = append(candidates,
		filepath.Join("C:\\Users", "pedro", "Downloads", "Delphi_Oracle", "oracle-lsp", "target", "release", "oracle-lsp.exe"),
	)

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return filepath.Clean(candidate)
		}
	}

	return ""
}

func resolveRealDelphiWorkspaceRoot() string {
	candidates := make([]string, 0, 2)

	if configuredPath := strings.TrimSpace(os.Getenv("MCP_REAL_DELPHI_WORKSPACE")); configuredPath != "" {
		candidates = append(candidates, configuredPath)
	}

	candidates = append(candidates,
		filepath.Join("C:\\Users", "pedro", "Downloads", "Projetos Teste LSP"),
	)

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return filepath.Clean(candidate)
		}
	}

	return ""
}

func ensureRealDelphiAnchorFile(t *testing.T, anchorPath string, mustContain string) {
	t.Helper()

	content, err := os.ReadFile(anchorPath)
	if err != nil {
		t.Skipf("arquivo ancora real nao disponivel em %s: %v", anchorPath, err)
	}

	if !strings.Contains(string(content), mustContain) {
		t.Skipf("arquivo ancora real %s nao contem o trecho esperado %q; workspace real mudou", anchorPath, mustContain)
	}
}

func setupDelphiOracleRealClient(t *testing.T, backendPath string, workspaceRoot string) (*lsp.Client, func()) {
	t.Helper()

	client, err := lsp.NewClient(backendPath)
	if err != nil {
		t.Fatalf("falha ao iniciar oracle-lsp real %s: %v", backendPath, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, workspaceRoot); err != nil {
		_ = client.Close()
		t.Fatalf("falha ao inicializar oracle-lsp real com workspace %s: %v", workspaceRoot, err)
	}

	cleanup := func() {
		if err := client.Close(); err != nil {
			_ = err
		}
	}

	return client, cleanup
}

func captureRawReferencePayload(ctx context.Context, client *lsp.Client, symbolName string) ([]protocol.Location, []protocol.Location, error) {
	symbolLocations, err := resolveReferenceSymbolLocations(ctx, client, symbolName)
	if err != nil {
		return nil, nil, err
	}

	rawLocations := make([]protocol.Location, 0)
	for _, location := range symbolLocations {
		references, err := collectReferenceLocations(ctx, client, location)
		if err != nil {
			return nil, nil, err
		}

		rawLocations = appendUniqueReferenceLocations(rawLocations, references)
	}

	return symbolLocations, rawLocations, nil
}

func summarizeReferenceLocations(locations []protocol.Location, maxFiles int, maxLinesPerFile int) string {
	if len(locations) == 0 {
		return "nenhuma location"
	}

	linesByFile := make(map[string][]int)
	for _, location := range locations {
		path := filepath.Clean(location.URI.Path())
		linesByFile[path] = append(linesByFile[path], int(location.Range.Start.Line)+1)
	}

	files := make([]string, 0, len(linesByFile))
	for file := range linesByFile {
		files = append(files, file)
	}
	sort.Strings(files)

	parts := make([]string, 0, len(files))
	for index, file := range files {
		if maxFiles > 0 && index >= maxFiles {
			parts = append(parts, fmt.Sprintf("... +%d arquivo(s)", len(files)-index))
			break
		}

		lineNumbers := uniqueSortedLineNumbers(linesByFile[file])
		parts = append(parts, fmt.Sprintf("%s [%s]", file, summarizeLineNumbers(lineNumbers, maxLinesPerFile)))
	}

	return strings.Join(parts, " | ")
}

func summarizeFilteredReferenceResult(result string) string {
	trimmed := strings.TrimSpace(result)
	if trimmed == "" {
		return "resultado vazio"
	}

	if strings.HasPrefix(trimmed, "No references found") {
		return trimmed
	}

	parts := make([]string, 0)
	lastNonEmpty := ""
	currentFile := ""
	for _, line := range strings.Split(result, "\n") {
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "References in File:") {
			currentFile = lastNonEmpty
			continue
		}

		if strings.HasPrefix(trimmedLine, "At: ") {
			if currentFile == "" {
				currentFile = "<arquivo-desconhecido>"
			}
			parts = append(parts, fmt.Sprintf("%s [%s]", currentFile, strings.TrimPrefix(trimmedLine, "At: ")))
			currentFile = ""
			continue
		}

		if trimmedLine != "" && trimmedLine != "---" {
			lastNonEmpty = trimmedLine
		}
	}

	if len(parts) == 0 {
		return trimmed
	}

	return strings.Join(parts, " | ")
}

func uniqueSortedLineNumbers(lines []int) []int {
	if len(lines) == 0 {
		return nil
	}

	sorted := append([]int(nil), lines...)
	sort.Ints(sorted)

	result := sorted[:0]
	for _, line := range sorted {
		if len(result) == 0 || result[len(result)-1] != line {
			result = append(result, line)
		}
	}

	return result
}

func summarizeLineNumbers(lines []int, maxLines int) string {
	if len(lines) == 0 {
		return "sem linhas"
	}

	limit := len(lines)
	if maxLines > 0 && limit > maxLines {
		limit = maxLines
	}

	parts := make([]string, 0, limit+1)
	for _, line := range lines[:limit] {
		parts = append(parts, fmt.Sprintf("L%d", line))
	}

	if limit < len(lines) {
		parts = append(parts, fmt.Sprintf("+%d", len(lines)-limit))
	}

	return strings.Join(parts, ", ")
}

func setupDelphiOracleFakeClientWithFixtures(t *testing.T, fixtures map[string]string) (*lsp.Client, map[string]string, func()) {
	t.Helper()

	workspaceDir := t.TempDir()
	filePaths := make(map[string]string, len(fixtures))
	for fileName, content := range fixtures {
		filePath := filepath.Join(workspaceDir, fileName)
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("falha ao escrever fixture Delphi %s: %v", fileName, err)
		}
		filePaths[fileName] = filePath
	}

	t.Setenv(fakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("falha ao obter binario de teste: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessDelphiOracleFakeLSP")
	if err != nil {
		t.Fatalf("falha ao iniciar fake LSP: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		_ = client.Close()
		t.Fatalf("falha ao inicializar fake LSP: %v", err)
	}

	cleanup := func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	}

	return client, filePaths, cleanup
}

func runDelphiOracleFakeLSP(stdin *os.File, stdout *os.File) {
	reader := bufio.NewReader(stdin)
	writer := stdout
	hasWorkspaceSymbol := os.Getenv(fakeLSPWorkspaceSymbolProviderEnv) == "1"
	returnsEmptyWorkspaceSymbol := os.Getenv(fakeLSPWorkspaceSymbolEmptyEnv) == "1"
	returnsIrrelevantWorkspaceSymbol := os.Getenv(fakeLSPWorkspaceSymbolIrrelevantEnv) == "1"
	returnsMatchedButUnsustainedWorkspaceSymbol := os.Getenv(fakeLSPWorkspaceSymbolMatchedButUnsustainedEnv) == "1"
	returnsQualifiedTypeAmbiguousWorkspaceSymbol := os.Getenv(fakeLSPWorkspaceSymbolQualifiedTypeAmbiguousEnv) == "1"
	returnsQualifiedTypeNoStrongWorkspaceSymbol := os.Getenv(fakeLSPWorkspaceSymbolQualifiedTypeNoStrongEnv) == "1"
	customReferenceRanges := loadFakeDelphiOracleReferenceRanges()
	openedURI := ""
	workspaceRoot := ""

	for {
		msg, err := lsp.ReadMessage(reader)
		if err != nil {
			return
		}

		switch msg.Method {
		case "initialize":
			var params struct {
				RootPath string `json:"rootPath"`
				RootURI  string `json:"rootUri"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			workspaceRoot = params.RootPath
			if workspaceRoot == "" && params.RootURI != "" {
				if path, ok := uriPathFromString(params.RootURI); ok {
					workspaceRoot = path
				}
			}

			capabilities := map[string]any{
				"definitionProvider": true,
				"referencesProvider": true,
				"hoverProvider":      true,
			}
			if hasWorkspaceSymbol {
				capabilities["workspaceSymbolProvider"] = true
			}

			result := map[string]any{"capabilities": capabilities}
			sendFakeResponse(writer, msg.ID, result, nil)
		case "initialized":
			// no-op
		case "textDocument/didOpen":
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			openedURI = params.TextDocument.URI

			notif := &lsp.Message{
				JSONRPC: "2.0",
				Method:  "textDocument/publishDiagnostics",
				Params: mustMarshal(map[string]any{
					"uri": params.TextDocument.URI,
					"diagnostics": []map[string]any{
						{
							"range": map[string]any{
								"start": map[string]any{"line": 2, "character": 2},
								"end":   map[string]any{"line": 2, "character": 18},
							},
							"severity": 1,
							"message":  "Undeclared identifier: MissingIdentifier",
							"source":   "Delphi_Oracle",
						},
					},
				}),
			}
			_ = lsp.WriteMessage(writer, notif)
		case "workspace/symbol":
			if returnsEmptyWorkspaceSymbol {
				sendFakeResponse(writer, msg.ID, []map[string]any{}, nil)
				break
			}

			if returnsQualifiedTypeAmbiguousWorkspaceSymbol {
				var params struct {
					Query string `json:"query"`
				}
				_ = json.Unmarshal(msg.Params, &params)

				sendFakeResponse(writer, msg.ID, buildDelphiOracleQualifiedTypeAmbiguousWorkspaceSymbols(params.Query, openedURI), nil)
				break
			}

			if returnsQualifiedTypeNoStrongWorkspaceSymbol {
				var params struct {
					Query string `json:"query"`
				}
				_ = json.Unmarshal(msg.Params, &params)

				sendFakeResponse(writer, msg.ID, buildDelphiOracleQualifiedTypeNoStrongWorkspaceSymbols(params.Query, openedURI), nil)
				break
			}

			if returnsMatchedButUnsustainedWorkspaceSymbol {
				var params struct {
					Query string `json:"query"`
				}
				_ = json.Unmarshal(msg.Params, &params)

				sendFakeResponse(writer, msg.ID, buildDelphiOracleMatchedButUnsustainedWorkspaceSymbols(params.Query, workspaceRoot), nil)
				break
			}

			if returnsIrrelevantWorkspaceSymbol {
				var params struct {
					Query string `json:"query"`
				}
				_ = json.Unmarshal(msg.Params, &params)

				sendFakeResponse(writer, msg.ID, buildDelphiOracleIrrelevantWorkspaceSymbols(params.Query, openedURI), nil)
				break
			}

			sendFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32601, Message: "method not found: workspace/symbol"})
		case "textDocument/documentSymbol":
			sendFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32601, Message: "method not found: textDocument/documentSymbol"})
		case "textDocument/definition":
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &params)

			result := []map[string]any{
				{
					"uri": params.TextDocument.URI,
					"range": map[string]any{
						"start": map[string]any{"line": 0, "character": 10},
						"end":   map[string]any{"line": 0, "character": 22},
					},
				},
			}
			sendFakeResponse(writer, msg.ID, result, nil)
		case "textDocument/references":
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &params)

			if len(customReferenceRanges) > 0 {
				sendFakeResponse(writer, msg.ID, buildDelphiOracleReferencePayload(params.TextDocument.URI, customReferenceRanges), nil)
				break
			}

			result := []map[string]any{
				{
					"uri": params.TextDocument.URI,
					"range": map[string]any{
						"start": map[string]any{"line": 0, "character": 10},
						"end":   map[string]any{"line": 0, "character": 22},
					},
				},
				{
					"uri": params.TextDocument.URI,
					"range": map[string]any{
						"start": map[string]any{"line": 2, "character": 2},
						"end":   map[string]any{"line": 2, "character": 14},
					},
				},
			}
			sendFakeResponse(writer, msg.ID, result, nil)
		case "textDocument/rename":
			sendFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32601, Message: "method not found: textDocument/rename"})
		case "textDocument/diagnostic":
			sendFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32601, Message: "method not found: textDocument/diagnostic"})
		case "shutdown":
			sendFakeResponse(writer, msg.ID, nil, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendFakeResponse(writer, msg.ID, nil, nil)
			}
		}
	}
}

func sendFakeResponse(w *os.File, id *lsp.MessageID, result any, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
	}

	if rpcErr == nil {
		if result == nil {
			resp.Result = mustMarshal(struct{}{})
		} else {
			resp.Result = mustMarshal(result)
		}
	}

	_ = lsp.WriteMessage(w, resp)
}

func buildDelphiOracleIrrelevantWorkspaceSymbols(query string, openedURI string) []map[string]any {
	location := map[string]any{
		"uri": openedURI,
		"range": map[string]any{
			"start": map[string]any{"line": 0, "character": 0},
			"end":   map[string]any{"line": 0, "character": 10},
		},
	}

	if _, member, ok := splitQualifiedSymbolQuery(query); ok {
		return []map[string]any{
			{
				"name":          member,
				"kind":          12,
				"containerName": "IrrelevantOwner",
				"location":      location,
			},
		}
	}

	return []map[string]any{
		{
			"name":     query + "Helper",
			"kind":     12,
			"location": location,
		},
	}
}

func buildDelphiOracleMatchedButUnsustainedWorkspaceSymbols(query string, workspaceRoot string) []map[string]any {
	if strings.TrimSpace(workspaceRoot) == "" {
		workspaceRoot = os.TempDir()
	}

	location := map[string]any{
		"uri": string(protocol.URIFromPath(filepath.Join(workspaceRoot, "matched-but-unsustained-target.pas"))),
		"range": map[string]any{
			"start": map[string]any{"line": 0, "character": 0},
			"end":   map[string]any{"line": 0, "character": 10},
		},
	}

	if owner, member, ok := splitQualifiedSymbolQuery(query); ok {
		return []map[string]any{
			{
				"name":          member,
				"kind":          12,
				"containerName": owner,
				"location":      location,
			},
		}
	}

	return []map[string]any{
		{
			"name":     query,
			"kind":     12,
			"location": location,
		},
	}
}

func buildDelphiOracleQualifiedTypeAmbiguousWorkspaceSymbols(query string, openedURI string) []map[string]any {
	normalizedQuery := strings.ToLower(normalizeQualifiedSymbolSeparators(query))
	if normalizedQuery == "blcksock.tblocksocket.create" {
		constructorLocation := map[string]any{
			"uri": openedURI,
			"range": map[string]any{
				"start": map[string]any{"line": 12, "character": 12},
				"end":   map[string]any{"line": 12, "character": 24},
			},
		}

		return []map[string]any{
			{
				"name":          "Create",
				"kind":          9,
				"containerName": "blcksock.TBlockSocket",
				"location":      constructorLocation,
			},
		}
	}

	if normalizedQuery != "blcksock.tblocksocket" {
		return buildDelphiOracleIrrelevantWorkspaceSymbols(query, openedURI)
	}

	classLocation := map[string]any{
		"uri": openedURI,
		"range": map[string]any{
			"start": map[string]any{"line": 5, "character": 2},
			"end":   map[string]any{"line": 5, "character": 14},
		},
	}

	constructorLocation := map[string]any{
		"uri": openedURI,
		"range": map[string]any{
			"start": map[string]any{"line": 12, "character": 12},
			"end":   map[string]any{"line": 12, "character": 24},
		},
	}

	return []map[string]any{
		{
			"name":          "TBlockSocket",
			"kind":          9,
			"containerName": "blcksock",
			"location":      constructorLocation,
		},
		{
			"name":          "TBlockSocket",
			"kind":          5,
			"containerName": "blcksock",
			"location":      classLocation,
		},
	}
}

func buildDelphiOracleQualifiedTypeNoStrongWorkspaceSymbols(query string, openedURI string) []map[string]any {
	if !strings.EqualFold(normalizeQualifiedSymbolSeparators(query), "blcksock.tblocksocket") {
		return buildDelphiOracleIrrelevantWorkspaceSymbols(query, openedURI)
	}

	constructorLocation := map[string]any{
		"uri": openedURI,
		"range": map[string]any{
			"start": map[string]any{"line": 12, "character": 12},
			"end":   map[string]any{"line": 12, "character": 24},
		},
	}

	return []map[string]any{
		{
			"name":          "TBlockSocket",
			"kind":          9,
			"containerName": "blcksock",
			"location":      constructorLocation,
		},
	}
}

func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("falha ao serializar json do fake LSP: %v", err))
	}
	return b
}

func setFakeDelphiOracleReferencePayload(t *testing.T, ranges []fakeDelphiOracleReferenceRange) {
	t.Helper()

	raw, err := json.Marshal(ranges)
	if err != nil {
		t.Fatalf("falha ao serializar payload cru de references para o fake LSP: %v", err)
	}

	t.Setenv(fakeLSPReferencesPayloadEnv, string(raw))
}

func loadFakeDelphiOracleReferenceRanges() []fakeDelphiOracleReferenceRange {
	raw := strings.TrimSpace(os.Getenv(fakeLSPReferencesPayloadEnv))
	if raw == "" {
		return nil
	}

	var ranges []fakeDelphiOracleReferenceRange
	if err := json.Unmarshal([]byte(raw), &ranges); err != nil {
		panic(fmt.Sprintf("falha ao desserializar payload cru de references do fake LSP: %v", err))
	}

	return ranges
}

func buildDelphiOracleReferencePayload(uri string, ranges []fakeDelphiOracleReferenceRange) []map[string]any {
	result := make([]map[string]any, 0, len(ranges))
	for _, current := range ranges {
		result = append(result, map[string]any{
			"uri": uri,
			"range": map[string]any{
				"start": map[string]any{"line": current.StartLine, "character": current.StartCharacter},
				"end":   map[string]any{"line": current.EndLine, "character": current.EndCharacter},
			},
		})
	}

	return result
}

func extractReferenceLinesFromResult(t *testing.T, result string) []int {
	t.Helper()

	positionsLine := ""
	for _, line := range strings.Split(result, "\n") {
		if strings.HasPrefix(line, "At: ") {
			positionsLine = line
			break
		}
	}

	if positionsLine == "" {
		t.Fatalf("resultado de references sem linha de posicoes At: %s", result)
	}

	matches := fakeLSPReferenceLinePattern.FindAllStringSubmatch(positionsLine, -1)
	lines := make([]int, 0, len(matches))
	for _, match := range matches {
		lineNumber, err := strconv.Atoi(match[1])
		if err != nil {
			t.Fatalf("falha ao converter linha de referencia %q: %v", match[1], err)
		}
		lines = append(lines, lineNumber)
	}

	return lines
}

func sameIntSlice(left []int, right []int) bool {
	if len(left) != len(right) {
		return false
	}

	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}

	return true
}
