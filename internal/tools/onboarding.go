package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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

// CheckOnboardingPerformed verifica se o onboarding ja foi executado para o projeto.
func CheckOnboardingPerformed(projectPath string) (bool, time.Time) {
	data, err := os.ReadFile(onboardingFilePath(projectPath))
	if err != nil {
		return false, time.Time{}
	}

	var record struct {
		PerformedAt time.Time `json:"performed_at"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		return false, time.Time{}
	}

	return true, record.PerformedAt
}

// PerformOnboarding executa o onboarding e salva o registro.
func PerformOnboarding(projectPath string) (string, error) {
	structure, err := ScanProjectStructure(projectPath)
	if err != nil {
		return "", err
	}

	record := map[string]any{
		"performed_at": time.Now(),
		"unit_count":   structure.UnitCount,
		"entry_point":  structure.EntryPoint,
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err == nil {
		_ = os.WriteFile(onboardingFilePath(projectPath), data, 0o644)
	}

	structureJSON, _ := json.MarshalIndent(structure, "", "  ")
	return fmt.Sprintf("Onboarding concluido para %s:\n%s", projectPath, string(structureJSON)), nil
}
