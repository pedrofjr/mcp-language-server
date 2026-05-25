package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func isolateOnboardingDir(t *testing.T) {
	t.Helper()
	t.Setenv("ORACLE_MCP_ONBOARDING_DIR", t.TempDir())
}

func TestPerformOnboarding_DefaultDoesNotCreateProjectMarkerFile(t *testing.T) {
	isolateOnboardingDir(t)
	projectPath := t.TempDir()
	createDelphiFixture(t, projectPath)

	_, err := PerformOnboardingWithContext(projectPath, "")
	require.NoError(t, err)

	_, err = os.Stat(projectOnboardingFilePath(projectPath))
	assert.True(t, os.IsNotExist(err), "padrao nao deve gravar .oracle-onboarding.json no workspace")

	externalPath, err := externalOnboardingFilePath(projectPath)
	require.NoError(t, err)
	_, err = os.Stat(externalPath)
	require.NoError(t, err, "estado deve persistir fora do projeto por padrao")
}

func TestPerformOnboarding_PersistInProjectOptInCreatesProjectMarkerFile(t *testing.T) {
	isolateOnboardingDir(t)
	projectPath := t.TempDir()
	createDelphiFixture(t, projectPath)

	_, err := PerformOnboardingWithContextAndOptions(
		context.Background(),
		projectPath,
		"",
		OnboardingOptions{PersistInProject: true},
	)
	require.NoError(t, err)

	_, err = os.Stat(projectOnboardingFilePath(projectPath))
	require.NoError(t, err, "opt-in deve gravar marcador no workspace")
}

func TestScanProjectStructureCtx_SkipsVcsAndDependencyDirs(t *testing.T) {
	projectPath := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectPath, "Visible.pas"), []byte("unit Visible;"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(projectPath, ".git", "objects"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectPath, ".git", "objects", "Hidden.pas"), []byte("unit Hidden;"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(projectPath, "node_modules", "pkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectPath, "node_modules", "pkg", "Hidden.pas"), []byte("unit Hidden;"), 0o644))

	structure, err := ScanProjectStructureCtx(context.Background(), projectPath)
	require.NoError(t, err)
	assert.Equal(t, []string{"Visible.pas"}, structure.Units)
}

func TestScanProjectStructureCtx_RespectsContextCancellation(t *testing.T) {
	projectPath := t.TempDir()
	createDelphiFixture(t, projectPath)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ScanProjectStructureCtx(ctx, projectPath)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
