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

const workspaceSymbolsFakeLSPEnv = "MCP_FAKE_LSP_WORKSPACE_SYMBOLS"

func TestHelperProcessWorkspaceSymbolsFakeLSP(t *testing.T) {
	if os.Getenv(workspaceSymbolsFakeLSPEnv) != "1" {
		return
	}

	runWorkspaceSymbolsFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestGetWorkspaceSymbols_WhenLSPReturnsStubMarker_ExposesExplicitIsStubInOutput(t *testing.T) {
	client, cleanup := setupWorkspaceSymbolsFakeClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	result, err := GetWorkspaceSymbols(ctx, client, "Trim")
	if err != nil {
		t.Fatalf("GetWorkspaceSymbols nao deveria falhar com fake LSP: %v", err)
	}

	if !strings.Contains(result, "Trim") {
		t.Fatalf("saida deve conter o simbolo retornado pelo LSP (Trim), obtido: %s", result)
	}

	if !strings.Contains(result, "file:///__oracle_stubs__/SysUtils.pas:1:1") {
		t.Fatalf("saida deve conter localizacao do simbolo de stub, obtido: %s", result)
	}

	if !strings.Contains(result, "isStub=true") {
		t.Fatalf("saida deve expor marcador explicito de stub vindo do LSP (isStub=true), obtido: %s", result)
	}
}

func setupWorkspaceSymbolsFakeClient(t *testing.T) (*lsp.Client, func()) {
	t.Helper()

	t.Setenv(workspaceSymbolsFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("falha ao resolver binario de teste: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessWorkspaceSymbolsFakeLSP")
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

func runWorkspaceSymbolsFakeLSP(stdin *os.File, stdout *os.File) {
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
			sendWorkspaceSymbolsFakeResponse(writer, msg.ID, result, nil)
		case "initialized":
			// no-op
		case "workspace/symbol":
			result := []map[string]any{
				{
					"name": "Trim",
					"kind": float64(protocol.Function),
					"location": map[string]any{
						"uri": "file:///__oracle_stubs__/SysUtils.pas",
						"range": map[string]any{
							"start": map[string]any{"line": 0.0, "character": 0.0},
							"end":   map[string]any{"line": 0.0, "character": 0.0},
						},
					},
					"containerName": "SysUtils",
					"data":          map[string]any{"isStub": true},
				},
			}
			sendWorkspaceSymbolsFakeResponse(writer, msg.ID, result, nil)
		case "shutdown":
			sendWorkspaceSymbolsFakeResponse(writer, msg.ID, nil, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendWorkspaceSymbolsFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32601, Message: "method not found: " + msg.Method})
			}
		}
	}
}

func sendWorkspaceSymbolsFakeResponse(w *os.File, id *lsp.MessageID, result any, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
	}

	if rpcErr == nil {
		if result == nil {
			resp.Result = mustMarshalWorkspaceSymbolsFake(struct{}{})
		} else {
			resp.Result = mustMarshalWorkspaceSymbolsFake(result)
		}
	}

	_ = lsp.WriteMessage(w, resp)
}

func mustMarshalWorkspaceSymbolsFake(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal fake LSP json: %v", err))
	}
	return b
}
