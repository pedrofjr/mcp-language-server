package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestOperationalErrorHelpers_IncludeStableTokens(t *testing.T) {
	validation, err := OpValidationError("uri must be a non-empty string")
	if err != nil {
		t.Fatalf("OpValidationError returned unexpected err: %v", err)
	}
	if validation == nil || !validation.IsError {
		t.Fatalf("expected validation tool error result")
	}
	validationText := extractToolResultText(t, validation)
	if !strings.Contains(validationText, OpValidation) {
		t.Fatalf("expected validation error to include %q, got %q", OpValidation, validationText)
	}
	if !strings.Contains(validationText, "action:") {
		t.Fatalf("expected validation error to include actionable guidance, got %q", validationText)
	}

	failed, err := OpToolFailedError("workspace_symbols", "boom", "retry after fixing LSP path")
	if err != nil {
		t.Fatalf("OpToolFailedError returned unexpected err: %v", err)
	}
	if failed == nil || !failed.IsError {
		t.Fatalf("expected tool failed error result")
	}
	failedText := extractToolResultText(t, failed)
	if !strings.Contains(failedText, OpToolFailed) {
		t.Fatalf("expected tool failed error to include %q, got %q", OpToolFailed, failedText)
	}
	if !strings.Contains(failedText, "action:") {
		t.Fatalf("expected tool failed error to include actionable guidance, got %q", failedText)
	}
}

func TestOperationalErrorFromParseArg_IncludesValidationToken(t *testing.T) {
	result, err := OpErrorFromParseArg(fmt.Errorf("limit must be a positive integer"))
	if err != nil {
		t.Fatalf("OpErrorFromParseArg returned unexpected err: %v", err)
	}
	text := extractToolResultText(t, result)
	if !strings.Contains(text, OpValidation) {
		t.Fatalf("expected parse arg error to include %q, got %q", OpValidation, text)
	}
	if !strings.Contains(text, "limit must be a positive integer") {
		t.Fatalf("expected original detail preserved, got %q", text)
	}
}

func TestOperationalErrorContract_CriticalToolsReturnOPValidationOnBadArgs(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	cases := []struct {
		tool string
		args map[string]any
	}{
		{tool: "semantic_search", args: map[string]any{"query": 123}},
		{tool: "run_query", args: map[string]any{}},
		{tool: "dependency_tree", args: map[string]any{"uri": "file:///u.pas", "direction": "sideways"}},
		{tool: "code_actions", args: map[string]any{"filePath": 1, "line": 1, "column": 1}},
	}

	for i, tc := range cases {
		callResp := handleTestMCPRequest(
			t,
			svc,
			mcp.MethodToolsCall,
			map[string]any{
				"name":      tc.tool,
				"arguments": tc.args,
			},
			700+i,
		)

		resultBytes, err := json.Marshal(callResp.Result)
		if err != nil {
			t.Fatalf("%s: marshal result: %v", tc.tool, err)
		}
		if !strings.Contains(string(resultBytes), OpValidation) {
			t.Fatalf("%s: expected %q in error payload, got %s", tc.tool, OpValidation, string(resultBytes))
		}
		if !strings.Contains(string(resultBytes), "action:") {
			t.Fatalf("%s: expected actionable guidance in error payload, got %s", tc.tool, string(resultBytes))
		}
		if !strings.Contains(string(resultBytes), "recovery:") {
			t.Fatalf("%s: expected recovery guidance in error payload, got %s", tc.tool, string(resultBytes))
		}
	}
}

func TestOperationalErrorFromMemory_ClassifiesValidation(t *testing.T) {
	result, err := OpErrorFromMemory("memory_write", fmt.Errorf("campo title e obrigatorio e nao pode ser vazio"))
	if err != nil {
		t.Fatalf("OpErrorFromMemory returned unexpected err: %v", err)
	}
	text := extractToolResultText(t, result)
	if !strings.Contains(text, OpValidation) {
		t.Fatalf("expected memory validation error to include %q, got %q", OpValidation, text)
	}
}

func extractToolResultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatalf("expected tool result content")
	}
	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	return textContent.Text
}
