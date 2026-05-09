package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

const symbolsOverviewFakeLSPEnv = "MCP_FAKE_LSP_SYMBOLS_OVERVIEW"

type symbolsOverviewResult struct {
	QueryUsed    string                `json:"queryUsed"`
	TotalUnits   int                   `json:"totalUnits"`
	TotalSymbols int                   `json:"totalSymbols"`
	Units        []symbolsOverviewUnit `json:"units"`
}

type symbolsOverviewUnit struct {
	URI          string                   `json:"uri"`
	UnitName     string                   `json:"unitName"`
	TotalSymbols int                      `json:"totalSymbols"`
	Symbols      []symbolsOverviewSymbol  `json:"symbols"`
}

type symbolsOverviewSymbol struct {
	Name string `json:"name"`
	Kind any    `json:"kind"`
}

func TestHelperProcessGetSymbolsOverviewFakeLSP(t *testing.T) {
	if os.Getenv(symbolsOverviewFakeLSPEnv) != "1" {
		return
	}

	runGetSymbolsOverviewFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestGetSymbolsOverview_WhenQueryWhitespace_UsesTrimmedQueryAndReturnsValidSchema(t *testing.T) {
	client, cleanup := setupGetSymbolsOverviewFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	resultText, err := GetSymbolsOverview(ctx, client, " \t  ")
	if err != nil {
		t.Fatalf("GetSymbolsOverview nao deveria falhar com query em whitespace: %v", err)
	}

	decoded := decodeSymbolsOverviewResult(t, resultText)
	assertSymbolsOverviewMinimumSchema(t, decoded)

	if decoded.QueryUsed != "" {
		t.Fatalf("queryUsed deve refletir query trimada vazia; obtido=%q", decoded.QueryUsed)
	}
}

func TestGetSymbolsOverview_GroupsByURIAndSortsDeterministically(t *testing.T) {
	client, cleanup := setupGetSymbolsOverviewFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	resultText, err := GetSymbolsOverview(ctx, client, "any")
	if err != nil {
		t.Fatalf("GetSymbolsOverview nao deveria falhar no cenario nominal: %v", err)
	}

	decoded := decodeSymbolsOverviewResult(t, resultText)
	assertSymbolsOverviewMinimumSchema(t, decoded)

	if decoded.TotalUnits != 2 {
		t.Fatalf("esperado totalUnits=2 apos agrupamento por URI, obtido=%d", decoded.TotalUnits)
	}

	if decoded.TotalSymbols != 4 {
		t.Fatalf("esperado totalSymbols=4, obtido=%d", decoded.TotalSymbols)
	}

	if len(decoded.Units) != 2 {
		t.Fatalf("esperado 2 units no payload, obtido=%d", len(decoded.Units))
	}

	if strings.ToLower(decoded.Units[0].UnitName) != "ualpha" {
		t.Fatalf("ordenacao de units deve priorizar unitName case-insensitive; first=%q", decoded.Units[0].UnitName)
	}

	if strings.ToLower(decoded.Units[1].UnitName) != "ubeta" {
		t.Fatalf("ordenacao de units deve colocar ubeta em segundo; second=%q", decoded.Units[1].UnitName)
	}

	firstSymbols := decoded.Units[0].Symbols
	if len(firstSymbols) != 2 {
		t.Fatalf("esperado 2 simbolos em UAlpha, obtido=%d", len(firstSymbols))
	}

	if strings.ToLower(firstSymbols[0].Name) != "alpha" || strings.ToLower(firstSymbols[1].Name) != "beta" {
		t.Fatalf("simbolos devem ser ordenados por name case-insensitive; obtido=%q,%q", firstSymbols[0].Name, firstSymbols[1].Name)
	}
}

func TestGetSymbolsOverview_WhenBackendReturnsEmpty_ReturnsZeroTotals(t *testing.T) {
	client, cleanup := setupGetSymbolsOverviewFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	resultText, err := GetSymbolsOverview(ctx, client, "__empty__")
	if err != nil {
		t.Fatalf("GetSymbolsOverview nao deveria falhar com resposta vazia: %v", err)
	}

	decoded := decodeSymbolsOverviewResult(t, resultText)
	assertSymbolsOverviewMinimumSchema(t, decoded)

	if decoded.TotalUnits != 0 || decoded.TotalSymbols != 0 {
		t.Fatalf("resposta vazia deve retornar totais zero; totalUnits=%d totalSymbols=%d", decoded.TotalUnits, decoded.TotalSymbols)
	}

	if len(decoded.Units) != 0 {
		t.Fatalf("resposta vazia deve retornar units vazio; obtido=%d", len(decoded.Units))
	}
}

func TestGetSymbolsOverview_WhenWorkspaceSymbolRequestFails_PropagatesContext(t *testing.T) {
	client, cleanup := setupGetSymbolsOverviewFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	_, err := GetSymbolsOverview(ctx, client, "__backend_error__")
	if err == nil {
		t.Fatal("esperado erro quando backend retorna falha em workspace/symbol")
	}

	if !strings.Contains(err.Error(), "workspace/symbol request failed") {
		t.Fatalf("erro deve propagar contexto claro do request LSP; obtido=%v", err)
	}
}

func setupGetSymbolsOverviewFakeClient(t *testing.T) (*lsp.Client, func()) {
	t.Helper()

	t.Setenv(symbolsOverviewFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("falha ao resolver binario de teste: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessGetSymbolsOverviewFakeLSP")
	if err != nil {
		t.Fatalf("falha ao iniciar fake LSP: %v", err)
	}

	cleanup := func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, t.TempDir()); err != nil {
		cleanup()
		t.Fatalf("falha ao inicializar fake LSP: %v", err)
	}

	return client, cleanup
}

func runGetSymbolsOverviewFakeLSP(stdin *os.File, stdout *os.File) {
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
					"workspaceSymbolProvider": true,
				},
			}
			sendGetSymbolsOverviewFakeResponse(writer, msg.ID, result, nil)
		case "initialized":
			// no-op
		case "workspace/symbol":
			var params protocol.WorkspaceSymbolParams
			if err := json.Unmarshal(msg.Params, &params); err != nil {
				sendGetSymbolsOverviewFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32602, Message: "invalid params"})
				continue
			}

			if params.Query != "" && strings.TrimSpace(params.Query) == "" {
				sendGetSymbolsOverviewFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32602, Message: "query not trimmed"})
				continue
			}

			switch params.Query {
			case "__backend_error__":
				sendGetSymbolsOverviewFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32001, Message: "backend exploded"})
			case "__empty__":
				sendGetSymbolsOverviewFakeResponse(writer, msg.ID, []map[string]any{}, nil)
			default:
				result := []map[string]any{
					{
						"name": "zeta",
						"kind": float64(protocol.Function),
						"location": map[string]any{
							"uri": "file:///C:/project/UBeta.pas",
							"range": map[string]any{
								"start": map[string]any{"line": 0.0, "character": 0.0},
								"end":   map[string]any{"line": 0.0, "character": 0.0},
							},
						},
					},
					{
						"name": "beta",
						"kind": float64(protocol.Function),
						"location": map[string]any{
							"uri": "file:///C:/project/UAlpha.pas",
							"range": map[string]any{
								"start": map[string]any{"line": 0.0, "character": 0.0},
								"end":   map[string]any{"line": 0.0, "character": 0.0},
							},
						},
					},
					{
						"name": "Alpha",
						"kind": float64(protocol.Method),
						"location": map[string]any{
							"uri": "file:///C:/project/UAlpha.pas",
							"range": map[string]any{
								"start": map[string]any{"line": 1.0, "character": 0.0},
								"end":   map[string]any{"line": 1.0, "character": 0.0},
							},
						},
					},
					{
						"name": "BetaHelper",
						"kind": float64(protocol.Function),
						"location": map[string]any{
							"uri": "file:///C:/project/UBeta.pas",
							"range": map[string]any{
								"start": map[string]any{"line": 1.0, "character": 0.0},
								"end":   map[string]any{"line": 1.0, "character": 0.0},
							},
						},
					},
				}
				sendGetSymbolsOverviewFakeResponse(writer, msg.ID, result, nil)
			}
		case "shutdown":
			sendGetSymbolsOverviewFakeResponse(writer, msg.ID, nil, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendGetSymbolsOverviewFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32601, Message: "method not found: " + msg.Method})
			}
		}
	}
}

