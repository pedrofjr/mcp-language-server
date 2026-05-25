package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isaacphi/mcp-language-server/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
)

func createMinimalOnboardingFixture(t *testing.T, projectPath string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(projectPath, "Unit1.pas"), []byte("unit Unit1; interface implementation end."), 0o644); err != nil {
		t.Fatalf("failed to create onboarding fixture: %v", err)
	}
}

func assertOnboardingImpactLog(t *testing.T, logs string, source string, projectPath string, contextKey string, postCheckPerformed string) {
	t.Helper()

	expectedFragments := []string{
		"onboarding_impact",
		"source=" + source,
		"project_path=" + projectPath,
		"context=" + contextKey,
		"post_check_performed=" + postCheckPerformed,
		"duration_ms=",
	}

	for _, fragment := range expectedFragments {
		if !strings.Contains(logs, fragment) {
			t.Fatalf("expected onboarding impact log to contain %q, got logs: %s", fragment, logs)
		}
	}
}

func TestOnboardingImpact_AutoFlow_EmitsStructuredLogAfterAttempt(t *testing.T) {
	t.Setenv("ORACLE_MCP_ONBOARDING_DIR", t.TempDir())
	projectPath := t.TempDir()
	createMinimalOnboardingFixture(t, projectPath)

	logBuf := setupToolLoggingCapture(t)

	if !shouldStartAutoOnboarding(projectPath) {
		t.Fatal("expected a new project to require auto onboarding before the attempt")
	}

	result, err := tools.PerformOnboardingWithContextAndOptions(
		context.Background(),
		projectPath,
		"",
		tools.OnboardingOptions{AutoRun: true},
	)
	if err != nil {
		t.Fatalf("PerformOnboardingWithContextAndOptions returned error: %v", err)
	}
	if result == "" {
		t.Fatal("expected onboarding attempt to return a non-empty payload")
	}

	performed, _ := tools.CheckOnboardingPerformedWithContext(projectPath, "")
	if !performed {
		t.Fatal("expected onboarding to be marked as performed after the attempt")
	}

	assertOnboardingImpactLog(t, logBuf.String(), "auto", projectPath, "default", "true")
}

func TestRegisterTools_OnboardingImpact_ManualTool_EmitsStructuredLogAndPreservesToolContract(t *testing.T) {
	t.Setenv("ORACLE_MCP_ONBOARDING_DIR", t.TempDir())
	projectPath := t.TempDir()
	createMinimalOnboardingFixture(t, projectPath)

	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)
	logBuf := setupToolLoggingCapture(t)

	callResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name": "onboarding",
			"arguments": map[string]any{
				"projectPath": projectPath,
				"context":     "manual",
			},
		},
		101,
	)

	resultBytes, err := json.Marshal(callResp.Result)
	if err != nil {
		t.Fatalf("failed to marshal onboarding result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("failed to decode onboarding result map: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	if isError {
		t.Fatalf("expected onboarding tool to preserve success contract, got %s", string(resultBytes))
	}

	contentRaw, ok := callResult["content"].([]any)
	if !ok || len(contentRaw) == 0 {
		t.Fatalf("expected onboarding tool to return content, got %s", string(resultBytes))
	}

	firstContent, ok := contentRaw[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first onboarding content item to be an object, got %s", string(resultBytes))
	}

	message, _ := firstContent["text"].(string)
	if !strings.Contains(message, "Onboarding concluido") {
		t.Fatalf("expected onboarding success payload to preserve text contract, got %q", message)
	}

	performed, _ := tools.CheckOnboardingPerformedWithContext(projectPath, "manual")
	if !performed {
		t.Fatal("expected manual onboarding to be recorded for the requested context")
	}

	assertOnboardingImpactLog(t, logBuf.String(), "manual", projectPath, "manual", "true")
}