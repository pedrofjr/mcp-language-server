package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultOnboardingContext = "default"

const projectOnboardingFileName = ".oracle-onboarding.json"

// OnboardingOptions controla persistencia e comportamento do scan.
type OnboardingOptions struct {
	// PersistInProject grava .oracle-onboarding.json no workspace (opt-in).
	PersistInProject bool
	// AutoRun marca telemetria de onboarding automatico no startup do servidor MCP.
	AutoRun bool
}

type onboardingContextRecord struct {
	PerformedAt time.Time `json:"performed_at"`
	UnitCount   int       `json:"unit_count,omitempty"`
	EntryPoint  string    `json:"entry_point,omitempty"`
}

type onboardingRecord struct {
	PerformedAt time.Time                          `json:"performed_at,omitempty"`
	UnitCount   int                                `json:"unit_count,omitempty"`
	EntryPoint  string                             `json:"entry_point,omitempty"`
	Contexts    map[string]onboardingContextRecord `json:"contexts,omitempty"`
}

// ProjectStructure descreve a estrutura de um projeto Delphi.
type ProjectStructure struct {
	ProjectPath string    `json:"project_path"`
	EntryPoint  string    `json:"entry_point"`
	Units       []string  `json:"units"`
	Forms       []string  `json:"forms"`
	UnitCount   int       `json:"unit_count"`
	ScannedAt   time.Time `json:"scanned_at"`
}

func shouldSkipOnboardingDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", ".hg", ".svn", "node_modules", "vendor", "target", "build", "dist", ".idea", ".vscode", ".oracle-lsp":
		return true
	default:
		return false
	}
}

// ScanProjectStructure escaneia um diretorio Delphi e retorna a estrutura do projeto.
func ScanProjectStructure(projectPath string) (*ProjectStructure, error) {
	return ScanProjectStructureCtx(context.Background(), projectPath)
}

// ScanProjectStructureCtx escaneia com WalkDir, exclusoes padrao e cancelamento via contexto.
func ScanProjectStructureCtx(ctx context.Context, projectPath string) (*ProjectStructure, error) {
	result := &ProjectStructure{
		ProjectPath: projectPath,
		ScannedAt:   time.Now(),
	}

	err := filepath.WalkDir(projectPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return nil
		}

		if entry.IsDir() {
			if path != projectPath && shouldSkipOnboardingDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		lower := strings.ToLower(path)
		switch {
		case strings.HasSuffix(lower, ".dpr") || strings.HasSuffix(lower, ".dpk"):
			if result.EntryPoint == "" {
				result.EntryPoint = filepath.Base(path)
			}
		case strings.HasSuffix(lower, ".pas"):
			result.Units = append(result.Units, filepath.Base(path))
		case strings.HasSuffix(lower, ".dfm"):
			result.Forms = append(result.Forms, filepath.Base(path))
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	result.UnitCount = len(result.Units)
	return result, nil
}

func onboardingDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("ORACLE_MCP_ONBOARDING_DIR")); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", err
		}
		return dir, nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(configDir, "oracle-mcp", "onboarding")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func projectOnboardingKey(projectPath string) (string, error) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return "", err
	}

	normalized := filepath.Clean(absPath)
	if os.PathSeparator == '\\' {
		normalized = strings.ToLower(normalized)
	}

	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:]), nil
}

func externalOnboardingFilePath(projectPath string) (string, error) {
	dir, err := onboardingDir()
	if err != nil {
		return "", err
	}

	key, err := projectOnboardingKey(projectPath)
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, key+".json"), nil
}

func projectOnboardingFilePath(projectPath string) string {
	return filepath.Join(projectPath, projectOnboardingFileName)
}

func resolvePersistInProject(opts OnboardingOptions) bool {
	if opts.PersistInProject {
		return true
	}
	return strings.TrimSpace(os.Getenv("DELPHI_ORACLE_MCP_ONBOARDING_IN_PROJECT")) == "1"
}

func onboardingWritePath(projectPath string, opts OnboardingOptions) (string, error) {
	if resolvePersistInProject(opts) {
		return projectOnboardingFilePath(projectPath), nil
	}
	return externalOnboardingFilePath(projectPath)
}

func loadOnboardingRecordFromPath(path string) (onboardingRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return onboardingRecord{}, err
	}

	var record onboardingRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return onboardingRecord{}, err
	}

	if record.Contexts == nil {
		record.Contexts = make(map[string]onboardingContextRecord)
	}

	return record, nil
}

func loadOnboardingRecord(projectPath string, opts OnboardingOptions) (onboardingRecord, error) {
	writePath, err := onboardingWritePath(projectPath, opts)
	if err != nil {
		return onboardingRecord{}, err
	}

	if record, err := loadOnboardingRecordFromPath(writePath); err == nil {
		return record, nil
	} else if !os.IsNotExist(err) {
		return onboardingRecord{}, err
	}

	if resolvePersistInProject(opts) {
		return onboardingRecord{}, os.ErrNotExist
	}

	legacyPath := projectOnboardingFilePath(projectPath)
	record, err := loadOnboardingRecordFromPath(legacyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return onboardingRecord{}, os.ErrNotExist
		}
		return onboardingRecord{}, err
	}
	return record, nil
}

func saveOnboardingRecord(projectPath string, record onboardingRecord, opts OnboardingOptions) error {
	writePath, err := onboardingWritePath(projectPath, opts)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(writePath), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(writePath, data, 0o644)
}

func normalizeOnboardingContext(contextKey string) string {
	normalized := strings.TrimSpace(contextKey)
	if normalized == "" {
		return defaultOnboardingContext
	}
	return normalized
}

