package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestStructuredToolLogging_EmitsRequiredFieldsOnSuccess(t *testing.T) {
	logBuf := setupToolLoggingCapture(t)

	inner := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("ok"), nil
	}

	ctx := contextWithToolObs(context.Background(), `C:\workspace\proj`)
	wrapped := withToolLogging("run_query", inner)
	_, err := wrapped(ctx, mcp.CallToolRequest{
		Params: struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Arguments: map[string]interface{}{
				"projectPath": `C:\workspace\proj\src`,
			},
			Meta: &struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			}{
				ProgressToken: "corr-42",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	logs := logBuf.String()
	for _, field := range []string{
		"tool=run_query",
		"outcome=success",
		"duration_ms=",
		"request_id=pt:corr-42",
		"project_path=",
		"timestamp=",
	} {
		if !strings.Contains(logs, field) {
			t.Fatalf("expected log to contain %q, got: %s", field, logs)
		}
	}
}

func TestStructuredToolLogging_EmitsErrorOutcome(t *testing.T) {
	logBuf := setupToolLoggingCapture(t)

	inner := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultError(OpValidation + " | bad args | action: fix"), nil
	}

	wrapped := withToolLogging("definition", inner)
	_, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("expected nil go error, got %v", err)
	}

	logs := logBuf.String()
	if !strings.Contains(logs, "tool=definition outcome=error") {
		t.Fatalf("expected error outcome log, got: %s", logs)
	}
}

func TestOperationalRecovery_AppendedToToolResultError(t *testing.T) {
	result, err := OpValidationError("missing filePath")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	text := extractToolResultText(t, result)
	if !strings.Contains(text, "recovery:") {
		t.Fatalf("expected recovery guidance in validation error, got %q", text)
	}
}

func TestOperationalRecovery_EnrichesLegacyOperationalErrors(t *testing.T) {
	raw := mcp.NewToolResultError(opErrMsg(OpLSPUnavailable, "language server not available | action: initialize language server client"))
	enriched := enrichToolResultWithRecovery(raw)
	text := extractToolResultText(t, enriched)
	if !strings.Contains(text, "recovery:") {
		t.Fatalf("expected recovery appended to LSP unavailable error, got %q", text)
	}
}

func TestCriticalMCPTools_AllRegisteredNamesCovered(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	listResp := handleTestMCPRequest(t, svc, mcp.MethodToolsList, map[string]any{}, 1)
	var listed struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	decodeTestMCPResult(t, listResp.Result, &listed)

	registered := make(map[string]struct{}, len(listed.Tools))
	for _, tool := range listed.Tools {
		registered[tool.Name] = struct{}{}
	}

	for name := range criticalMCPTools {
		if _, ok := registered[name]; !ok {
			t.Fatalf("critical tool %q is not registered in tools/list", name)
		}
	}
}

func TestWithToolLogging_PropagatesGoErrorStillLogsStructured(t *testing.T) {
	logBuf := setupToolLoggingCapture(t)
	expectedErr := errors.New("boom")

	inner := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, expectedErr
	}
	wrapped := withToolLogging("failing_tool", inner)
	_, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	if !strings.Contains(logBuf.String(), "outcome=error") {
		t.Fatalf("expected structured error outcome log, got: %s", logBuf.String())
	}
}
