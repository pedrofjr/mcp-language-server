package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// NFRCorpusStats descreve o workspace Delphi usado no gate NFR representativo.
type NFRCorpusStats struct {
	Root            string
	DelphiFileCount int
	TotalBytes      int64
	SampleFiles     []string
}

var delphiSourceExtensions = map[string]struct{}{
	".pas": {},
	".dpr": {},
	".dpk": {},
}

func resolveNFRCorpusRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	candidates := make([]string, 0, 4)
	if env := strings.TrimSpace(os.Getenv("MCP_NFR_DELPHI_CORPUS_ROOT")); env != "" {
		candidates = append(candidates, env)
	}

	candidates = append(candidates,
		filepath.Join(wd, "third_party", "nfr-delphi-corpus"),
		filepath.Join(wd, "..", "Delphi_Oracle", "oracle-lsp", "test-fixtures"),
	)

	for _, candidate := range candidates {
		cleaned := filepath.Clean(candidate)
		info, err := os.Stat(cleaned)
		if err != nil || !info.IsDir() {
			continue
		}
		if stats, err := describeNFRCorpus(cleaned); err != nil {
			continue
		} else if stats.DelphiFileCount > 0 {
			return cleaned, nil
		}
	}

	return "", fmt.Errorf(
		"representative Delphi corpus not found (set MCP_NFR_DELPHI_CORPUS_ROOT or sync third_party/nfr-delphi-corpus)",
	)
}

func describeNFRCorpus(root string) (NFRCorpusStats, error) {
	stats := NFRCorpusStats{Root: filepath.Clean(root)}

	err := filepath.WalkDir(stats.Root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			if path != stats.Root && shouldSkipNFRCorpusDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if _, ok := delphiSourceExtensions[ext]; !ok {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return nil
		}

		stats.DelphiFileCount++
		stats.TotalBytes += info.Size()
		if len(stats.SampleFiles) < 5 {
			stats.SampleFiles = append(stats.SampleFiles, path)
		}
		return nil
	})
	if err != nil {
		return NFRCorpusStats{}, err
	}

	sort.Strings(stats.SampleFiles)
	if stats.DelphiFileCount == 0 {
		return NFRCorpusStats{}, fmt.Errorf("corpus at %s has no Delphi source files", stats.Root)
	}

	return stats, nil
}

func shouldSkipNFRCorpusDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", ".hg", ".svn", "node_modules", "vendor", "target", "build", "dist", ".idea", ".vscode", ".oracle-lsp":
		return true
	default:
		return false
	}
}

func formatNFRCorpusReport(stats NFRCorpusStats, samples int, p95 string) string {
	return fmt.Sprintf(
		"NFR Delphi corpus root=%s files=%d bytes=%d sample_paths=%v tool_samples=%d p95=%s",
		stats.Root,
		stats.DelphiFileCount,
		stats.TotalBytes,
		stats.SampleFiles,
		samples,
		p95,
	)
}
