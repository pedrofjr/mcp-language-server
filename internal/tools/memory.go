package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryEntry representa uma entrada no sistema de memoria do agente.
type MemoryEntry struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

var memoryMu sync.Mutex

// memoryDir retorna o diretorio de memoria (configuravel via ORACLE_MEMORY_DIR para testes).
func memoryDir() (string, error) {
	if dir := os.Getenv("ORACLE_MEMORY_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", err
		}
		return dir, nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(configDir, "oracle-mcp")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func memoryFilePath() (string, error) {
	dir, err := memoryDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "memory.json"), nil
}

func loadMemory() (map[string]MemoryEntry, error) {
	path, err := memoryFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(map[string]MemoryEntry), nil
	}
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return make(map[string]MemoryEntry), nil
	}

	var entries map[string]MemoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}

	if entries == nil {
		entries = make(map[string]MemoryEntry)
	}

	return entries, nil
}

func saveMemory(entries map[string]MemoryEntry) error {
	path, err := memoryFilePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

// MemoryWrite cria uma nova entrada de memoria e retorna o ID gerado.
func MemoryWrite(title, content string, tags []string) (string, error) {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	entries, err := loadMemory()
	if err != nil {
		return "", err
	}

	id := uuid.New().String()
	if tags == nil {
		tags = []string{}
	}

	now := time.Now()
	entries[id] = MemoryEntry{
		ID:        id,
		Title:     title,
		Content:   content,
		Tags:      tags,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := saveMemory(entries); err != nil {
		return "", err
	}

	return id, nil
}

// MemoryRead retorna a entrada pelo ID.
func MemoryRead(id string) (*MemoryEntry, error) {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	entries, err := loadMemory()
	if err != nil {
		return nil, err
	}

	entry, ok := entries[id]
	if !ok {
		return nil, fmt.Errorf("entrada de memoria %q nao encontrada", id)
	}

	return &entry, nil
}

// MemoryList retorna todas as entradas, opcionalmente filtradas por tag.
func MemoryList(tag string) ([]*MemoryEntry, error) {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	entries, err := loadMemory()
	if err != nil {
		return nil, err
	}

	result := make([]*MemoryEntry, 0, len(entries))
	for _, entry := range entries {
		if tag == "" {
			ec := entry
			result = append(result, &ec)
			continue
		}

		for _, currentTag := range entry.Tags {
			if currentTag == tag {
				ec := entry
				result = append(result, &ec)
				break
			}
		}
	}

	return result, nil
}

// MemoryEdit atualiza o conteudo de uma entrada existente.
func MemoryEdit(id, newContent string) error {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	entries, err := loadMemory()
	if err != nil {
		return err
	}

	entry, ok := entries[id]
	if !ok {
		return fmt.Errorf("entrada de memoria %q nao encontrada", id)
	}

	entry.Content = newContent
	entry.UpdatedAt = time.Now()
	entries[id] = entry

	return saveMemory(entries)
}

// MemoryDelete remove uma entrada pelo ID.
func MemoryDelete(id string) error {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	entries, err := loadMemory()
	if err != nil {
		return err
	}

	if _, ok := entries[id]; !ok {
		return fmt.Errorf("entrada de memoria %q nao encontrada", id)
	}

	delete(entries, id)
	return saveMemory(entries)
}
