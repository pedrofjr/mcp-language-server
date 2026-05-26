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

func TestRegisterTools_Definition_ContextCanceledBeforeExecution_ReturnsDeterministicCanceledError(t *testing.T) {
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

	assertToolCallResultContainsDeterministicToolError(
		t,
		callResp,
		"failed: definition canceled: context canceled",
		"OP_DEFINITION_CANCELED",
	)
}

func TestRegisterTools_Definition_DeadlineAlreadyExceeded_ReturnsDeterministicDeadlineExceededError(t *testing.T) {
	svc := newRegisteredTestMCPServerWithContextFakeLSP(t)
	initializeTestMCPServer(t, svc)

	requestCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

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
		601,
	)

	assertToolCallResultContainsDeterministicToolError(
		t,
		callResp,
		"failed: definition deadline exceeded: context deadline exceeded",
		"OP_DEFINITION_DEADLINE",
	)
}

func TestRegisterTools_References_ContextCanceledBeforeExecution_ReturnsDeterministicCanceledError(t *testing.T) {
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
		602,
	)

	assertToolCallResultContainsDeterministicToolError(
		t,
		callResp,
		"failed: references canceled: context canceled",
		"OP_REFERENCES_CANCELED",
	)
}

func TestRegisterTools_References_DeadlineAlreadyExceeded_ReturnsDeterministicDeadlineExceededError(t *testing.T) {
	svc := newRegisteredTestMCPServerWithContextFakeLSP(t)
	initializeTestMCPServer(t, svc)

	requestCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

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
		603,
	)

	assertToolCallResultContainsDeterministicToolError(
		t,
		callResp,
		"failed: references deadline exceeded: context deadline exceeded",
		"OP_REFERENCES_DEADLINE",
	)
}

func TestRegisterTools_Definition_ExplicitTimeout_WhenLSPIsSlow(t *testing.T) {
	originalTimeout := definitionReferencesHandlerTimeout
	definitionReferencesHandlerTimeout = 100 * time.Millisecond
	t.Cleanup(func() {
		definitionReferencesHandlerTimeout = originalTimeout
	})

	svc := newRegisteredTestMCPServerWithContextFakeLSPDelay(t, 750*time.Millisecond)
	initializeTestMCPServer(t, svc)

	requestCtx := context.Background()

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
		620,
	)

	assertToolCallResultContainsDeadlineExceededToolError(t, callResp, "definition")
}

func TestRegisterTools_References_ExplicitTimeout_WhenLSPIsSlow(t *testing.T) {
	originalTimeout := definitionReferencesHandlerTimeout
	definitionReferencesHandlerTimeout = 100 * time.Millisecond
	t.Cleanup(func() {
		definitionReferencesHandlerTimeout = originalTimeout
	})

	svc := newRegisteredTestMCPServerWithContextFakeLSPDelay(t, 750*time.Millisecond)
	initializeTestMCPServer(t, svc)

	requestCtx := context.Background()

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
		621,
	)

	assertToolCallResultContainsDeadlineExceededToolError(t, callResp, "references")
}

func TestRegisterTools_AllLSPBackedTools_ContextCanceledBeforeExecution_ReturnsDeterministicCanceledError(t *testing.T) {
	svc := newRegisteredTestMCPServerWithContextFakeLSP(t)
	initializeTestMCPServer(t, svc)

	fixturePath := requestContextFixturePath(t, svc)
	cases := lspBackedToolRequestContextCases(fixturePath)

	requestCtx, cancel := context.WithCancel(context.Background())
	cancel()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			callResp := handleTestMCPRequestWithContext(
				t,
				svc,
				requestCtx,
				mcp.MethodToolsCall,
				map[string]any{
					"name":      tc.name,
					"arguments": tc.args,
				},
				tc.id,
			)

			assertToolCallResultContainsDeterministicToolError(
				t,
				callResp,
				fmt.Sprintf("failed: %s canceled: context canceled", tc.name),
				opTokenForToolEvent(tc.name, "CANCELED"),
			)
		})
	}
}

func TestRegisterTools_AllLSPBackedTools_ExplicitTimeout_WhenLSPIsSlow(t *testing.T) {
	originalTimeout := criticalLSPHandlerTimeout
	criticalLSPHandlerTimeout = 100 * time.Millisecond
	originalDefinitionTimeout := definitionReferencesHandlerTimeout
	definitionReferencesHandlerTimeout = 100 * time.Millisecond
	t.Cleanup(func() {
		criticalLSPHandlerTimeout = originalTimeout
		definitionReferencesHandlerTimeout = originalDefinitionTimeout
	})

	svc := newRegisteredTestMCPServerWithContextFakeLSPDelay(t, 750*time.Millisecond)
	initializeTestMCPServer(t, svc)

	fixturePath := requestContextFixturePath(t, svc)
	cases := lspBackedToolSlowLSPTimeoutCases(fixturePath)
	requestCtx := context.Background()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			callResp := handleTestMCPRequestWithContext(
				t,
				svc,
				requestCtx,
				mcp.MethodToolsCall,
				map[string]any{
					"name":      tc.name,
					"arguments": tc.args,
				},
				tc.id+100,
			)
			elapsed := time.Since(start)

			assertToolCallResultContainsDeadlineExceededToolError(t, callResp, tc.name)
			if elapsed >= 500*time.Millisecond {
				t.Fatalf("expected %s to fail before global timeout; elapsed=%s", tc.name, elapsed)
			}
		})
	}
}

