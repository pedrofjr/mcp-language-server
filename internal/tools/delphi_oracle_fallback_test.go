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

const fakeLSPEnv = "MCP_FAKE_LSP_DELPHI_ORACLE"

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

	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "main.pas")
	fixture := strings.Join([]string{
		"procedure FindCustomer;",
		"begin",
		"  MissingIdentifier := 1;",
		"end;",
		"",
	}, "\n")

	if err := os.WriteFile(filePath, []byte(fixture), 0o644); err != nil {
		t.Fatalf("falha ao escrever fixture Delphi: %v", err)
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

	return client, filePath, cleanup
}

func runDelphiOracleFakeLSP(stdin *os.File, stdout *os.File) {
	reader := bufio.NewReader(stdin)
	writer := stdout

	for {
		msg, err := lsp.ReadMessage(reader)
		if err != nil {
			return
		}

		switch msg.Method {
		case "initialize":
			result := map[string]any{
				"capabilities": map[string]any{
					"definitionProvider": true,
					"referencesProvider": true,
					"hoverProvider":      true,
				},
			}
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