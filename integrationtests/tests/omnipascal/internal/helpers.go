package internal

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isaacphi/mcp-language-server/internal/omnipascal"
)

const defaultWorkspaceDir = `C:\Users\pedro.ailton\Downloads\SelecaoTimCFOPLote`

type TestSuite struct {
	Client       *omnipascal.Client
	Context      context.Context
	Cancel       context.CancelFunc
	WorkspaceDir string
	TargetFile   string
	ProjectFile  string
	RefLine      int
	RefColumn    int
}

func GetTestSuite(t *testing.T) *TestSuite {
	t.Helper()

	serverPath := strings.TrimSpace(os.Getenv("OMNIPASCAL_SERVER"))
	if serverPath == "" {
		t.Skip("set OMNIPASCAL_SERVER to run OmniPascal integration smoke tests")
	}

	workspaceDir := strings.TrimSpace(os.Getenv("OMNIPASCAL_WORKSPACE"))
	if workspaceDir == "" {
		workspaceDir = defaultWorkspaceDir
	}

	absWorkspace, err := filepath.Abs(workspaceDir)
	if err != nil {
		t.Fatalf("failed to resolve workspace path: %v", err)
	}
	workspaceDir = absWorkspace

	if _, err := os.Stat(workspaceDir); err != nil {
		t.Fatalf("workspace does not exist: %s (%v)", workspaceDir, err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	startupArgs := []string{fmt.Sprintf("--workspacePaths=%s", workspaceDir)}
	client, err := omnipascal.NewClient(serverPath, startupArgs...)
	if err != nil {
		cancel()
		t.Fatalf("failed to start OmniPascal server: %v", err)
	}

	config := map[string]any{"workspacePaths": workspaceDir}
	if v := strings.TrimSpace(os.Getenv("OMNIPASCAL_DELPHI_PATH")); v != "" {
		config["delphiInstallationPath"] = v
	}
	if v := strings.TrimSpace(os.Getenv("OMNIPASCAL_SEARCH_PATH")); v != "" {
		config["searchPath"] = v
	}
	if v := strings.TrimSpace(os.Getenv("OMNIPASCAL_DEFAULT_ENV")); v != "" {
		config["defaultDevelopmentEnvironment"] = v
	}

	if err := client.SynchronizeConfig(ctx, config); err != nil {
		_ = client.Close()
		cancel()
		t.Fatalf("failed to sync OmniPascal config: %v", err)
	}

	targetFile := strings.TrimSpace(os.Getenv("OMNIPASCAL_TARGET_FILE"))
	if targetFile == "" {
		targetFile, err = findFirstByExtensions(workspaceDir, ".pas", ".pp")
		if err != nil {
			_ = client.Close()
			cancel()
			t.Fatalf("failed to auto-discover target file: %v", err)
		}
	} else if !filepath.IsAbs(targetFile) {
		targetFile = filepath.Join(workspaceDir, targetFile)
	}

	if _, err := os.Stat(targetFile); err != nil {
		_ = client.Close()
		cancel()
		t.Fatalf("target file does not exist: %s (%v)", targetFile, err)
	}

	projectFile := strings.TrimSpace(os.Getenv("OMNIPASCAL_PROJECT_FILE"))
	if projectFile != "" && !filepath.IsAbs(projectFile) {
		projectFile = filepath.Join(workspaceDir, projectFile)
	}
	if projectFile == "" {
		if detectedProject, detectErr := findFirstByExtensions(workspaceDir, ".dpr", ".dpk", ".lpi"); detectErr == nil {
			projectFile = detectedProject
		}
	}

	refLine, refColumn, err := findReferencePosition(targetFile)
	if err != nil {
		_ = client.Close()
		cancel()
		t.Fatalf("failed to compute reference position: %v", err)
	}

	suite := &TestSuite{
		Client:       client,
		Context:      ctx,
		Cancel:       cancel,
		WorkspaceDir: workspaceDir,
		TargetFile:   targetFile,
		ProjectFile:  projectFile,
		RefLine:      refLine,
		RefColumn:    refColumn,
	}

	t.Logf("OmniPascal workspace: %s", suite.WorkspaceDir)
	t.Logf("OmniPascal target file: %s", suite.TargetFile)
	if suite.ProjectFile != "" {
		t.Logf("OmniPascal project file: %s", suite.ProjectFile)
	}

	t.Cleanup(func() {
		suite.Cancel()
		if suite.Client != nil {
			_ = suite.Client.Close()
		}
	})

	return suite
}

func findFirstByExtensions(root string, exts ...string) (string, error) {
	wanted := make(map[string]struct{}, len(exts))
	for _, ext := range exts {
		wanted[strings.ToLower(ext)] = struct{}{}
	}

	var firstMatch string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if _, ok := wanted[ext]; ok {
			firstMatch = path
			return errors.New("_found")
		}
		return nil
	})

	if err != nil && err.Error() != "_found" {
		return "", err
	}
	if firstMatch == "" {
		return "", fmt.Errorf("no files found with extensions: %v", exts)
	}
	return firstMatch, nil
}

func findReferencePosition(filePath string) (int, int, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, 0, err
	}
	defer func() {
		_ = f.Close()
	}()

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		for i, r := range line {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_' {
				return lineNo, i + 1, nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, err
	}
	return 1, 1, nil
}
