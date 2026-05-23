package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/isaacphi/mcp-language-server/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
)

func setupToolLoggingCapture(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	prevLevel, ok := logging.ComponentLevels[logging.Core]
	if !ok {
		prevLevel = logging.DefaultMinLevel
	}

	logging.SetupTestLogging(&buf)
	logging.SetLevel(logging.Core, logging.LevelDebug)

	t.Cleanup(func() {
		logging.ResetTestLogging()
		logging.SetLevel(logging.Core, prevLevel)
	})

	return &buf
}

// TestWithToolLogging_WrapperExistsAndCallsUnderlyingHandler verifica que
// withToolLogging retorna um handler válido que delega ao handler interno.
func TestWithToolLogging_WrapperExistsAndCallsUnderlyingHandler(t *testing.T) {
	called := false
	inner := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return mcp.NewToolResultText("ok"), nil
	}
	wrapped := withToolLogging("test_tool", inner)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("inner handler was not called")
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

// TestWithToolLogging_PropagatesError verifica que erros do handler interno
// são propagados corretamente pelo wrapper.
func TestWithToolLogging_PropagatesError(t *testing.T) {
	expectedErr := errors.New("test error")
	logBuf := setupToolLoggingCapture(t)

	inner := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, expectedErr
	}
	wrapped := withToolLogging("failing_tool", inner)
	_, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	logs := logBuf.String()
	if !strings.Contains(logs, "tool=failing_tool outcome=error") {
		t.Fatalf("expected error log for failing_tool, got logs: %s", logs)
	}
}

func TestWithToolLogging_ResultErrorWithoutGoError_LogsAsOperationalErrorAndPreservesPayload(t *testing.T) {
	logBuf := setupToolLoggingCapture(t)
	expectedResult := mcp.NewToolResultError("tool-level failure")

	inner := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return expectedResult, nil
	}

	wrapped := withToolLogging("result_error_tool", inner)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("expected nil go error, got %v", err)
	}
	if result != expectedResult {
		t.Fatalf("expected same result pointer to be preserved")
	}
	if !result.IsError {
		t.Fatalf("expected result.IsError=true")
	}

	logs := logBuf.String()
	if !strings.Contains(logs, "tool=result_error_tool outcome=error") {
		t.Fatalf("expected outcome=error log when result.IsError=true, got logs: %s", logs)
	}
	if strings.Contains(logs, "tool=result_error_tool outcome=success") {
		t.Fatalf("did not expect outcome=success when result.IsError=true, got logs: %s", logs)
	}
}

func TestWithToolLogging_PrecedenceMatrix(t *testing.T) {
	goErr := errors.New("boom")

	tests := []struct {
		name             string
		result           *mcp.CallToolResult
		err              error
		expectErrorLog   bool
		expectSuccessLog bool
	}{
		{
			name:             "err_not_nil_precedes_everything",
			result:           mcp.NewToolResultText("ignored by precedence"),
			err:              goErr,
			expectErrorLog:   true,
			expectSuccessLog: false,
		},
		{
			name:             "result_is_error_without_go_error",
			result:           mcp.NewToolResultError("domain failure"),
			err:              nil,
			expectErrorLog:   true,
			expectSuccessLog: false,
		},
		{
			name:             "regular_success",
			result:           mcp.NewToolResultText("ok"),
			err:              nil,
			expectErrorLog:   false,
			expectSuccessLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logBuf := setupToolLoggingCapture(t)

			inner := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return tt.result, tt.err
			}

			wrapped := withToolLogging("matrix_tool", inner)
			result, err := wrapped(context.Background(), mcp.CallToolRequest{})
			if !errors.Is(err, tt.err) {
				t.Fatalf("expected err=%v, got %v", tt.err, err)
			}
			if result != tt.result {
				t.Fatalf("expected result pointer to be preserved")
			}

			logs := logBuf.String()
			hasError := strings.Contains(logs, "tool=matrix_tool outcome=error")
			hasEnd := strings.Contains(logs, "tool=matrix_tool outcome=success")

			if hasError != tt.expectErrorLog {
				t.Fatalf("expected hasError=%v, got %v. logs=%s", tt.expectErrorLog, hasError, logs)
			}
			if hasEnd != tt.expectSuccessLog {
				t.Fatalf("expected hasEnd=%v, got %v. logs=%s", tt.expectSuccessLog, hasEnd, logs)
			}
		})
	}
}

func TestWithToolLogging_WhenGoErrorAndResultErrorBothPresent_UsesErrorPathDeterministically(t *testing.T) {
	logBuf := setupToolLoggingCapture(t)
	goErr := errors.New("go error wins")
	resultErr := mcp.NewToolResultError("result also marked as error")

	inner := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return resultErr, goErr
	}

	wrapped := withToolLogging("double_error_tool", inner)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if !errors.Is(err, goErr) {
		t.Fatalf("expected go error to be preserved, got %v", err)
	}
	if result != resultErr {
		t.Fatalf("expected result pointer to be preserved")
	}

	logs := logBuf.String()
	if !strings.Contains(logs, "tool=double_error_tool outcome=error") {
		t.Fatalf("expected deterministic error path log, got logs: %s", logs)
	}
	if strings.Contains(logs, "tool=double_error_tool outcome=success") {
		t.Fatalf("did not expect success log when both error channels are set, got logs: %s", logs)
	}
}

func TestWithToolLogging_WhenResultAndErrorAreNil_DocumentsCurrentBehavior(t *testing.T) {
	logBuf := setupToolLoggingCapture(t)

	inner := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, nil
	}

	wrapped := withToolLogging("nil_result_tool", inner)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %#v", result)
	}

	logs := logBuf.String()
	if !strings.Contains(logs, "tool=nil_result_tool outcome=success") {
		t.Fatalf("expected current behavior to log outcome=success for nil result+nil err, got logs: %s", logs)
	}
}
