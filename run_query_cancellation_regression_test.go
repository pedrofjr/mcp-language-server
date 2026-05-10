package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync/atomic"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func handleTestMCPRequestWithContext(t *testing.T, svc *mcpServer, ctx context.Context, method mcp.MCPMethod, params map[string]any, id int) mcp.JSONRPCResponse {
	t.Helper()

	request := mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      id,
		Request: mcp.Request{Method: string(method)},
		Params:  params,
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("json.Marshal(%s request) returned error: %v", method, err)
	}

	response := svc.mcpServer.HandleMessage(ctx, requestBytes)
	rpcResp, ok := response.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("HandleMessage() for %s returned %T, want mcp.JSONRPCResponse", method, response)
	}

	return rpcResp
}

func TestRegisterTools_RunQuery_ContextCanceledBeforeExecution_ReturnsDeterministicCanceledError(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	callResp := handleTestMCPRequestWithContext(
		t,
		svc,
		ctx,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query": "procedure",
				"limit": 5,
			},
		},
		4001,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal run_query canceled-before-execution result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode run_query canceled-before-execution result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatalf("expected run_query with pre-canceled context to return tool error, got %s", string(resultBytes))
	}

	if !strings.Contains(strings.ToLower(string(resultBytes)), "canceled") {
		t.Fatalf("expected deterministic cancellation error mentioning canceled, got %s", string(resultBytes))
	}

	if !regexp.MustCompile(`(?i)failed:\s*run_query\s+canceled:`).Match(resultBytes) {
		t.Fatalf("expected handler error contract to include 'failed: run_query canceled:', got %s", string(resultBytes))
	}
}

func TestRegisterTools_RunQuery_DeadlineAlreadyExceeded_ReturnsDeterministicDeadlineExceededError(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	callResp := handleTestMCPRequestWithContext(
		t,
		svc,
		ctx,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query": "procedure",
				"limit": 5,
			},
		},
		4002,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal run_query expired-deadline result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode run_query expired-deadline result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatalf("expected run_query with expired deadline to return tool error, got %s", string(resultBytes))
	}

	if !strings.Contains(strings.ToLower(string(resultBytes)), "deadline") {
		t.Fatalf("expected deterministic deadline exceeded error mentioning deadline, got %s", string(resultBytes))
	}

	if !regexp.MustCompile(`(?i)failed:\s*run_query\s+deadline\s+exceeded:`).Match(resultBytes) {
		t.Fatalf("expected handler error contract to include 'failed: run_query deadline exceeded:', got %s", string(resultBytes))
	}
}

func TestRegisterTools_RunQuery_CooperativeCheckpoint_RespectsContextCancellationDuringScan(t *testing.T) {
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to capture working directory: %v", err)
	}

	tempDir := t.TempDir()
	for i := 0; i < 120; i++ {
		filePath := filepath.Join(tempDir, "scan_target_"+formatRunQueryCancellationIndex(i)+".pas")
		content := strings.Repeat("procedure SampleProc"+formatRunQueryCancellationIndex(i)+";\n", 400)
		if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
			t.Fatalf("failed to write scan fixture %q: %v", filePath, err)
		}
	}

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change working directory for run_query cancellation scan test: %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(originalWD); chdirErr != nil {
			t.Fatalf("failed to restore working directory: %v", chdirErr)
		}
	})

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	originalHook := runQueryCheckpointHook
	var checkpointCount atomic.Int32
	runQueryCheckpointHook = func() {
		if checkpointCount.Add(1) == 8 {
			cancel()
		}
	}
	t.Cleanup(func() {
		runQueryCheckpointHook = originalHook
	})

	callResp := handleTestMCPRequestWithContext(
		t,
		svc,
		ctx,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query": "TokenThatShouldNotExistAnywhere",
				"limit": 10000,
			},
		},
		4003,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal run_query cooperative-cancel result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode run_query cooperative-cancel result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatalf("expected run_query to stop with tool error when context is canceled during scan, got %s", string(resultBytes))
	}

	if checkpointCount.Load() < 8 {
		t.Fatalf("expected cooperative cancellation to happen after scan started; checkpoint count=%d", checkpointCount.Load())
	}

	lower := strings.ToLower(string(resultBytes))
	if !strings.Contains(lower, "canceled") && !strings.Contains(lower, "deadline") {
		t.Fatalf("expected cooperative checkpoint cancellation/deadline error, got %s", string(resultBytes))
	}

	if !regexp.MustCompile(`(?i)failed:\s*run_query\s+(canceled|deadline\s+exceeded):`).Match(resultBytes) {
		t.Fatalf("expected cooperative cancellation to follow handler error contract 'failed: run_query ...', got %s", string(resultBytes))
	}
}

func formatRunQueryCancellationIndex(i int) string {
	return fmt.Sprintf("%03d", i)
}
