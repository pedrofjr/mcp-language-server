package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestShouldStartAutoOnboarding_ReturnsTrueWhenNeverPerformed verifica que
// shouldStartAutoOnboarding retorna true quando o projeto nunca passou por onboarding.
func TestShouldStartAutoOnboarding_ReturnsTrueWhenNeverPerformed(t *testing.T) {
	t.Setenv("ORACLE_MCP_ONBOARDING_DIR", t.TempDir())
	tmpDir := t.TempDir()
	result := shouldStartAutoOnboarding(tmpDir)
	if !result {
		t.Fatal("expected shouldStartAutoOnboarding to return true for new project")
	}
}

// TestShouldStartAutoOnboarding_ReturnsFalseWhenAlreadyPerformed verifica que
// shouldStartAutoOnboarding retorna false quando o projeto já passou por onboarding.
func TestShouldStartAutoOnboarding_ReturnsFalseWhenAlreadyPerformed(t *testing.T) {
	t.Setenv("ORACLE_MCP_ONBOARDING_DIR", t.TempDir())
	tmpDir := t.TempDir()

	// Simula que onboarding já foi executado criando um arquivo dummy .pas
	// para que ScanProjectStructure funcione corretamente.
	err := os.WriteFile(tmpDir+"/test.pas", []byte("unit Test;\nend."), 0o644)
	if err != nil {
		t.Fatalf("failed to create test.pas: %v", err)
	}

	// Executa onboarding (persistencia padrao fora do workspace)
	_, err = tools.PerformOnboarding(tmpDir)
	if err != nil {
		t.Fatalf("PerformOnboarding failed: %v", err)
	}

	// Verifica que CheckOnboardingPerformed agora retorna true
	performed, _ := tools.CheckOnboardingPerformed(tmpDir)
	if !performed {
		t.Fatal("CheckOnboardingPerformed should return true after PerformOnboarding")
	}

	// Agora shouldStartAutoOnboarding deve retornar false
	result := shouldStartAutoOnboarding(tmpDir)
	if result {
		t.Fatal("expected shouldStartAutoOnboarding to return false when already performed")
	}

	if tools.ProjectOnboardingFileExists(tmpDir) {
		t.Fatal("auto-onboarding padrao nao deve criar .oracle-onboarding.json no workspace")
	}
}

func TestAutoOnboarding_DefaultPersistenceStaysOutsideWorkspace(t *testing.T) {
	t.Setenv("ORACLE_MCP_ONBOARDING_DIR", t.TempDir())
	projectPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectPath, "Unit1.pas"), []byte("unit Unit1; interface implementation end."), 0o644); err != nil {
		t.Fatalf("failed to create fixture: %v", err)
	}

	_, err := tools.PerformOnboardingWithContextAndOptions(
		context.Background(),
		projectPath,
		"",
		tools.OnboardingOptions{AutoRun: true},
	)
	if err != nil {
		t.Fatalf("auto onboarding attempt failed: %v", err)
	}

	if tools.ProjectOnboardingFileExists(projectPath) {
		t.Fatal("expected no project-local onboarding marker without opt-in")
	}
}
