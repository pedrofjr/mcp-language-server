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
	URI          string                  `json:"uri"`
	UnitName     string                  `json:"unitName"`
	TotalSymbols int                     `json:"totalSymbols"`
	Symbols      []symbolsOverviewSymbol `json:"symbols"`
}

type symbolsOverviewSymbol struct {
	Name          string                     `json:"name"`
	Kind          any                        `json:"kind"`
	ContainerName string                     `json:"containerName,omitempty"`
	Trace         symbolsOverviewSymbolTrace `json:"trace"`
}

type symbolsOverviewSymbolTrace struct {
	PreferredSymbolName  string                            `json:"preferredSymbolName"`
	SymbolNameCandidates []string                          `json:"symbolNameCandidates"`
	Definition           symbolsOverviewTraceTargetRequest `json:"definition"`
	References           symbolsOverviewTraceTargetRequest `json:"references"`
}

type symbolsOverviewTraceTargetRequest struct {
	SymbolName string `json:"symbolName"`
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

	if decoded.TotalSymbols != 5 {
		t.Fatalf("esperado totalSymbols=5, obtido=%d", decoded.TotalSymbols)
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
	if len(firstSymbols) != 3 {
		t.Fatalf("esperado 3 simbolos em UAlpha, obtido=%d", len(firstSymbols))
	}

	if strings.ToLower(firstSymbols[0].Name) != "alpha" || strings.ToLower(firstSymbols[1].Name) != "beta" {
		t.Fatalf("simbolos devem ser ordenados por name case-insensitive; obtido=%q,%q", firstSymbols[0].Name, firstSymbols[1].Name)
	}
}

func TestGetSymbolsOverview_TraceSchemaAndMinimumRules(t *testing.T) {
	client, cleanup := setupGetSymbolsOverviewFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	resultText, err := GetSymbolsOverview(ctx, client, "trace")
	if err != nil {
		t.Fatalf("GetSymbolsOverview nao deveria falhar no cenario de trace: %v", err)
	}

	decoded := decodeSymbolsOverviewResult(t, resultText)
	assertSymbolsOverviewMinimumSchema(t, decoded)

	if len(decoded.Units) == 0 {
		t.Fatal("esperado ao menos uma unit no payload de trace")
	}

	for _, unit := range decoded.Units {
		for _, symbol := range unit.Symbols {
			assertTraceMinimumRules(t, symbol)
		}
	}
}

func TestGetSymbolsOverview_TraceIncludesContainerAndSimpleFallbackCandidates(t *testing.T) {
	client, cleanup := setupGetSymbolsOverviewFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	resultText, err := GetSymbolsOverview(ctx, client, "trace")
	if err != nil {
		t.Fatalf("GetSymbolsOverview nao deveria falhar no cenario de candidatos de trace: %v", err)
	}

	decoded := decodeSymbolsOverviewResult(t, resultText)
	assertSymbolsOverviewMinimumSchema(t, decoded)

	withContainer := findOverviewSymbolByName(t, decoded, "DoWork")
	assertContainsCandidate(t, withContainer.Trace.SymbolNameCandidates, withContainer.Name)
	assertContainsCandidate(t, withContainer.Trace.SymbolNameCandidates, "TAlphaService."+withContainer.Name)
	assertContainsCandidate(t, withContainer.Trace.SymbolNameCandidates, "UAlpha."+withContainer.Name)

	withoutContainer := findOverviewSymbolByName(t, decoded, "BetaHelper")
	assertContainsCandidate(t, withoutContainer.Trace.SymbolNameCandidates, withoutContainer.Name)
	assertContainsCandidate(t, withoutContainer.Trace.SymbolNameCandidates, "UBeta."+withoutContainer.Name)
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

func TestGetSymbolsOverview_LargeVolumeDeterministicTotalsAndOrdering(t *testing.T) {
	client, cleanup := setupGetSymbolsOverviewFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	resultText, err := GetSymbolsOverview(ctx, client, "__large_volume__")
	if err != nil {
		t.Fatalf("GetSymbolsOverview nao deveria falhar no cenario de alto volume: %v", err)
	}

	decoded := decodeSymbolsOverviewResult(t, resultText)
	assertSymbolsOverviewMinimumSchema(t, decoded)

	const expectedUnits = 12
	const expectedSymbolsPerUnit = 132
	const expectedTotalSymbols = expectedUnits * expectedSymbolsPerUnit

	if decoded.TotalUnits != expectedUnits {
		t.Fatalf("esperado totalUnits=%d, obtido=%d", expectedUnits, decoded.TotalUnits)
	}

	if decoded.TotalSymbols != expectedTotalSymbols {
		t.Fatalf("esperado totalSymbols=%d, obtido=%d", expectedTotalSymbols, decoded.TotalSymbols)
	}

	if len(decoded.Units) != expectedUnits {
		t.Fatalf("esperado len(units)=%d, obtido=%d", expectedUnits, len(decoded.Units))
	}

	accumulatedSymbols := 0
	for i, unit := range decoded.Units {
		if unit.TotalSymbols != len(unit.Symbols) {
			t.Fatalf("unit.TotalSymbols deve refletir len(unit.Symbols); unit=%q total=%d len=%d", unit.UnitName, unit.TotalSymbols, len(unit.Symbols))
		}
		accumulatedSymbols += unit.TotalSymbols

		for j := 1; j < len(unit.Symbols); j++ {
			if compareOverviewSymbols(unit.Symbols[j-1], unit.Symbols[j]) > 0 {
				t.Fatalf("ordenacao de symbols deve ser deterministica por name/kind/containerName/preferred; unit=%q indexAnterior=%d indexAtual=%d", unit.UnitName, j-1, j)
			}
		}

		for _, sym := range unit.Symbols {
			assertTraceMinimumRules(t, sym)
		}

		if i > 0 && compareOverviewUnits(decoded.Units[i-1], unit) > 0 {
			t.Fatalf("ordenacao de units deve ser deterministica por unitName(case-insensitive) e uri; indexAnterior=%d indexAtual=%d", i-1, i)
		}
	}

	if accumulatedSymbols != decoded.TotalSymbols {
		t.Fatalf("somatorio de symbols por unit deve bater com totalSymbols; sum=%d total=%d", accumulatedSymbols, decoded.TotalSymbols)
	}
}

func TestGetSymbolsOverview_WhenWorkspaceSymbolsAreDegraded_UsesSafeFallbacksWithoutPanic(t *testing.T) {
	client, cleanup := setupGetSymbolsOverviewFakeClient(t)
	defer cleanup()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("GetSymbolsOverview nao deve panicar em payload degradado: %v", r)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	resultText, err := GetSymbolsOverview(ctx, client, "__degraded__")
	if err != nil {
		t.Fatalf("GetSymbolsOverview nao deveria falhar no cenario degradado: %v", err)
	}

	decoded := decodeSymbolsOverviewResult(t, resultText)
	assertSymbolsOverviewMinimumSchema(t, decoded)

	if decoded.TotalUnits != 4 {
		t.Fatalf("esperado totalUnits=4 no cenario degradado, obtido=%d", decoded.TotalUnits)
	}

	if decoded.TotalSymbols != 4 {
		t.Fatalf("esperado totalSymbols=4 no cenario degradado, obtido=%d", decoded.TotalSymbols)
	}

	degraded := findOverviewSymbolByName(t, decoded, "NoRangeMethod")
	if strings.TrimSpace(degraded.ContainerName) == "" {
		t.Fatalf("compatibilidade de schema: containerName deve ser preservado para simbolo com container; simbolo=%q", degraded.Name)
	}

	assertContainsCandidate(t, degraded.Trace.SymbolNameCandidates, "TNoRangeService.NoRangeMethod")

	hasUnknownURI := false
	for _, unit := range decoded.Units {
		if unit.URI == "unknown:///" {
			hasUnknownURI = true
		}
		if strings.TrimSpace(unit.UnitName) == "" {
			t.Fatalf("unitName nao pode ficar vazio no fallback degradado; uri=%q", unit.URI)
		}
	}

	if !hasUnknownURI {
		t.Fatal("cenario degradado deve mapear URI vazio para unknown:/// sem quebrar agregacao")
	}
}

func TestGetSymbolsOverview_WhenSameInputTwice_ResultIsStable(t *testing.T) {
	client, cleanup := setupGetSymbolsOverviewFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	first, err := GetSymbolsOverview(ctx, client, "__large_volume__")
	if err != nil {
		t.Fatalf("primeira execucao nao deveria falhar: %v", err)
	}

	second, err := GetSymbolsOverview(ctx, client, "__large_volume__")
	if err != nil {
		t.Fatalf("segunda execucao nao deveria falhar: %v", err)
	}

	if first != second {
		t.Fatalf("resultado JSON deve ser estavel para mesmo input; firstLen=%d secondLen=%d", len(first), len(second))
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
			case "__large_volume__":
				sendGetSymbolsOverviewFakeResponse(writer, msg.ID, buildLargeVolumeGetSymbolsOverviewFakePayload(), nil)
			case "__degraded__":
				sendGetSymbolsOverviewFakeResponse(writer, msg.ID, buildDegradedGetSymbolsOverviewFakePayload(), nil)
			default:
				sendGetSymbolsOverviewFakeResponse(writer, msg.ID, buildDefaultGetSymbolsOverviewFakePayload(), nil)
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

func findOverviewSymbolByName(t *testing.T, result symbolsOverviewResult, name string) symbolsOverviewSymbol {
	t.Helper()

	for _, unit := range result.Units {
		for _, symbol := range unit.Symbols {
			if strings.EqualFold(strings.TrimSpace(symbol.Name), strings.TrimSpace(name)) {
				return symbol
			}
		}
	}

	t.Fatalf("simbolo %q nao encontrado no payload de get_symbols_overview", name)
	return symbolsOverviewSymbol{}
}

func assertTraceMinimumRules(t *testing.T, symbol symbolsOverviewSymbol) {
	t.Helper()

	if strings.TrimSpace(symbol.Trace.PreferredSymbolName) == "" {
		t.Fatalf("trace.preferredSymbolName deve existir e ser nao-vazio para simbolo=%q", symbol.Name)
	}

	if len(symbol.Trace.SymbolNameCandidates) == 0 {
		t.Fatalf("trace.symbolNameCandidates deve existir com ao menos um candidato para simbolo=%q", symbol.Name)
	}

	first := strings.TrimSpace(symbol.Trace.SymbolNameCandidates[0])
	if first == "" {
		t.Fatalf("primeiro candidato nao pode ser vazio para simbolo=%q", symbol.Name)
	}

	if symbol.Trace.PreferredSymbolName != first {
		t.Fatalf("preferredSymbolName deve ser o primeiro candidato; simbolo=%q preferred=%q first=%q", symbol.Name, symbol.Trace.PreferredSymbolName, first)
	}

	if symbol.Trace.Definition.SymbolName != symbol.Trace.PreferredSymbolName {
		t.Fatalf("definition.symbolName deve ser igual ao preferredSymbolName; simbolo=%q definition=%q preferred=%q", symbol.Name, symbol.Trace.Definition.SymbolName, symbol.Trace.PreferredSymbolName)
	}

	if symbol.Trace.References.SymbolName != symbol.Trace.PreferredSymbolName {
		t.Fatalf("references.symbolName deve ser igual ao preferredSymbolName; simbolo=%q references=%q preferred=%q", symbol.Name, symbol.Trace.References.SymbolName, symbol.Trace.PreferredSymbolName)
	}

	seen := make(map[string]struct{}, len(symbol.Trace.SymbolNameCandidates))
	for _, candidate := range symbol.Trace.SymbolNameCandidates {
		normalized := strings.TrimSpace(candidate)
		if normalized == "" {
			t.Fatalf("symbolNameCandidates nao deve conter vazio para simbolo=%q", symbol.Name)
		}
		key := strings.ToLower(normalized)
		if _, exists := seen[key]; exists {
			t.Fatalf("symbolNameCandidates deve ser deduplicado; simbolo=%q candidatoDuplicado=%q", symbol.Name, normalized)
		}
		seen[key] = struct{}{}
	}
}

func assertContainsCandidate(t *testing.T, candidates []string, expected string) {
	t.Helper()

	want := strings.TrimSpace(expected)
	if want == "" {
		t.Fatal("assertContainsCandidate recebeu expected vazio")
	}

	for _, candidate := range candidates {
		if strings.EqualFold(strings.TrimSpace(candidate), want) {
			return
		}
	}

	t.Fatalf("candidato esperado nao encontrado; expected=%q candidates=%v", want, candidates)
}

func compareOverviewUnits(left, right symbolsOverviewUnit) int {
	leftName := strings.ToLower(strings.TrimSpace(left.UnitName))
	rightName := strings.ToLower(strings.TrimSpace(right.UnitName))
	if leftName < rightName {
		return -1
	}
	if leftName > rightName {
		return 1
	}

	if left.URI < right.URI {
		return -1
	}
	if left.URI > right.URI {
		return 1
	}
	return 0
}

func compareOverviewSymbols(left, right symbolsOverviewSymbol) int {
	leftName := strings.ToLower(strings.TrimSpace(left.Name))
	rightName := strings.ToLower(strings.TrimSpace(right.Name))
	if leftName < rightName {
		return -1
	}
	if leftName > rightName {
		return 1
	}

	leftKind := strings.ToLower(fmt.Sprint(left.Kind))
	rightKind := strings.ToLower(fmt.Sprint(right.Kind))
	if leftKind < rightKind {
		return -1
	}
	if leftKind > rightKind {
		return 1
	}

	leftContainer := strings.ToLower(strings.TrimSpace(left.ContainerName))
	rightContainer := strings.ToLower(strings.TrimSpace(right.ContainerName))
	if leftContainer < rightContainer {
		return -1
	}
	if leftContainer > rightContainer {
		return 1
	}

	leftPreferred := strings.ToLower(strings.TrimSpace(left.Trace.PreferredSymbolName))
	rightPreferred := strings.ToLower(strings.TrimSpace(right.Trace.PreferredSymbolName))
	if leftPreferred < rightPreferred {
		return -1
	}
	if leftPreferred > rightPreferred {
		return 1
	}

	return 0
}

func buildDefaultGetSymbolsOverviewFakePayload() []map[string]any {
	return []map[string]any{
		{
			"name":          "DoWork",
			"kind":          float64(protocol.Method),
			"containerName": "TAlphaService",
			"location": map[string]any{
				"uri": "file:///C:/project/UAlpha.pas",
				"range": map[string]any{
					"start": map[string]any{"line": 0.0, "character": 0.0},
					"end":   map[string]any{"line": 0.0, "character": 0.0},
				},
			},
		},
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
}

func buildLargeVolumeGetSymbolsOverviewFakePayload() []map[string]any {
	const units = 12
	const symbolsPerUnit = 130

	result := make([]map[string]any, 0, units*(symbolsPerUnit+2))
	for unitIndex := units - 1; unitIndex >= 0; unitIndex-- {
		uri := fmt.Sprintf("file:///C:/project/ULarge%02d.pas", unitIndex)

		for symbolIndex := symbolsPerUnit - 1; symbolIndex >= 0; symbolIndex-- {
			normalized := (symbolIndex*17 + unitIndex*3) % 10000
			name := fmt.Sprintf("Sym%04d", normalized)
			container := fmt.Sprintf("TLoad%02d", (symbolIndex+unitIndex)%9)
			kind := float64(protocol.Function)
			if symbolIndex%3 == 0 {
				kind = float64(protocol.Method)
			}

			result = append(result, map[string]any{
				"name":          name,
				"kind":          kind,
				"containerName": container,
				"location": map[string]any{
					"uri": uri,
					"range": map[string]any{
						"start": map[string]any{"line": float64(symbolIndex), "character": 0.0},
						"end":   map[string]any{"line": float64(symbolIndex), "character": 1.0},
					},
				},
			})
		}

		result = append(result,
			map[string]any{
				"name":          "TieBreaker",
				"kind":          float64(protocol.Function),
				"containerName": "TAlpha",
				"location": map[string]any{
					"uri": uri,
					"range": map[string]any{
						"start": map[string]any{"line": 2000.0, "character": 0.0},
						"end":   map[string]any{"line": 2000.0, "character": 1.0},
					},
				},
			},
			map[string]any{
				"name":          "TieBreaker",
				"kind":          float64(protocol.Function),
				"containerName": "TZeta",
				"location": map[string]any{
					"uri": uri,
					"range": map[string]any{
						"start": map[string]any{"line": 2001.0, "character": 0.0},
						"end":   map[string]any{"line": 2001.0, "character": 1.0},
					},
				},
			},
		)
	}

	return result
}

func buildDegradedGetSymbolsOverviewFakePayload() []map[string]any {
	return []map[string]any{
		{
			"name":          "NoRangeMethod",
			"kind":          float64(protocol.Method),
			"containerName": "TNoRangeService",
			"location": map[string]any{
				"uri": "file:///C:/project/NoRangeUnit.pas",
			},
		},
		{
			"name": "OddURIProc",
			"kind": float64(protocol.Function),
			"location": map[string]any{
				"uri": "untitled:Scratch Buffer",
			},
		},
		{
			"name": "EmptyURIFunc",
			"kind": float64(protocol.Function),
			"location": map[string]any{
				"uri": "",
			},
		},
		{
			"name": "MinSymbol",
			"kind": float64(protocol.Variable),
			"location": map[string]any{
				"uri": "custom-scheme://host",
			},
		},
	}
}
