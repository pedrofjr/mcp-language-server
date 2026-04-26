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

const capabilitiesRegressionFakeLSPEnv = "MCP_FAKE_LSP_CAPABILITIES_REGRESSION"
const capabilitiesRegressionDocSymbolEnv = "MCP_FAKE_LSP_CAPABILITIES_DOCUMENT_SYMBOL"
const capabilitiesRegressionWorkspaceSymbolEnv = "MCP_FAKE_LSP_CAPABILITIES_WORKSPACE_SYMBOL"
const capabilitiesRegressionDocSymbolErrorURIEnv = "MCP_FAKE_LSP_CAPABILITIES_DOCUMENT_SYMBOL_ERROR_URI"
const capabilitiesRegressionQualifiedContainerSymbolEnv = "MCP_FAKE_LSP_CAPABILITIES_QUALIFIED_CONTAINER_SYMBOL"
const capabilitiesRegressionQualifiedReferenceOwnerEnv = "MCP_FAKE_LSP_CAPABILITIES_QUALIFIED_REFERENCE_OWNER_SYMBOL"
const capabilitiesRegressionReferencesRetryEnv = "MCP_FAKE_LSP_CAPABILITIES_REFERENCES_RETRY"

func TestHelperProcessCapabilitiesRegressionFakeLSP(t *testing.T) {
	if os.Getenv(capabilitiesRegressionFakeLSPEnv) != "1" {
		return
	}

	runCapabilitiesRegressionFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestCapabilitiesRegression_GetFullDefinition_DoesNotCallDocumentSymbolWhenCapabilityNotAdvertised(t *testing.T) {
	client, filePath, cleanup := setupCapabilitiesRegressionFakeClient(t, false, true)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	start := protocol.Location{
		URI: protocol.URIFromPath(filePath),
		Range: protocol.Range{
			Start: protocol.Position{Line: 2, Character: 5},
			End:   protocol.Position{Line: 2, Character: 11},
		},
	}

	definition, _, err := GetFullDefinition(ctx, client, start)
	if err != nil {
		t.Fatalf("GetFullDefinition nao deveria falhar sem documentSymbolProvider: %v", err)
	}

	if !strings.Contains(definition, "func Target") {
		t.Fatalf("esperado fallback local contendo assinatura da funcao Target, obtido: %q", definition)
	}

	callCount, err := fakeLSPMethodCallCount(ctx, client, "textDocument/documentSymbol")
	if err != nil {
		t.Fatalf("falha ao obter contador de chamadas do fake LSP: %v", err)
	}

	if callCount != 0 {
		t.Fatalf("esperado nao chamar textDocument/documentSymbol quando capability nao anunciada; chamadas observadas: %d", callCount)
	}
}

func TestCapabilitiesRegression_GetLineRangesToDisplay_DedupesDocumentSymbolRequestsByURI(t *testing.T) {
	client, filePath, cleanup := setupCapabilitiesRegressionFakeClient(t, true, true)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	uri := protocol.URIFromPath(filePath)
	locations := []protocol.Location{
		{
			URI: uri,
			Range: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 5},
				End:   protocol.Position{Line: 2, Character: 11},
			},
		},
		{
			URI: uri,
			Range: protocol.Range{
				Start: protocol.Position{Line: 3, Character: 1},
				End:   protocol.Position{Line: 3, Character: 7},
			},
		},
		{
			URI: uri,
			Range: protocol.Range{
				Start: protocol.Position{Line: 4, Character: 1},
				End:   protocol.Position{Line: 4, Character: 2},
			},
		},
	}

	_, err := GetLineRangesToDisplay(ctx, client, locations, 6, 1)
	if err != nil {
		t.Fatalf("GetLineRangesToDisplay nao deveria falhar: %v", err)
	}

	callCount, err := fakeLSPMethodCallCount(ctx, client, "textDocument/documentSymbol")
	if err != nil {
		t.Fatalf("falha ao obter contador de chamadas do fake LSP: %v", err)
	}

	if callCount != 1 {
		t.Fatalf("esperado dedupe/cache de textDocument/documentSymbol por URI dentro da chamada; chamadas observadas: %d", callCount)
	}
}

