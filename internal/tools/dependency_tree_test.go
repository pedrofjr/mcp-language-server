package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
)

const dependencyTreeFakeLSPEnv = "MCP_FAKE_LSP_DEPENDENCY_TREE"

func TestHelperProcessDependencyTreeFakeLSP(t *testing.T) {
	if os.Getenv(dependencyTreeFakeLSPEnv) != "1" {
		return
	}

	runDependencyTreeFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestGetDependencyTree_WhenImportedByIncludesTreeBySection_PreservesSectionBuckets(t *testing.T) {
	client, cleanup := setupDependencyTreeFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	out, err := GetDependencyTree(
		ctx,
		client,
		"file:///workspace/Foo.pas",
		"importedBy",
	)
	if err != nil {
		t.Fatalf("GetDependencyTree nao deveria falhar com fake LSP: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("saida deve ser JSON valido: %v\noutput=%s", err, out)
	}

	treeRaw, ok := payload["tree"].(map[string]any)
	if !ok {
		t.Fatalf("payload deve conter objeto tree, got=%#v", payload["tree"])
	}
	fooImportersRaw, ok := treeRaw["foo"].([]any)
	if !ok {
		t.Fatalf("payload deve conter tree['foo'] como array, got=%#v", treeRaw["foo"])
	}
	if len(fooImportersRaw) != 2 {
		t.Fatalf("tree['foo'] deve conter exatamente 2 itens (bar e baz), got=%#v", fooImportersRaw)
	}

	gotImporter := map[string]bool{}
	for _, entry := range fooImportersRaw {
		name, _ := entry.(string)
		gotImporter[name] = true
	}
	if !gotImporter["bar"] || !gotImporter["baz"] {
		t.Fatalf("tree['foo'] deve preservar bar e baz, got=%#v", fooImportersRaw)
	}

	bySectionRaw, ok := payload["treeBySection"].(map[string]any)
	if !ok {
		t.Fatalf("payload deve conter objeto treeBySection, got=%#v", payload["treeBySection"])
	}
	fooSectionsRaw, ok := bySectionRaw["foo"].(map[string]any)
	if !ok {
		t.Fatalf("payload deve conter treeBySection['foo'] como objeto, got=%#v", bySectionRaw["foo"])
	}

	interfaceRaw, ok := fooSectionsRaw["interface"].([]any)
	if !ok {
		t.Fatalf("payload deve conter treeBySection['foo'].interface como array, got=%#v", fooSectionsRaw["interface"])
	}
	implementationRaw, ok := fooSectionsRaw["implementation"].([]any)
	if !ok {
		t.Fatalf("payload deve conter treeBySection['foo'].implementation como array, got=%#v", fooSectionsRaw["implementation"])
	}

	if len(interfaceRaw) != 1 || interfaceRaw[0] != "bar" {
		t.Fatalf("treeBySection['foo'].interface deve conter apenas bar, got=%#v", interfaceRaw)
	}
	if len(implementationRaw) != 1 || implementationRaw[0] != "baz" {
		t.Fatalf("treeBySection['foo'].implementation deve conter apenas baz, got=%#v", implementationRaw)
	}
}

func setupDependencyTreeFakeClient(t *testing.T) (*lsp.Client, func()) {
	t.Helper()

	t.Setenv(dependencyTreeFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("falha ao resolver binario de teste: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessDependencyTreeFakeLSP")
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

func runDependencyTreeFakeLSP(stdin *os.File, stdout *os.File) {
	reader := bufio.NewReader(stdin)
	writer := stdout

	for {
		msg, err := lsp.ReadMessage(reader)
		if err != nil {
			return
		}

		switch msg.Method {
		case "initialize":
			result := map[string]any{"capabilities": map[string]any{}}
			sendDependencyTreeFakeResponse(writer, msg.ID, result, nil)
		case "initialized":
			// no-op
		case "custom/dependencyTree":
			var params struct {
				Direction string `json:"direction"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			if params.Direction != "importedBy" {
				sendDependencyTreeFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32602, Message: "direction must be importedBy in this test"})
				continue
			}

			result := map[string]any{
				"root": "foo",
				"tree": map[string]any{
					"foo": []string{"bar", "baz"},
				},
				"treeBySection": map[string]any{
					"foo": map[string]any{
						"interface":      []string{"bar"},
						"implementation": []string{"baz"},
					},
				},
				"cycles": []string{},
			}
			sendDependencyTreeFakeResponse(writer, msg.ID, result, nil)
		case "shutdown":
			sendDependencyTreeFakeResponse(writer, msg.ID, nil, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendDependencyTreeFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32601, Message: "method not found: " + msg.Method})
			}
		}
	}
}

func sendDependencyTreeFakeResponse(w *os.File, id *lsp.MessageID, result any, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
	}

	if rpcErr == nil {
		if result == nil {
			resp.Result = mustMarshalDependencyTreeFake(struct{}{})
		} else {
			resp.Result = mustMarshalDependencyTreeFake(result)
		}
	}

	_ = lsp.WriteMessage(w, resp)
}

func mustMarshalDependencyTreeFake(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal fake LSP json: %v", err))
	}
	return b
}
