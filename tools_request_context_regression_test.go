package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/mark3labs/mcp-go/mcp"
)

const registerToolsRequestCtxFakeLSPEnv = "MCP_FAKE_LSP_REGISTER_TOOLS_REQUEST_CTX"
const registerToolsRequestCtxFakeLSPFixturePathEnv = "MCP_FAKE_LSP_REGISTER_TOOLS_REQUEST_CTX_FIXTURE"
const registerToolsRequestCtxFakeLSPDelayMSEnv = "MCP_FAKE_LSP_REGISTER_TOOLS_REQUEST_CTX_DELAY_MS"

func TestRegisterTools_Definition_UsesRequestContext_WhenCanceledBeforeExecution(t *testing.T) {
	svc := newRegisteredTestMCPServerWithContextFakeLSP(t)
	initializeTestMCPServer(t, svc)

	requestCtx, cancel := context.WithCancel(context.Background())
	cancel()

	callResp := handleTestMCPRequestWithContext(
		t,
		svc,
		requestCtx,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "definition",
			"arguments": map[string]any{
				"symbolName": "TargetSymbol",
			},
		},
		600,
	)

	assertToolCallResultContainsContextError(t, callResp, "canceled")
}

func TestRegisterTools_References_UsesRequestContext_WhenCanceledBeforeExecution(t *testing.T) {
	svc := newRegisteredTestMCPServerWithContextFakeLSP(t)
	initializeTestMCPServer(t, svc)

	requestCtx, cancel := context.WithCancel(context.Background())
	cancel()

	callResp := handleTestMCPRequestWithContext(
		t,
		svc,
		requestCtx,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "references",
			"arguments": map[string]any{
				"symbolName": "TargetSymbol",
			},
		},
		601,
	)

	assertToolCallResultContainsContextError(t, callResp, "canceled")
}

func TestRegisterTools_DefinitionAndReferences_UsesRequestDeadline_WhenExpired(t *testing.T) {
	svc := newRegisteredTestMCPServerWithContextFakeLSP(t)
	initializeTestMCPServer(t, svc)

	toolsToCheck := []string{"definition", "references"}
	for i, toolName := range toolsToCheck {
		requestCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))

		callResp := handleTestMCPRequestWithContext(
			t,
			svc,
			requestCtx,
			mcp.MethodToolsCall,
			map[string]any{
				"name": toolName,
				"arguments": map[string]any{
					"symbolName": "TargetSymbol",
				},
			},
			610+i,
		)

		assertToolCallResultContainsContextError(t, callResp, "deadline exceeded")
		cancel()
	}
}

func newRegisteredTestMCPServerWithContextFakeLSP(t *testing.T) *mcpServer {
	t.Helper()

	workspaceDir := t.TempDir()
	fixturePath := filepath.Join(workspaceDir, "Unit1.pas")
	fixtureContent := "unit Unit1;\ninterface\nprocedure TargetSymbol;\nimplementation\nprocedure TargetSymbol; begin end;\nend.\n"
	if err := os.WriteFile(fixturePath, []byte(fixtureContent), 0o644); err != nil {
		t.Fatalf("failed to create Delphi fixture for request-context tests: %v", err)
	}

	client := newRegisterToolsRequestContextFakeLSPClient(t, workspaceDir, fixturePath)

	svc := &mcpServer{ctx: context.Background(), lspClient: client}
	svc.mcpServer = newMCPServer()

	if err := svc.registerTools(); err != nil {
		t.Fatalf("registerTools() returned error: %v", err)
	}

	return svc
}

func newRegisterToolsRequestContextFakeLSPClient(t *testing.T, workspaceDir string, fixturePath string) *lsp.Client {
	t.Helper()

	t.Setenv(registerToolsRequestCtxFakeLSPEnv, "1")
	t.Setenv(registerToolsRequestCtxFakeLSPFixturePathEnv, fixturePath)
	t.Setenv(registerToolsRequestCtxFakeLSPDelayMSEnv, "100")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessRegisterToolsRequestContextFakeLSP")
	if err != nil {
		t.Fatalf("failed to start fake LSP for request-context tests: %v", err)
	}

	t.Cleanup(func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		t.Fatalf("InitializeLSPClient() returned error in request-context test setup: %v", err)
	}

	return client
}

func TestHelperProcessRegisterToolsRequestContextFakeLSP(t *testing.T) {
	if os.Getenv(registerToolsRequestCtxFakeLSPEnv) != "1" {
		return
	}

	runRegisterToolsRequestContextFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func runRegisterToolsRequestContextFakeLSP(stdin *os.File, stdout *os.File) {
	reader := bufio.NewReader(stdin)
	writer := stdout

	fixturePath := os.Getenv(registerToolsRequestCtxFakeLSPFixturePathEnv)
	if fixturePath == "" {
		fixturePath = filepath.Join(os.TempDir(), "Unit1.pas")
	}

	delay := 100 * time.Millisecond
	if rawDelay := strings.TrimSpace(os.Getenv(registerToolsRequestCtxFakeLSPDelayMSEnv)); rawDelay != "" {
		if parsedDelay, err := strconv.Atoi(rawDelay); err == nil && parsedDelay >= 0 {
			delay = time.Duration(parsedDelay) * time.Millisecond
		}
	}

	location := map[string]any{
		"uri": string(protocol.URIFromPath(fixturePath)),
		"range": map[string]any{
			"start": map[string]any{"line": 2, "character": 10},
			"end":   map[string]any{"line": 2, "character": 22},
		},
	}

	for {
		msg, err := lsp.ReadMessage(reader)
		if err != nil {
			return
		}

		switch msg.Method {
		case "initialize":
			capabilities := map[string]any{
				"workspaceSymbolProvider": true,
			}
			sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, map[string]any{"capabilities": capabilities}, nil)
		case "initialized":
			continue
		case "workspace/symbol":
			time.Sleep(delay)

			query := "TargetSymbol"
			var params map[string]any
			_ = json.Unmarshal(msg.Params, &params)
			if rawQuery, ok := params["query"].(string); ok && strings.TrimSpace(rawQuery) != "" {
				query = rawQuery
			}

			results := []map[string]any{
				{
					"name":          query,
					"kind":          12,
					"location":      location,
					"containerName": "Unit1",
				},
			}
			sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, results, nil)
		case "textDocument/references":
			time.Sleep(delay)
			refs := []map[string]any{location}
			sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, refs, nil)
		case "shutdown":
			sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, map[string]any{}, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, map[string]any{}, nil)
			}
		}
	}
}

func sendRegisterToolsRequestContextFakeLSPResponse(w *os.File, id *lsp.MessageID, result any, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{JSONRPC: "2.0", ID: id, Error: rpcErr}
	if rpcErr == nil {
		resp.Result = mustMarshalRegisterToolsRequestContextFake(result)
	}
	_ = lsp.WriteMessage(w, resp)
}

func mustMarshalRegisterToolsRequestContextFake(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal request-context fake LSP json: %v", err))
	}
	return b
}

func assertToolCallResultContainsContextError(t *testing.T, response mcp.JSONRPCResponse, expectedErrorSubstr string) {
	t.Helper()

	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		t.Fatalf("failed to marshal tool call result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode tool call result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatalf("expected tool error containing %q, but result was not an error: %s", expectedErrorSubstr, string(resultBytes))
	}

	lowerResult := strings.ToLower(string(resultBytes))
	if !strings.Contains(lowerResult, strings.ToLower(expectedErrorSubstr)) {
		t.Fatalf("expected tool error to contain %q, got %s", expectedErrorSubstr, string(resultBytes))
	}
}
