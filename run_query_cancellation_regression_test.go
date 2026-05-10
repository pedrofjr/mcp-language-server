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

	assertRunQueryErrorContainsActionableMarker(t, resultBytes, "run_query canceled-before-execution")
	assertRunQueryErrorContainsOpToken(t, resultBytes, "OP_RUN_QUERY_CANCELED", "run_query canceled-before-execution")
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

	assertRunQueryErrorContainsActionableMarker(t, resultBytes, "run_query expired-deadline")
	assertRunQueryErrorContainsOpToken(t, resultBytes, "OP_RUN_QUERY_DEADLINE", "run_query expired-deadline")
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

	assertRunQueryErrorContainsActionableMarker(t, resultBytes, "run_query cooperative checkpoint cancellation")
	if !strings.Contains(string(resultBytes), "OP_RUN_QUERY_CANCELED") && !strings.Contains(string(resultBytes), "OP_RUN_QUERY_DEADLINE") {
		t.Fatalf("expected run_query cooperative checkpoint cancellation error to include OP_RUN_QUERY_CANCELED or OP_RUN_QUERY_DEADLINE, got %s", string(resultBytes))
	}
}

func TestRegisterTools_RunQuery_ExplicitTimeout_WhenScannerIsSlow(t *testing.T) {
	originalTimeout := runQueryHandlerTimeout
	runQueryHandlerTimeout = 40 * time.Millisecond
	t.Cleanup(func() {
		runQueryHandlerTimeout = originalTimeout
	})

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to capture working directory: %v", err)
	}

	tempDir := t.TempDir()
	for i := 0; i < 120; i++ {
		filePath := filepath.Join(tempDir, "slow_scan_"+formatRunQueryCancellationIndex(i)+".pas")
		content := strings.Repeat("procedure SlowProc"+formatRunQueryCancellationIndex(i)+";\n", 300)
		if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
			t.Fatalf("failed to write slow scan fixture %q: %v", filePath, err)
		}
	}

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change working directory for run_query explicit-timeout test: %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(originalWD); chdirErr != nil {
			t.Fatalf("failed to restore working directory: %v", chdirErr)
		}
	})

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	gate := make(chan struct{})
	releaseFallback := make(chan struct{})
	defer close(releaseFallback)

	// Gate the first checkpoint to force a deterministic slow scan start.
	originalHook := runQueryCheckpointHook
	var checkpointCount atomic.Int32
	runQueryCheckpointHook = func() {
		if checkpointCount.Add(1) == 1 {
			select {
			case <-gate:
			case <-releaseFallback:
			}
		}
	}
	t.Cleanup(func() {
		runQueryCheckpointHook = originalHook
	})

	go func() {
		time.Sleep(250 * time.Millisecond)
		close(gate)
	}()

	callResp := handleTestMCPRequestWithContext(
		t,
		svc,
		context.Background(),
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query": "TokenThatShouldNotExistAnywhere",
				"limit": 10000,
			},
		},
		4004,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal run_query explicit-timeout result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode run_query explicit-timeout result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatalf("expected run_query explicit local timeout to return tool error, got %s", string(resultBytes))
	}

	if checkpointCount.Load() == 0 {
		t.Fatalf("expected run_query explicit-timeout test to hit checkpoint hook at least once")
	}

	lower := strings.ToLower(string(resultBytes))
	if !strings.Contains(lower, "deadline exceeded") {
		t.Fatalf("expected run_query explicit local timeout error mentioning deadline exceeded, got %s", string(resultBytes))
	}

	if !regexp.MustCompile(`(?i)failed:\s*run_query\s+deadline\s+exceeded:`).Match(resultBytes) {
		t.Fatalf("expected explicit local timeout to follow handler error contract 'failed: run_query deadline exceeded:', got %s", string(resultBytes))
	}

	assertRunQueryErrorContainsActionableMarker(t, resultBytes, "run_query explicit local timeout")
	assertRunQueryErrorContainsOpToken(t, resultBytes, "OP_RUN_QUERY_DEADLINE", "run_query explicit local timeout")
}

