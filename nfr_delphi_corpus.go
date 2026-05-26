package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/tools"
)

// NFRCorpusStats descreve o workspace Delphi usado no gate NFR representativo.
type NFRCorpusStats struct {
	Root             string
	DelphiFileCount  int
	TotalBytes       int64
	SampleFiles      []string
	FilesByExtension map[string]int
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
		if err := ensureNFRCorpusMatrixExtensions(cleaned); err != nil {
			continue
		}
		stats, err := describeNFRCorpus(cleaned)
		if err != nil {
			continue
		}
		if stats.DelphiFileCount > 0 && hasRequiredMatrixExtensions(stats.FilesByExtension) {
			return cleaned, nil
		}
	}

	return "", fmt.Errorf(
		"representative Delphi corpus not found with .pas/.inc/.pp/.lpr (set MCP_NFR_DELPHI_CORPUS_ROOT, sync third_party/nfr-delphi-corpus, or use Delphi_Oracle test-fixtures)",
	)
}

func describeNFRCorpus(root string) (NFRCorpusStats, error) {
	stats := NFRCorpusStats{
		Root:             filepath.Clean(root),
		FilesByExtension: make(map[string]int),
	}

	if err := ensureNFRCorpusMatrixExtensions(stats.Root); err != nil {
		return NFRCorpusStats{}, err
	}

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

		if !tools.IsDelphiWorkspaceSourceFile(entry.Name()) {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		stats.DelphiFileCount++
		stats.TotalBytes += info.Size()
		stats.FilesByExtension[ext]++
		if len(stats.SampleFiles) < 8 {
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
	if !hasRequiredMatrixExtensions(stats.FilesByExtension) {
		return NFRCorpusStats{}, fmt.Errorf(
			"corpus at %s missing required matrix extensions (.inc/.pp/.lpr); matrix=%s",
			stats.Root,
			tools.DelphiWorkspaceExtensionsDoc,
		)
	}

	return stats, nil
}

func hasRequiredMatrixExtensions(counts map[string]int) bool {
	for _, ext := range []string{".inc", ".pp", ".lpr"} {
		if counts[ext] < 1 {
			return false
		}
	}
	return true
}

func ensureNFRCorpusMatrixExtensions(root string) error {
	fixtures := []struct {
		name    string
		content string
	}{
		{
			name: "nfr_matrix_gate.inc",
			content: strings.Join([]string{
				"{$IFDEF NFR_MATRIX}",
				"procedure NfrMatrixGateIncToken;",
				"const NfrMatrixGateIncConst = 42;",
				"{$ENDIF}",
			}, "\n"),
		},
		{
			name: "nfr_matrix_gate.pp",
			content: strings.Join([]string{
				"program NfrMatrixGatePp;",
				"begin",
				"end.",
			}, "\n"),
		},
		{
			name: "nfr_matrix_gate.lpr",
			content: strings.Join([]string{
				"program NfrMatrixGateLpr;",
				"uses",
				"  SysUtils;",
				"begin",
				"end.",
			}, "\n"),
		},
		{
			name: "nfr_matrix_cross.pas",
			content: strings.Join([]string{
				"unit NfrMatrixCross;",
				"interface",
				"procedure NfrMatrixCrossPasToken;",
				"implementation",
				"procedure NfrMatrixCrossPasToken; begin end;",
				"end.",
			}, "\n"),
		},
		{
			name: "nfr_matrix_cross.dpr",
			content: strings.Join([]string{
				"program NfrMatrixCross;",
				"uses",
				"  NfrMatrixCross in 'nfr_matrix_cross.pas';",
				"begin",
				"end.",
			}, "\n"),
		},
	}

	for _, fixture := range fixtures {
		path := filepath.Join(root, fixture.name)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("mkdir corpus matrix fixture dir for %s: %w", fixture.name, err)
		}
		if err := os.WriteFile(path, []byte(fixture.content), 0o600); err != nil {
			return fmt.Errorf("write corpus matrix fixture %s: %w", fixture.name, err)
		}
	}

	return nil
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
	keys := make([]string, 0, len(stats.FilesByExtension))
	for ext := range stats.FilesByExtension {
		keys = append(keys, ext)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, ext := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", ext, stats.FilesByExtension[ext]))
	}

	return fmt.Sprintf(
		"NFR Delphi corpus root=%s matrix=%s files=%d bytes=%d by_ext=[%s] sample_paths=%v tool_samples=%d p95=%s",
		stats.Root,
		tools.DelphiWorkspaceExtensionsDoc,
		stats.DelphiFileCount,
		stats.TotalBytes,
		strings.Join(parts, " "),
		stats.SampleFiles,
		samples,
		p95,
	)
}

func nfrCorpusIncPath(stats NFRCorpusStats) (string, bool) {
	var matches []string
	err := filepath.WalkDir(stats.Root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".inc") {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", false
	}
	if len(matches) == 0 {
		return "", false
	}
	sort.Strings(matches)
	return matches[0], true
}
