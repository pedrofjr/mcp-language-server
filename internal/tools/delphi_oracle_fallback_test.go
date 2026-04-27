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

	for {
		msg, err := lsp.ReadMessage(reader)
		if err != nil {
			return
		}

		switch msg.Method {
		case "initialize":
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

func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("falha ao serializar json do fake LSP: %v", err))
	}
	return b
}