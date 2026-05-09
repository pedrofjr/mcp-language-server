package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOnboardingReadiness(t *testing.T) {
	t.Run("quando_diretorio_nao_existe_retorna_nao_pronto_com_guidance", func(t *testing.T) {
		projectPath := filepath.Join(t.TempDir(), "nao-existe")

		ready, guidance := EvaluateOnboardingReadiness(projectPath)
		if ready {
			t.Fatalf("expected readiness=false for nonexistent directory %q", projectPath)
		}
		if strings.TrimSpace(guidance) == "" {
			t.Fatalf("expected actionable guidance for nonexistent directory")
		}
	})

	t.Run("quando_diretorio_sem_fontes_delphi_retorna_nao_pronto", func(t *testing.T) {
		projectPath := t.TempDir()
		err := os.WriteFile(filepath.Join(projectPath, "README.txt"), []byte("sem fontes delphi"), 0o644)
		if err != nil {
			t.Fatalf("failed to create fixture file: %v", err)
		}

		ready, guidance := EvaluateOnboardingReadiness(projectPath)
		if ready {
			t.Fatalf("expected readiness=false when no Delphi source files are present")
		}
		lowerGuidance := strings.ToLower(guidance)
		if !strings.Contains(lowerGuidance, ".pas") {
			t.Fatalf("expected readiness guidance to mention Delphi source files, got %q", guidance)
		}
	})

	t.Run("quando_diretorio_tem_pas_retorna_pronto", func(t *testing.T) {
		projectPath := t.TempDir()
		err := os.WriteFile(filepath.Join(projectPath, "Unit1.pas"), []byte("unit Unit1; interface implementation end."), 0o644)
		if err != nil {
			t.Fatalf("failed to create Delphi fixture: %v", err)
		}

		ready, guidance := EvaluateOnboardingReadiness(projectPath)
		if !ready {
			t.Fatalf("expected readiness=true when .pas file exists; guidance=%q", guidance)
		}
	})
}