func TestRegisterTools_RemainingLSPBackedTools_ContextCanceledBeforeExecution_ReturnsDeterministicCanceledError(t *testing.T) {
	TestRegisterTools_AllLSPBackedTools_ContextCanceledBeforeExecution_ReturnsDeterministicCanceledError(t)
}

func TestRegisterTools_RemainingLSPBackedTools_ExplicitTimeout_WhenLSPIsSlow(t *testing.T) {
	TestRegisterTools_AllLSPBackedTools_ExplicitTimeout_WhenLSPIsSlow(t)
}

func requestContextFixturePath(t *testing.T, svc *mcpServer) string {
	t.Helper()

	workspaceDir := svc.config.workspaceDir
	if workspaceDir == "" {
		t.Fatal("expected workspaceDir on test MCP server")
	}

	return filepath.Join(workspaceDir, "Unit1.pas")
}

func TestRegisterTools_DefinitionAndReferences_RequestDeadlinePrecedence_WhenSmallerThanLocalTimeout(t *testing.T) {
	svc := newRegisteredTestMCPServerWithContextFakeLSPDelay(t, 750*time.Millisecond)
	initializeTestMCPServer(t, svc)

	toolsToCheck := []string{"definition", "references"}
	for i, toolName := range toolsToCheck {
		requestCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		start := time.Now()

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
			630+i,
		)

		elapsed := time.Since(start)
		cancel()

		assertToolCallResultContainsDeadlineExceededToolError(t, callResp, toolName)
		if elapsed >= 300*time.Millisecond {
			t.Fatalf("expected %s to honor the smaller request deadline and fail quickly; elapsed=%s", toolName, elapsed)
		}
	}
}

func newRegisteredTestMCPServerWithContextFakeLSP(t *testing.T) *mcpServer {
	t.Helper()

	return newRegisteredTestMCPServerWithContextFakeLSPDelay(t, 100*time.Millisecond)
}

func newRegisteredTestMCPServerWithContextFakeLSPDelay(t *testing.T, fakeDelay time.Duration) *mcpServer {
	t.Helper()

	workspaceDir := t.TempDir()
	fixturePath := filepath.Join(workspaceDir, "Unit1.pas")
	fixtureContent := "unit Unit1;\ninterface\ntype\n  IFoo = interface\n    procedure DoFoo;\n  end;\nprocedure TargetSymbol;\nimplementation\nprocedure TargetSymbolImpl; begin end;\nprocedure ZZZ_DEL_Only; begin end;\nend.\n"
	if err := os.WriteFile(fixturePath, []byte(fixtureContent), 0o644); err != nil {
		t.Fatalf("failed to create Delphi fixture for request-context tests: %v", err)
	}

	client := newRegisterToolsRequestContextFakeLSPClientWithDelay(t, workspaceDir, fixturePath, fakeDelay)

	svc := &mcpServer{
		ctx:       context.Background(),
		lspClient: client,
		config:    config{workspaceDir: workspaceDir},
	}
	svc.mcpServer = newMCPServer(workspaceDir)

	if err := svc.registerTools(); err != nil {
		t.Fatalf("registerTools() returned error: %v", err)
	}

	return svc
}

func newRegisterToolsRequestContextFakeLSPClient(t *testing.T, workspaceDir string, fixturePath string) *lsp.Client {
	t.Helper()

	return newRegisterToolsRequestContextFakeLSPClientWithDelay(t, workspaceDir, fixturePath, 100*time.Millisecond)
}