func sendGetSymbolsOverviewFakeResponse(w *os.File, id *lsp.MessageID, result any, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
	}

	if rpcErr == nil {
		if result == nil {
			resp.Result = mustMarshalGetSymbolsOverviewFake(struct{}{})
		} else {
			resp.Result = mustMarshalGetSymbolsOverviewFake(result)
		}
	}

	_ = lsp.WriteMessage(w, resp)
}

func mustMarshalGetSymbolsOverviewFake(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal fake LSP json: %v", err))
	}
	return b
}

func decodeSymbolsOverviewResult(t *testing.T, payload string) symbolsOverviewResult {
	t.Helper()

	if !json.Valid([]byte(payload)) {
		t.Fatalf("saida deve ser JSON textual valido; obtido=%s", payload)
	}

	var decoded symbolsOverviewResult
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("falha ao parsear json de get_symbols_overview: %v", err)
	}

	return decoded
}

func assertSymbolsOverviewMinimumSchema(t *testing.T, result symbolsOverviewResult) {
	t.Helper()

	if result.QueryUsed == "__missing__" {
		t.Fatal("queryUsed deve existir no schema")
	}

	if result.TotalUnits < 0 {
		t.Fatalf("totalUnits deve ser >= 0; obtido=%d", result.TotalUnits)
	}

	if result.TotalSymbols < 0 {
		t.Fatalf("totalSymbols deve ser >= 0; obtido=%d", result.TotalSymbols)
	}

	if result.Units == nil {
		t.Fatal("units deve existir no schema como array (pode ser vazio)")
	}

	for _, unit := range result.Units {
		if strings.TrimSpace(unit.URI) == "" {
			t.Fatal("cada unit deve possuir uri nao-vazia")
		}
		if strings.TrimSpace(unit.UnitName) == "" {
			t.Fatal("cada unit deve possuir unitName derivado para exibicao")
		}
		if unit.TotalSymbols < 0 {
			t.Fatalf("totalSymbols da unit deve ser >= 0; obtido=%d", unit.TotalSymbols)
		}
		if unit.Symbols == nil {
			t.Fatal("cada unit deve possuir symbols como array")
		}
		for _, symbol := range unit.Symbols {
			if strings.TrimSpace(symbol.Name) == "" {
				t.Fatal("cada simbolo deve possuir name nao-vazio")
			}
			if symbol.Kind == nil {
				t.Fatal("cada simbolo deve possuir kind")
			}
		}
	}
}