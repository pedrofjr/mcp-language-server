package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type activeProjectState struct {
	Dir string `json:"dir"`
}

func activeProjectFile() string {
	memoryDir := os.Getenv("ORACLE_MEMORY_DIR")
	if memoryDir == "" {
		memoryDir = filepath.Join(os.TempDir(), "oracle_memory")
	}
	return filepath.Join(memoryDir, "active_project.json")
}

// ActivateProject valida um diretório Delphi e persiste seu caminho como projeto ativo.
func ActivateProject(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("cannot read dir %q: %w", dir, err)
	}

	for _, entry := range entries {
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".dpr" && ext != ".dpk" {
			continue
		}

		payload, err := json.Marshal(activeProjectState{Dir: dir})
		if err != nil {
			return "", err
		}

		stateFile := activeProjectFile()
		if err := os.MkdirAll(filepath.Dir(stateFile), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(stateFile, payload, 0o644); err != nil {
			return "", err
		}
		return dir, nil
	}

	return "", errors.New("no .dpr or .dpk file found in directory")
}

// GetActiveProject retorna o diretório previamente persistido como projeto ativo.
func GetActiveProject() (string, error) {
	data, err := os.ReadFile(activeProjectFile())
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.New("no active project set")
		}
		return "", err
	}

	var state activeProjectState
	if err := json.Unmarshal(data, &state); err != nil {
		return "", err
	}
	return state.Dir, nil
}