func newRegisterToolsRequestContextFakeLSPClientWithDelay(t *testing.T, workspaceDir string, fixturePath string, delay time.Duration) *lsp.Client {
	t.Helper()

	t.Setenv(registerToolsRequestCtxFakeLSPEnv, "1")
	t.Setenv(registerToolsRequestCtxFakeLSPFixturePathEnv, fixturePath)
	t.Setenv(registerToolsRequestCtxFakeLSPDelayMSEnv, strconv.Itoa(int(delay.Milliseconds())))

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
				"hoverProvider":           true,
				"codeActionProvider":      true,
			}
			sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, map[string]any{"capabilities": capabilities}, nil)
		case "initialized":
			continue
		case "textDocument/didOpen":
			if msg.ID != nil && msg.ID.Value != nil {
				sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, map[string]any{}, nil)
			}
		case "textDocument/hover":
			time.Sleep(delay)
			hover := map[string]any{
				"contents": map[string]any{
					"kind":  "plaintext",
					"value": "hover ok",
				},
			}
			sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, hover, nil)
		case "textDocument/diagnostic":
			time.Sleep(delay)
			sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, map[string]any{
				"items": []map[string]any{},
			}, nil)
		case "textDocument/codeAction":
			time.Sleep(delay)
			sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, []map[string]any{}, nil)
		case "custom/semanticSearch":
			time.Sleep(delay)
			sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, []map[string]any{}, nil)
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
		case "textDocument/rename":
			time.Sleep(delay)
			sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, map[string]any{}, nil)
		case "textDocument/implementation":
			time.Sleep(delay)
			sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, []map[string]any{}, nil)
		case "textDocument/didChange":
			if msg.ID != nil && msg.ID.Value != nil {
				time.Sleep(delay)
				sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, map[string]any{}, nil)
			}
		case "custom/astSummary":
			time.Sleep(delay)
			sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, map[string]any{"unit": "Unit1"}, nil)
		case "custom/dependencyTree":
			if registerToolsFakeLSPParamsForceError(msg) {
				sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, nil, &lsp.ResponseError{
					Code:    -32603,
					Message: "forced NFR LSP error",
				})
				break
			}
			time.Sleep(delay)
			sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, map[string]any{"tree": map[string]any{}}, nil)
		case "custom/graph/neighbors":
			time.Sleep(delay)
			sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, map[string]any{"neighbors": []any{}}, nil)
		case "custom/graph/node":
			time.Sleep(delay)
			sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, map[string]any{"node": map[string]any{}}, nil)
		case "custom/graph/query":
			if registerToolsFakeLSPParamsForceError(msg) {
				sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, nil, &lsp.ResponseError{
					Code:    -32603,
					Message: "forced NFR LSP error",
				})
				break
			}
			time.Sleep(delay)
			sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, map[string]any{"nodes": []any{}}, nil)
		case "custom/callGraph":
			time.Sleep(delay)
			sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, map[string]any{"calls": []any{}}, nil)
		case "shutdown":
			sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, map[string]any{}, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				time.Sleep(delay)
				sendRegisterToolsRequestContextFakeLSPResponse(writer, msg.ID, map[string]any{}, nil)
			}
		}
	}
}

func registerToolsFakeLSPParamsForceError(msg *lsp.Message) bool {
	if msg == nil || len(msg.Params) == 0 {
		return false
	}
	var params map[string]any
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return false
	}
	uri, _ := params["uri"].(string)
	return strings.Contains(uri, "nfr-force-lsp-error")
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

func assertToolCallResultContainsDeterministicToolError(t *testing.T, response mcp.JSONRPCResponse, expectedErrorText string, opToken string) {
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
		t.Fatalf("expected deterministic tool error %q, but result was not an error: %s", expectedErrorText, string(resultBytes))
	}

	if !strings.Contains(string(resultBytes), expectedErrorText) {
		t.Fatalf("expected tool error to contain exact contract %q, got %s", expectedErrorText, string(resultBytes))
	}

	if !strings.Contains(strings.ToLower(string(resultBytes)), "action:") {
		t.Fatalf("expected tool error to include actionable marker 'action:', got %s", string(resultBytes))
	}

	if opToken != "" && !strings.Contains(string(resultBytes), opToken) {
		t.Fatalf("expected tool error to include operational prefix %q, got %s", opToken, string(resultBytes))
	}
}

func assertToolCallResultContainsDeadlineExceededToolError(t *testing.T, response mcp.JSONRPCResponse, toolName string) {
	t.Helper()

	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		t.Fatalf("failed to marshal %s result: %v", toolName, err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode %s result map: %v", toolName, err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatalf("expected %s to return tool error, got %s", toolName, string(resultBytes))
	}

	expected := "failed: " + toolName + " deadline exceeded: context deadline exceeded"
	if !strings.Contains(string(resultBytes), expected) {
		t.Fatalf("expected %s tool error contract %q, got %s", toolName, expected, string(resultBytes))
	}

	if !strings.Contains(strings.ToLower(string(resultBytes)), "action:") {
		t.Fatalf("expected %s deadline/timeout error to include actionable marker 'action:', got %s", toolName, string(resultBytes))
	}

	opToken := opTokenForToolEvent(toolName, "DEADLINE")
	if !strings.Contains(string(resultBytes), opToken) {
		t.Fatalf("expected %s deadline error to include operational prefix %q, got %s", toolName, opToken, string(resultBytes))
	}
}
