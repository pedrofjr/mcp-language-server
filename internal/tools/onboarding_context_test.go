package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createDelphiFixture(t *testing.T, projectPath string) {
	t.Helper()

	pasPath := filepath.Join(projectPath, "Unit1.pas")
	err := os.WriteFile(pasPath, []byte("unit Unit1;\ninterface\nimplementation\nend."), 0o644)
	require.NoError(t, err)
}

func TestOnboardingContext_quando_onboarding_em_ctxA_so_ctxA_fica_performed(t *testing.T) {
	isolateOnboardingDir(t)
	projectPath := t.TempDir()
	createDelphiFixture(t, projectPath)

	_, err := PerformOnboardingWithContext(projectPath, "ctx-A")
	require.NoError(t, err)

	performedCtxA, _ := CheckOnboardingPerformedWithContext(projectPath, "ctx-A")
	performedCtxB, _ := CheckOnboardingPerformedWithContext(projectPath, "ctx-B")

	assert.True(t, performedCtxA, "ctx-A deve estar marcado como performed apos onboarding no mesmo contexto")
	assert.False(t, performedCtxB, "ctx-B nao deve ser marcado quando onboarding ocorre apenas em ctx-A")
}

func TestOnboardingContext_quando_sem_contexto_explicito_mantem_compat_default_legacy(t *testing.T) {
	isolateOnboardingDir(t)
	projectPath := t.TempDir()
	createDelphiFixture(t, projectPath)

	_, err := PerformOnboarding(projectPath)
	require.NoError(t, err)

	performedLegacy, _ := CheckOnboardingPerformed(projectPath)
	performedDefaultCtx, _ := CheckOnboardingPerformedWithContext(projectPath, "")

	assert.True(t, performedLegacy, "fluxo legacy deve continuar retornando true apos PerformOnboarding sem contexto")
	assert.True(t, performedDefaultCtx, "contexto default deve refletir onboarding legacy quando contexto explicito nao for informado")
}

func TestOnboardingContext_quando_arquivo_legado_so_performed_at_reconhece_contexto_default(t *testing.T) {
	projectPath := t.TempDir()

	legacyRecord := map[string]any{
		"performed_at": time.Date(2026, time.May, 9, 10, 0, 0, 0, time.UTC),
	}
	data, err := json.MarshalIndent(legacyRecord, "", "  ")
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(projectPath, ".oracle-onboarding.json"), data, 0o644)
	require.NoError(t, err)

	performedLegacy, _ := CheckOnboardingPerformed(projectPath)
	performedDefaultCtx, _ := CheckOnboardingPerformedWithContext(projectPath, "")

	assert.True(t, performedLegacy, "arquivo legado com apenas performed_at deve continuar valido no fluxo legacy")
	assert.True(t, performedDefaultCtx, "arquivo legado deve ser reconhecido como contexto default")
}

func TestOnboardingContext_quando_contexts_default_existe_tem_precedencia_sobre_legacy(t *testing.T) {
	projectPath := t.TempDir()

	legacyAt := time.Date(2026, time.May, 9, 10, 0, 0, 0, time.UTC)
	contextDefaultAt := legacyAt.Add(2 * time.Hour)
	record := map[string]any{
		"performed_at": legacyAt,
		"contexts": map[string]any{
			"default": map[string]any{
				"performed_at": contextDefaultAt,
				"unit_count":   1,
				"entry_point":  "App.dpr",
			},
		},
	}

	data, err := json.MarshalIndent(record, "", "  ")
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(projectPath, ".oracle-onboarding.json"), data, 0o644)
	require.NoError(t, err)

	performedDefaultCtx, atDefaultCtx := CheckOnboardingPerformedWithContext(projectPath, "")
	performedLegacy, atLegacy := CheckOnboardingPerformed(projectPath)

	assert.True(t, performedDefaultCtx, "contexto default deve estar marcado quando contexts.default existe")
	assert.True(t, performedLegacy, "wrapper legado deve continuar funcional")
	assert.Equal(t, contextDefaultAt, atDefaultCtx, "contexts.default deve ter precedencia sobre performed_at legado")
	assert.Equal(t, contextDefaultAt, atLegacy, "fluxo legado deve refletir a mesma precedencia do default")
}

func TestOnboardingContext_quando_contexto_nao_default_nao_faz_fallback_para_legacy(t *testing.T) {
	projectPath := t.TempDir()

	legacyRecord := map[string]any{
		"performed_at": time.Date(2026, time.May, 9, 10, 0, 0, 0, time.UTC),
	}
	data, err := json.MarshalIndent(legacyRecord, "", "  ")
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(projectPath, ".oracle-onboarding.json"), data, 0o644)
	require.NoError(t, err)

	performedCtxA, _ := CheckOnboardingPerformedWithContext(projectPath, "ctx-A")
	assert.False(t, performedCtxA, "contexto nao-default nao deve usar fallback do campo legado performed_at")
}

func TestOnboardingContext_quando_legado_existe_e_onboarding_em_ctxA_preserva_legacy_default(t *testing.T) {
	isolateOnboardingDir(t)
	projectPath := t.TempDir()
	createDelphiFixture(t, projectPath)

	legacyAt := time.Date(2026, time.May, 9, 10, 0, 0, 0, time.UTC)
	legacyRecord := map[string]any{
		"performed_at": legacyAt,
	}
	data, err := json.MarshalIndent(legacyRecord, "", "  ")
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(projectPath, ".oracle-onboarding.json"), data, 0o644)
	require.NoError(t, err)

	_, err = PerformOnboardingWithContext(projectPath, "ctx-A")
	require.NoError(t, err)

	performedLegacy, atLegacy := CheckOnboardingPerformed(projectPath)
	performedDefaultCtx, atDefaultCtx := CheckOnboardingPerformedWithContext(projectPath, "")
	performedCtxA, _ := CheckOnboardingPerformedWithContext(projectPath, "ctx-A")

	assert.True(t, performedLegacy, "fluxo legado deve continuar valido apos onboarding em contexto nao-default")
	assert.True(t, performedDefaultCtx, "contexto default deve manter compatibilidade com legado existente")
	assert.Equal(t, legacyAt, atLegacy, "onboarding em contexto nao-default nao deve apagar/alterar performed_at legado")
	assert.Equal(t, legacyAt, atDefaultCtx, "default deve continuar lendo legado quando contexts.default estiver ausente")
	assert.True(t, performedCtxA, "ctx-A deve estar marcado apos onboarding no proprio contexto")
}

func TestOnboardingContext_quando_default_explicito_zerado_nao_faz_fallback_para_legacy(t *testing.T) {
	projectPath := t.TempDir()

	legacyAt := time.Date(2026, time.May, 9, 10, 0, 0, 0, time.UTC)
	record := map[string]any{
		"performed_at": legacyAt,
		"contexts": map[string]any{
			"default": map[string]any{
				"performed_at": "0001-01-01T00:00:00Z",
			},
		},
	}

	data, err := json.MarshalIndent(record, "", "  ")
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(projectPath, ".oracle-onboarding.json"), data, 0o644)
	require.NoError(t, err)

	performedDefaultCtx, _ := CheckOnboardingPerformedWithContext(projectPath, "")
	performedLegacy, _ := CheckOnboardingPerformed(projectPath)

	assert.False(t, performedDefaultCtx, "quando contexts.default existe mas e invalido/zerado, nao deve usar fallback legado")
	assert.False(t, performedLegacy, "wrapper legado deve refletir a mesma precedencia estrita do contexto default")
}