func TestCapabilitiesRegression_GetLineRangesToDisplay_DoesNotDisableDocumentSymbolAcrossDifferentURIs(t *testing.T) {
	t.Setenv(capabilitiesRegressionDocSymbolErrorURIEnv, "a.go")

	fixtures := map[string]string{
		"a.go": strings.Join([]string{
			"package main",
			"",
			"func TargetA() {",
			"\tTargetA()",
			"}",
			"",
		}, "\n"),
		"b.go": strings.Join([]string{
			"package main",
			"",
			"func TargetB() {",
			"\tTargetB()",
			"}",
			"",
		}, "\n"),
	}

	client, filePaths, cleanup := setupCapabilitiesRegressionFakeClientWithFixtures(t, true, true, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	uriA := protocol.URIFromPath(filePaths["a.go"])
	uriB := protocol.URIFromPath(filePaths["b.go"])

	locations := []protocol.Location{
		{
			URI: uriA,
			Range: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 5},
				End:   protocol.Position{Line: 2, Character: 12},
			},
		},
		{
			URI: uriB,
			Range: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 5},
				End:   protocol.Position{Line: 2, Character: 12},
			},
		},
	}

	_, err := GetLineRangesToDisplay(ctx, client, locations, 6, 1)
	if err != nil {
		t.Fatalf("GetLineRangesToDisplay nao deveria falhar quando documentSymbol falha apenas para um URI: %v", err)
	}

	docSymbolURIs, err := fakeLSPDocumentSymbolURIs(ctx, client)
	if err != nil {
		t.Fatalf("falha ao obter URIs de chamadas documentSymbol do fake LSP: %v", err)
	}

	if len(docSymbolURIs) < 2 {
		t.Fatalf("esperado ao menos duas chamadas a textDocument/documentSymbol (A e B), observadas: %v", docSymbolURIs)
	}

	if docSymbolURIs[0] != string(uriA) {
		t.Fatalf("esperado primeira chamada de documentSymbol para URI A (%s), observado: %v", uriA, docSymbolURIs)
	}

	calledB := false
	for i := 1; i < len(docSymbolURIs); i++ {
		if docSymbolURIs[i] == string(uriB) {
			calledB = true
			break
		}
	}

	if !calledB {
		t.Fatalf("esperado ao menos uma chamada de documentSymbol para URI B (%s) apos falha em A; chamadas observadas: %v", uriB, docSymbolURIs)
	}
}

func TestCapabilitiesRegression_ReadDefinition_DoesNotCallWorkspaceSymbolWhenCapabilityNotAdvertised(t *testing.T) {
	client, filePath, cleanup := setupCapabilitiesRegressionFakeClient(t, true, false)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture para fallback de definition: %v", err)
	}

	result, err := ReadDefinition(ctx, client, "Target")
	if err != nil {
		t.Fatalf("ReadDefinition nao deveria falhar sem workspaceSymbolProvider: %v", err)
	}

	if !strings.Contains(result, "Target") {
		t.Fatalf("esperado resultado de definition contendo Target, obtido: %s", result)
	}

	callCount, err := fakeLSPMethodCallCount(ctx, client, "workspace/symbol")
	if err != nil {
		t.Fatalf("falha ao obter contador de chamadas do fake LSP: %v", err)
	}

	if callCount != 0 {
		t.Fatalf("esperado nao chamar workspace/symbol quando capability nao anunciada; chamadas observadas: %d", callCount)
	}
}

func TestCapabilitiesRegression_ReadDefinition_KeepsQualifiedQueryWhenBackendSplitsNameAndContainer(t *testing.T) {
	t.Setenv(capabilitiesRegressionQualifiedContainerSymbolEnv, "1")

	fixtures := map[string]string{
		"asn1util.pas": strings.Join([]string{
			"unit asn1util;",
			"",
			"function ASNEncOIDItem: Integer;",
			"begin",
			"  Result := 1;",
			"end;",
			"",
		}, "\n"),
	}

	client, filePaths, cleanup := setupCapabilitiesRegressionFakeClientWithFixtures(t, false, true, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	filePath := filePaths["asn1util.pas"]
	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture qualificada para definition: %v", err)
	}

	result, err := ReadDefinition(ctx, client, "asn1util.ASNEncOIDItem")
	if err != nil {
		t.Fatalf("ReadDefinition nao deveria falhar quando backend separa Name e ContainerName: %v", err)
	}

	if strings.Contains(result, "not found") {
		t.Fatalf("esperado preservar consulta qualificada quando backend retorna nome folha com container, obtido: %s", result)
	}

	if !strings.Contains(result, "Container Name: asn1util") {
		t.Fatalf("esperado definition incluir Container Name: asn1util, obtido: %s", result)
	}
}

