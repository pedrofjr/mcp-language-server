package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

const (
	fakeLSPEnv                         = "MCP_FAKE_LSP_DELPHI_ORACLE"
	fakeLSPWorkspaceSymbolProviderEnv = "MCP_FAKE_LSP_DELPHI_ORACLE_WORKSPACE_SYMBOL_PROVIDER"
	fakeLSPWorkspaceSymbolEmptyEnv    = "MCP_FAKE_LSP_DELPHI_ORACLE_WORKSPACE_SYMBOL_EMPTY_RESULT"
	fakeLSPWorkspaceSymbolIrrelevantEnv = "MCP_FAKE_LSP_DELPHI_ORACLE_WORKSPACE_SYMBOL_IRRELEVANT_RESULT"
	fakeLSPWorkspaceSymbolMatchedButUnsustainedEnv = "MCP_FAKE_LSP_DELPHI_ORACLE_WORKSPACE_SYMBOL_MATCHED_BUT_UNSUSTAINED_RESULT"
)

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
		name      string
		fileName  string
		symbol    string
		content   string
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

func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("falha ao serializar json do fake LSP: %v", err))
	}
	return b
}