func syncLegacyDefault(record *onboardingRecord) {
	defaultRecord, hasDefault := record.Contexts[defaultOnboardingContext]
	if !hasDefault {
		return
	}

	record.PerformedAt = defaultRecord.PerformedAt
	record.UnitCount = defaultRecord.UnitCount
	record.EntryPoint = defaultRecord.EntryPoint
}

// CheckOnboardingPerformed verifica se o onboarding ja foi executado para o projeto.
func CheckOnboardingPerformed(projectPath string) (bool, time.Time) {
	return CheckOnboardingPerformedWithContext(projectPath, "")
}

// CheckOnboardingPerformedWithContext verifica se o onboarding ja foi executado para um contexto.
func CheckOnboardingPerformedWithContext(projectPath, contextKey string) (bool, time.Time) {
	return checkOnboardingPerformedWithOptions(projectPath, contextKey, OnboardingOptions{})
}

func checkOnboardingPerformedWithOptions(projectPath, contextKey string, opts OnboardingOptions) (bool, time.Time) {
	record, err := loadOnboardingRecord(projectPath, opts)
	if err != nil {
		return false, time.Time{}
	}

	contextKey = normalizeOnboardingContext(contextKey)

	if contextRecord, ok := record.Contexts[contextKey]; ok && !contextRecord.PerformedAt.IsZero() {
		return true, contextRecord.PerformedAt
	}

	if contextKey == defaultOnboardingContext {
		if _, hasDefault := record.Contexts[defaultOnboardingContext]; hasDefault {
			return false, time.Time{}
		}

		if !record.PerformedAt.IsZero() {
			return true, record.PerformedAt
		}
	}

	return false, time.Time{}
}

// EvaluateOnboardingReadiness avalia se o diretorio parece pronto para onboarding.
func EvaluateOnboardingReadiness(projectPath string) (bool, string) {
	return evaluateOnboardingReadinessCtx(context.Background(), projectPath)
}

func evaluateOnboardingReadinessCtx(ctx context.Context, projectPath string) (bool, string) {
	info, err := os.Stat(projectPath)
	if err != nil || !info.IsDir() {
		return false, "Proximo passo: execute onboarding e rode check_onboarding_performed novamente. Se nao houver arquivos Delphi (.pas/.dpr/.dpk), o onboarding pode nao encontrar unidades."
	}

	hasDelphiSource := false
	_ = filepath.WalkDir(projectPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil || entry == nil {
			return nil
		}
		if entry.IsDir() {
			if path != projectPath && shouldSkipOnboardingDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		lower := strings.ToLower(path)
		if strings.HasSuffix(lower, ".pas") || strings.HasSuffix(lower, ".dpr") || strings.HasSuffix(lower, ".dpk") {
			hasDelphiSource = true
		}
		return nil
	})

	if !hasDelphiSource {
		return false, "Proximo passo: execute onboarding e rode check_onboarding_performed novamente. Se nao houver arquivos Delphi (.pas/.dpr/.dpk), o onboarding pode nao encontrar unidades."
	}

	return true, ""
}

func logOnboardingImpact(source, projectPath, contextKey string, performed bool, startedAt time.Time) {
	toolsLogger.Info(
		"onboarding_impact source=%s project_path=%s context=%s post_check_performed=%t duration_ms=%d",
		source,
		projectPath,
		normalizeOnboardingContext(contextKey),
		performed,
		time.Since(startedAt).Milliseconds(),
	)
}

// PerformOnboarding executa o onboarding e salva o registro.
func PerformOnboarding(projectPath string) (string, error) {
	return PerformOnboardingWithContext(projectPath, "")
}

// PerformOnboardingWithContext executa o onboarding e salva o registro por contexto.
func PerformOnboardingWithContext(projectPath, contextKey string) (string, error) {
	return PerformOnboardingWithContextAndOptions(context.Background(), projectPath, contextKey, OnboardingOptions{})
}

// PerformOnboardingWithContextAndOptions executa onboarding com persistencia e cancelamento configuraveis.
func PerformOnboardingWithContextAndOptions(
	ctx context.Context,
	projectPath, contextKey string,
	opts OnboardingOptions,
) (string, error) {
	startedAt := time.Now()
	normalizedContext := normalizeOnboardingContext(contextKey)
	defer func() {
		performed, _ := checkOnboardingPerformedWithOptions(projectPath, normalizedContext, opts)
		source := "manual"
		if opts.AutoRun {
			source = "auto"
		}
		logOnboardingImpact(source, projectPath, normalizedContext, performed, startedAt)
	}()

	if err := ctx.Err(); err != nil {
		return "", err
	}

	structure, err := ScanProjectStructureCtx(ctx, projectPath)
	if err != nil {
		return "", err
	}

	record, err := loadOnboardingRecord(projectPath, opts)
	if err != nil {
		if os.IsNotExist(err) {
			record = onboardingRecord{Contexts: make(map[string]onboardingContextRecord)}
		} else {
			return "", err
		}
	}

	record.Contexts[normalizedContext] = onboardingContextRecord{
		PerformedAt: time.Now(),
		UnitCount:   structure.UnitCount,
		EntryPoint:  structure.EntryPoint,
	}

	syncLegacyDefault(&record)

	if err := saveOnboardingRecord(projectPath, record, opts); err != nil {
		return "", err
	}

	structureJSON, _ := json.MarshalIndent(structure, "", "  ")
	return fmt.Sprintf("Onboarding concluido para %s:\n%s", projectPath, string(structureJSON)), nil
}

// ProjectOnboardingFileExists reports whether the legacy/opt-in project marker file exists.
func ProjectOnboardingFileExists(projectPath string) bool {
	_, err := os.Stat(projectOnboardingFilePath(projectPath))
	return err == nil
}