func TestCapabilitiesRegression_FindReferences_DoesNotCallWorkspaceSymbolWhenCapabilityNotAdvertised(t *testing.T) {
	client, filePath, cleanup := setupCapabilitiesRegressionFakeClient(t, true, false)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := client.OpenFile(ctx, filePath); err != nil {
		t.Fatalf("falha ao abrir fixture para fallback de references: %v", err)
	}

	result, err := FindReferences(ctx, client, "MissingTarget")
	if err != nil {
		t.Fatalf("FindReferences nao deveria falhar sem workspaceSymbolProvider: %v", err)
	}

	if !strings.Contains(result, "No references found") {
		t.Fatalf("esperado comportamento estavel de fallback sem simbolo (No references found...), obtido: %s", result)
	}

	callCount, err := fakeLSPMethodCallCount(ctx, client, "workspace/symbol")
	if err != nil {
		t.Fatalf("falha ao obter contador de chamadas do fake LSP: %v", err)
	}

	if callCount != 0 {
		t.Fatalf("esperado nao chamar workspace/symbol em references quando capability nao anunciada; chamadas observadas: %d", callCount)
	}
}

func TestCapabilitiesRegression_FindReferences_RetriesOpenFileTextCandidateWhenWorkspaceSymbolLocationReturnsEmpty(t *testing.T) {
	t.Setenv(capabilitiesRegressionReferencesRetryEnv, "1")

	fixtures := map[string]string{
		"uHighlighterProcs.pas": strings.Join([]string{
			"unit uHighlighterProcs;",
			"",
			"function GetHighlightersFilter(const Extension: string): string;",
			"begin",
			"  Result := Extension;",
			"end;",
			"",
		}, "\n"),
		"EditU2.pas": strings.Join([]string{
			"unit EditU2;",
			"",
			"procedure Demo;",
			"begin",
			"  GetHighlightersFilter('.pas');",
			"end;",
			"",
		}, "\n"),
	}

	client, filePaths, cleanup := setupCapabilitiesRegressionFakeClientWithFixtures(t, false, true, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := client.OpenFile(ctx, filePaths["uHighlighterProcs.pas"]); err != nil {
		t.Fatalf("falha ao abrir fixture de declaracao para retry de references: %v", err)
	}

	if err := client.OpenFile(ctx, filePaths["EditU2.pas"]); err != nil {
		t.Fatalf("falha ao abrir fixture textual para retry de references: %v", err)
	}

	result, err := FindReferences(ctx, client, "GetHighlightersFilter")
	if err != nil {
		t.Fatalf("FindReferences nao deveria falhar no cenario de retry textual: %v", err)
	}

	if strings.Contains(result, "No references found") {
		t.Fatalf("esperado retry textual apos references vazio na declaracao; obtido: %s", result)
	}

	if !strings.Contains(result, filePaths["EditU2.pas"]) {
		t.Fatalf("esperado recuperar references reais a partir do candidato textual em %s; obtido: %s", filePaths["EditU2.pas"], result)
	}

	referenceURIs, err := fakeLSPReferenceURIs(ctx, client)
	if err != nil {
		t.Fatalf("falha ao obter URIs de textDocument/references do fake LSP: %v", err)
	}

	if len(referenceURIs) < 2 {
		t.Fatalf("esperado duas chamadas a textDocument/references (declaracao e retry textual), observadas: %v", referenceURIs)
	}

	if referenceURIs[0] != string(protocol.URIFromPath(filePaths["uHighlighterProcs.pas"])) {
		t.Fatalf("esperado primeira chamada de references na declaracao de uHighlighterProcs.pas; chamadas observadas: %v", referenceURIs)
	}

	if referenceURIs[1] != string(protocol.URIFromPath(filePaths["EditU2.pas"])) {
		t.Fatalf("esperado segunda chamada de references no candidato textual EditU2.pas; chamadas observadas: %v", referenceURIs)
	}
}

func TestCapabilitiesRegression_FindReferences_QualifiedQuerySelectsWorkspaceSymbolByContainerName(t *testing.T) {
	t.Setenv(capabilitiesRegressionQualifiedReferenceOwnerEnv, "1")

	fixtures := map[string]string{
		"main_form.pas": strings.Join([]string{
			"procedure TFormMain.FormCreate;",
			"begin",
			"  MainRef;",
			"end;",
			"",
		}, "\n"),
		"other_form.pas": strings.Join([]string{
			"procedure TOtherForm.FormCreate;",
			"begin",
			"  OtherRef;",
			"end;",
			"",
		}, "\n"),
	}

	client, filePaths, cleanup := setupCapabilitiesRegressionFakeClientWithFixtures(t, false, true, fixtures)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := client.OpenFile(ctx, filePaths["main_form.pas"]); err != nil {
		t.Fatalf("falha ao abrir fixture principal para references qualificado: %v", err)
	}

	if err := client.OpenFile(ctx, filePaths["other_form.pas"]); err != nil {
		t.Fatalf("falha ao abrir fixture secundario para references qualificado: %v", err)
	}

	result, err := FindReferences(ctx, client, "TFormMain.FormCreate")
	if err != nil {
		t.Fatalf("FindReferences nao deveria falhar para consulta qualificada com owner distinto: %v", err)
	}

	if !strings.Contains(result, filePaths["main_form.pas"]) {
		t.Fatalf("esperado references usar o simbolo do owner TFormMain e incluir %s, obtido: %s", filePaths["main_form.pas"], result)
	}

	if strings.Contains(result, filePaths["other_form.pas"]) {
		t.Fatalf("esperado ignorar FormCreate de outro owner e nao consultar %s, obtido: %s", filePaths["other_form.pas"], result)
	}

	callCount, err := fakeLSPMethodCallCount(ctx, client, "textDocument/references")
	if err != nil {
		t.Fatalf("falha ao obter contador de textDocument/references do fake LSP: %v", err)
	}

	if callCount != 1 {
		t.Fatalf("esperado uma unica chamada a textDocument/references para o owner correto; chamadas observadas: %d", callCount)
	}
}

func setupCapabilitiesRegressionFakeClient(t *testing.T, documentSymbolProvider bool, workspaceSymbolProvider bool) (*lsp.Client, string, func()) {
	t.Helper()

	fixtures := map[string]string{
		"main.go": strings.Join([]string{
			"package main",
			"",
			"func Target() {",
			"\tTarget()",
			"}",
			"",
		}, "\n"),
	}

	client, filePaths, cleanup := setupCapabilitiesRegressionFakeClientWithFixtures(t, documentSymbolProvider, workspaceSymbolProvider, fixtures)
	return client, filePaths["main.go"], cleanup
}

func setupCapabilitiesRegressionFakeClientWithFixtures(t *testing.T, documentSymbolProvider bool, workspaceSymbolProvider bool, fixtures map[string]string) (*lsp.Client, map[string]string, func()) {
	t.Helper()

	workspaceDir := t.TempDir()
	filePaths := make(map[string]string, len(fixtures))
	for fileName, content := range fixtures {
		filePath := filepath.Join(workspaceDir, fileName)
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("falha ao escrever fixture %s: %v", fileName, err)
		}
		filePaths[fileName] = filePath
	}

	t.Setenv(capabilitiesRegressionFakeLSPEnv, "1")
	if documentSymbolProvider {
		t.Setenv(capabilitiesRegressionDocSymbolEnv, "1")
	} else {
		t.Setenv(capabilitiesRegressionDocSymbolEnv, "0")
	}
	if workspaceSymbolProvider {
		t.Setenv(capabilitiesRegressionWorkspaceSymbolEnv, "1")
	} else {
		t.Setenv(capabilitiesRegressionWorkspaceSymbolEnv, "0")
	}

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("falha ao obter caminho do binario de teste: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessCapabilitiesRegressionFakeLSP")
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

func fakeLSPMethodCallCount(ctx context.Context, client *lsp.Client, method string) (int, error) {
	var counts map[string]int
	if err := client.Call(ctx, "mcp/getCallCounts", struct{}{}, &counts); err != nil {
		return 0, fmt.Errorf("falha ao buscar contadores de metodos: %w", err)
	}

	return counts[method], nil
}

func fakeLSPDocumentSymbolURIs(ctx context.Context, client *lsp.Client) ([]string, error) {
	var uris []string
	if err := client.Call(ctx, "mcp/getDocumentSymbolURIs", struct{}{}, &uris); err != nil {
		return nil, fmt.Errorf("falha ao buscar URIs de chamadas documentSymbol: %w", err)
	}

	return uris, nil
}

func fakeLSPReferenceURIs(ctx context.Context, client *lsp.Client) ([]string, error) {
	var uris []string
	if err := client.Call(ctx, "mcp/getReferenceURIs", struct{}{}, &uris); err != nil {
		return nil, fmt.Errorf("falha ao buscar URIs de chamadas references: %w", err)
	}

	return uris, nil
}

func runCapabilitiesRegressionFakeLSP(stdin *os.File, stdout *os.File) {
	reader := bufio.NewReader(stdin)
	writer := stdout
	counts := make(map[string]int)
	documentSymbolURIs := make([]string, 0)
	referenceURIs := make([]string, 0)
	openedURI := ""
	openedURIs := make([]string, 0)

	hasDocumentSymbol := os.Getenv(capabilitiesRegressionDocSymbolEnv) == "1"
	hasWorkspaceSymbol := os.Getenv(capabilitiesRegressionWorkspaceSymbolEnv) == "1"
	docSymbolErrorURI := os.Getenv(capabilitiesRegressionDocSymbolErrorURIEnv)
	qualifiedContainerSymbol := os.Getenv(capabilitiesRegressionQualifiedContainerSymbolEnv) == "1"
	qualifiedReferenceOwner := os.Getenv(capabilitiesRegressionQualifiedReferenceOwnerEnv) == "1"
	referencesRetryScenario := os.Getenv(capabilitiesRegressionReferencesRetryEnv) == "1"

	for {
		msg, err := lsp.ReadMessage(reader)
		if err != nil {
			return
		}

		if msg.Method != "" {
			counts[msg.Method]++
		}

		switch msg.Method {
		case "initialize":
			capabilities := map[string]any{
				"definitionProvider": true,
			}
			if hasDocumentSymbol {
				capabilities["documentSymbolProvider"] = true
			}
			if hasWorkspaceSymbol {
				capabilities["workspaceSymbolProvider"] = true
			}

			result := map[string]any{"capabilities": capabilities}
			sendCapabilitiesRegressionFakeResponse(writer, msg.ID, result, nil)
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
			openedURIs = append(openedURIs, params.TextDocument.URI)
		case "textDocument/documentSymbol":
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			documentSymbolURIs = append(documentSymbolURIs, params.TextDocument.URI)

			if docSymbolErrorURI != "" && strings.HasSuffix(params.TextDocument.URI, docSymbolErrorURI) {
				sendCapabilitiesRegressionFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32603, Message: "internal error: forced documentSymbol failure"})
				continue
			}

			result := []map[string]any{
				{
					"name": "Target",
					"kind": 12,
					"range": map[string]any{
						"start": map[string]any{"line": 2, "character": 0},
						"end":   map[string]any{"line": 4, "character": 1},
					},
					"selectionRange": map[string]any{
						"start": map[string]any{"line": 2, "character": 5},
						"end":   map[string]any{"line": 2, "character": 11},
					},
				},
			}
			sendCapabilitiesRegressionFakeResponse(writer, msg.ID, result, nil)
		case "workspace/symbol":
			var params struct {
				Query string `json:"query"`
			}
			_ = json.Unmarshal(msg.Params, &params)

			if referencesRetryScenario && params.Query == "GetHighlightersFilter" {
				declarationURI := findCapabilitiesRegressionOpenedURIBySuffix(openedURIs, "uHighlighterProcs.pas")
				result := []map[string]any{
					{
						"name": "GetHighlightersFilter",
						"kind": 12,
						"location": map[string]any{
							"uri": declarationURI,
							"range": map[string]any{
								"start": map[string]any{"line": 2, "character": 9},
								"end":   map[string]any{"line": 2, "character": 30},
							},
						},
					},
				}
				sendCapabilitiesRegressionFakeResponse(writer, msg.ID, result, nil)
				continue
			}

			if qualifiedReferenceOwner && params.Query == "TFormMain.FormCreate" {
				mainURI := findCapabilitiesRegressionOpenedURIBySuffix(openedURIs, "main_form.pas")
				otherURI := findCapabilitiesRegressionOpenedURIBySuffix(openedURIs, "other_form.pas")

				result := []map[string]any{
					{
						"name":          "FormCreate",
						"kind":          6,
						"containerName": "TOtherForm",
						"location": map[string]any{
							"uri": otherURI,
							"range": map[string]any{
								"start": map[string]any{"line": 0, "character": 22},
								"end":   map[string]any{"line": 0, "character": 32},
							},
						},
					},
					{
						"name":          "FormCreate",
						"kind":          6,
						"containerName": "TFormMain",
						"location": map[string]any{
							"uri": mainURI,
							"range": map[string]any{
								"start": map[string]any{"line": 0, "character": 21},
								"end":   map[string]any{"line": 0, "character": 31},
							},
						},
					},
				}
				sendCapabilitiesRegressionFakeResponse(writer, msg.ID, result, nil)
				continue
			}

			if openedURI == "" {
				sendCapabilitiesRegressionFakeResponse(writer, msg.ID, []map[string]any{}, nil)
				continue
			}

			if qualifiedContainerSymbol {
				result := []map[string]any{
					{
						"name":          "ASNEncOIDItem",
						"kind":          12,
						"containerName": "asn1util",
						"location": map[string]any{
							"uri": openedURI,
							"range": map[string]any{
								"start": map[string]any{"line": 2, "character": 9},
								"end":   map[string]any{"line": 2, "character": 22},
							},
						},
					},
				}
				sendCapabilitiesRegressionFakeResponse(writer, msg.ID, result, nil)
				continue
			}

			result := []map[string]any{
				{
					"name": "Target",
					"kind": 12,
					"location": map[string]any{
						"uri": openedURI,
						"range": map[string]any{
							"start": map[string]any{"line": 2, "character": 5},
							"end":   map[string]any{"line": 2, "character": 11},
						},
					},
				},
			}
			sendCapabilitiesRegressionFakeResponse(writer, msg.ID, result, nil)
		case "textDocument/definition":
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &params)

			uri := params.TextDocument.URI
			if uri == "" {
				uri = openedURI
			}

			result := []map[string]any{
				{
					"uri": uri,
					"range": map[string]any{
						"start": map[string]any{"line": 2, "character": 5},
						"end":   map[string]any{"line": 2, "character": 11},
					},
				},
			}
			sendCapabilitiesRegressionFakeResponse(writer, msg.ID, result, nil)
		case "textDocument/references":
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &params)

			uri := params.TextDocument.URI
			if uri == "" {
				uri = openedURI
			}
			referenceURIs = append(referenceURIs, uri)

			if referencesRetryScenario {
				declarationURI := findCapabilitiesRegressionOpenedURIBySuffix(openedURIs, "uHighlighterProcs.pas")
				fallbackURI := findCapabilitiesRegressionOpenedURIBySuffix(openedURIs, "EditU2.pas")

				switch uri {
				case declarationURI:
					sendCapabilitiesRegressionFakeResponse(writer, msg.ID, []map[string]any{}, nil)
					continue
				case fallbackURI:
					result := []map[string]any{
						{
							"uri": fallbackURI,
							"range": map[string]any{
								"start": map[string]any{"line": 4, "character": 2},
								"end":   map[string]any{"line": 4, "character": 23},
							},
						},
					}
					sendCapabilitiesRegressionFakeResponse(writer, msg.ID, result, nil)
					continue
				}
			}

			result := []map[string]any{
				{
					"uri": uri,
					"range": map[string]any{
						"start": map[string]any{"line": 0, "character": 0},
						"end":   map[string]any{"line": 0, "character": 10},
					},
				},
				{
					"uri": uri,
					"range": map[string]any{
						"start": map[string]any{"line": 2, "character": 2},
						"end":   map[string]any{"line": 2, "character": 9},
					},
				},
			}
			sendCapabilitiesRegressionFakeResponse(writer, msg.ID, result, nil)
		case "mcp/getCallCounts":
			sendCapabilitiesRegressionFakeResponse(writer, msg.ID, counts, nil)
		case "mcp/getDocumentSymbolURIs":
			sendCapabilitiesRegressionFakeResponse(writer, msg.ID, documentSymbolURIs, nil)
		case "mcp/getReferenceURIs":
			sendCapabilitiesRegressionFakeResponse(writer, msg.ID, referenceURIs, nil)
		default:
			sendCapabilitiesRegressionFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32601, Message: "method not found: " + msg.Method})
		}
	}
}

func findCapabilitiesRegressionOpenedURIBySuffix(openedURIs []string, suffix string) string {
	for _, uri := range openedURIs {
		if strings.HasSuffix(uri, suffix) {
			return uri
		}
	}

	return ""
}

func sendCapabilitiesRegressionFakeResponse(writer *os.File, id *lsp.MessageID, result any, respErr *lsp.ResponseError) {
	resp := &lsp.Message{
		JSONRPC: "2.0",
		ID:      id,
	}

	if respErr != nil {
		resp.Error = respErr
	} else {
		resp.Result = mustMarshal(result)
	}

	_ = lsp.WriteMessage(writer, resp)
}