func TestRegisterTools_RunQuery_RequestDeadlinePrecedence_WhenSmallerThanLocalTimeout(t *testing.T) {
	originalTimeout := runQueryHandlerTimeout
	runQueryHandlerTimeout = 2 * time.Second
	t.Cleanup(func() {
		runQueryHandlerTimeout = originalTimeout
	})

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to capture working directory: %v", err)
	}

	tempDir := t.TempDir()
	for i := 0; i < 120; i++ {
		filePath := filepath.Join(tempDir, "slow_scan_deadline_precedence_"+formatRunQueryCancellationIndex(i)+".pas")
		content := strings.Repeat("procedure SlowDeadlinePrecedenceProc"+formatRunQueryCancellationIndex(i)+";\n", 300)
		if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
			t.Fatalf("failed to write slow scan fixture %q: %v", filePath, err)
		}
	}

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change working directory for run_query request-deadline precedence test: %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(originalWD); chdirErr != nil {
			t.Fatalf("failed to restore working directory: %v", chdirErr)
		}
	})

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	gate := make(chan struct{})
	releaseFallback := make(chan struct{})
	defer close(releaseFallback)

	originalHook := runQueryCheckpointHook
	var checkpointCount atomic.Int32
	runQueryCheckpointHook = func() {
		if checkpointCount.Add(1) == 1 {
			select {
			case <-gate:
			case <-releaseFallback:
			}
		}
	}
	t.Cleanup(func() {
		runQueryCheckpointHook = originalHook
	})

	go func() {
		time.Sleep(250 * time.Millisecond)
		close(gate)
	}()

	requestCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	start := time.Now()
	callResp := handleTestMCPRequestWithContext(
		t,
		svc,
		requestCtx,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "run_query",
			"arguments": map[string]any{
				"query": "TokenThatShouldNotExistAnywhere",
				"limit": 10000,
			},
		},
		4005,
	)
	elapsed := time.Since(start)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal run_query request-deadline precedence result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode run_query request-deadline precedence result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if !isError {
		t.Fatalf("expected run_query request deadline precedence to return tool error, got %s", string(resultBytes))
	}

	if checkpointCount.Load() == 0 {
		t.Fatalf("expected run_query request-deadline precedence test to hit checkpoint hook at least once")
	}

	lower := strings.ToLower(string(resultBytes))
	if !strings.Contains(lower, "deadline exceeded") {
		t.Fatalf("expected run_query request-deadline precedence error mentioning deadline exceeded, got %s", string(resultBytes))
	}

	if !regexp.MustCompile(`(?i)failed:\s*run_query\s+deadline\s+exceeded:`).Match(resultBytes) {
		t.Fatalf("expected request deadline precedence to follow handler error contract 'failed: run_query deadline exceeded:', got %s", string(resultBytes))
	}

	assertRunQueryErrorContainsActionableMarker(t, resultBytes, "run_query request-deadline precedence")
	assertRunQueryErrorContainsOpToken(t, resultBytes, "OP_RUN_QUERY_DEADLINE", "run_query request-deadline precedence")

	if elapsed >= 1*time.Second {
		t.Fatalf("expected run_query to honor the smaller request deadline and fail well before local timeout=%s; elapsed=%s", runQueryHandlerTimeout, elapsed)
	}
}

func formatRunQueryCancellationIndex(i int) string {
	return fmt.Sprintf("%03d", i)
}

func assertRunQueryErrorContainsActionableMarker(t *testing.T, resultBytes []byte, scenario string) {
	t.Helper()

	if !strings.Contains(strings.ToLower(string(resultBytes)), "action:") {
		t.Fatalf("expected %s error to include actionable marker 'action:', got %s", scenario, string(resultBytes))
	}
}

func assertRunQueryErrorContainsOpToken(t *testing.T, resultBytes []byte, opToken string, scenario string) {
	t.Helper()

	if !strings.Contains(string(resultBytes), opToken) {
		t.Fatalf("expected %s error to include operational prefix %q, got %s", scenario, opToken, string(resultBytes))
	}
}
