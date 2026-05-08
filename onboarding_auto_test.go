package main

import (
	"os"
	"testing"

	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// TestShouldStartAutoOnboarding_ReturnsTrueWhenNeverPerformed verifica que
// shouldStartAutoOnboarding retorna true quando o projeto nunca passou por onboarding.
func TestShouldStartAutoOnboarding_ReturnsTrueWhenNeverPerformed(t *testing.T) {
	tmpDir := t.TempDir()
	result := shouldStartAutoOnboarding(tmpDir)
	if !result {
		t.Fatal("expected shouldStartAutoOnboarding to return true for new project")
	}
}

// TestShouldStartAutoOnboarding_ReturnsFalseWhenAlreadyPerformed verifica que
// shouldStartAutoOnboarding retorna false quando o projeto já passou por onboarding.
func TestShouldStartAutoOnboarding_ReturnsFalseWhenAlreadyPerformed(t *testing.T) {
	tmpDir := t.TempDir()

	// Simula que onboarding já foi executado criando um arquivo dummy .pas
	// para que ScanProjectStructure funcione corretamente.
	err := os.WriteFile(tmpDir+"/test.pas", []byte("unit Test;\nend."), 0o644)
	if err != nil {
		t.Fatalf("failed to create test.pas: %v", err)
	}

	// Executa onboarding para criar o arquivo de marca .oracle-onboarding.json
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
}
