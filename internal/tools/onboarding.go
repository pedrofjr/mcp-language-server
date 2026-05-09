package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultOnboardingContext = "default"

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

// ScanProjectStructure escaneia um diretorio Delphi e retorna a estrutura do projeto.
func ScanProjectStructure(projectPath string) (*ProjectStructure, error) {
	result := &ProjectStructure{
		ProjectPath: projectPath,
		ScannedAt:   time.Now(),
	}

	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info == nil || info.IsDir() {
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

// onboardingFilePath retorna o caminho do arquivo de flag de onboarding.
func onboardingFilePath(projectPath string) string {
	return filepath.Join(projectPath, ".oracle-onboarding.json")
}

func normalizeOnboardingContext(contextKey string) string {
	normalized := strings.TrimSpace(contextKey)
	if normalized == "" {
		return defaultOnboardingContext
	}
	return normalized
}

func loadOnboardingRecord(projectPath string) (onboardingRecord, error) {
	data, err := os.ReadFile(onboardingFilePath(projectPath))
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

func syncLegacyDefault(record *onboardingRecord) {
	defaultRecord, hasDefault := record.Contexts[defaultOnboardingContext]
	if !hasDefault {
		// Compatibilidade retroativa: quando apenas contextos nao-default sao gravados,
		// preservamos o topo legado existente ao inves de limpa-lo.
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
	record, err := loadOnboardingRecord(projectPath)
	if err != nil {
		return false, time.Time{}
	}

	contextKey = normalizeOnboardingContext(contextKey)

	if contextRecord, ok := record.Contexts[contextKey]; ok && !contextRecord.PerformedAt.IsZero() {
		return true, contextRecord.PerformedAt
	}

	if contextKey == defaultOnboardingContext {
		if _, hasDefault := record.Contexts[defaultOnboardingContext]; hasDefault {
			// Precedencia estrita: se contexts.default existe, nunca usar fallback legado.
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
	info, err := os.Stat(projectPath)
	if err != nil || !info.IsDir() {
		return false, "Proximo passo: execute onboarding e rode check_onboarding_performed novamente. Se nao houver arquivos Delphi (.pas/.dpr/.dpk), o onboarding pode nao encontrar unidades."
	}

	hasDelphiSource := false
	_ = filepath.Walk(projectPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil || info.IsDir() {
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
	startedAt := time.Now()
	normalizedContext := normalizeOnboardingContext(contextKey)
	defer func() {
		performed, _ := CheckOnboardingPerformedWithContext(projectPath, normalizedContext)
		source := "manual"
		if normalizedContext == defaultOnboardingContext {
			source = "auto"
		}
		logOnboardingImpact(source, projectPath, normalizedContext, performed, startedAt)
	}()

	structure, err := ScanProjectStructure(projectPath)
	if err != nil {
		return "", err
	}

	contextKey = normalizedContext

	record, err := loadOnboardingRecord(projectPath)
	if err != nil {
		record = onboardingRecord{Contexts: make(map[string]onboardingContextRecord)}
	}

	record.Contexts[contextKey] = onboardingContextRecord{
		PerformedAt: time.Now(),
		UnitCount:   structure.UnitCount,
		EntryPoint:  structure.EntryPoint,
	}

	syncLegacyDefault(&record)

	data, err := json.MarshalIndent(record, "", "  ")
	if err == nil {
		_ = os.WriteFile(onboardingFilePath(projectPath), data, 0o644)
	}

	structureJSON, _ := json.MarshalIndent(structure, "", "  ")
	return fmt.Sprintf("Onboarding concluido para %s:\n%s", projectPath, string(structureJSON)), nil
}
