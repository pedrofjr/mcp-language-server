package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// ErrDiagnosticsUnavailable indicates diagnostics could not be loaded reliably.
var ErrDiagnosticsUnavailable = errors.New("diagnostics unavailable")

// GetDiagnosticsForFile retrieves diagnostics for a specific file from the language server
func GetDiagnosticsForFile(ctx context.Context, client *lsp.Client, filePath string, contextLines int, showLineNumbers bool) (string, error) {
	// Override with environment variable if specified
	if envLines := os.Getenv("LSP_CONTEXT_LINES"); envLines != "" {
		if val, err := strconv.Atoi(envLines); err == nil && val >= 0 {
			contextLines = val
		}
	}

	normalizedPath, err := normalizeFilePathOrURI(filePath)
	if err != nil {
		return "", fmt.Errorf("invalid file path or URI: %v", err)
	}

	err = client.OpenFile(ctx, normalizedPath)
	if err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	uri := protocol.URIFromPath(normalizedPath)
	diagParams := protocol.DocumentDiagnosticParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	}

	report, pullErr := client.Diagnostic(ctx, diagParams)
	diagnostics, resolveErr := resolveDiagnosticsForFile(ctx, client, uri, report, pullErr)
	if resolveErr != nil {
		return "", resolveErr
	}

	if len(diagnostics) == 0 {
		return "No diagnostics found for " + filePath, nil
	}

	return formatDiagnosticsOutput(ctx, client, filePath, normalizedPath, uri, diagnostics, contextLines, showLineNumbers)
}

func resolveDiagnosticsForFile(
	ctx context.Context,
	client *lsp.Client,
	uri protocol.DocumentUri,
	report protocol.DocumentDiagnosticReport,
	pullErr error,
) ([]protocol.Diagnostic, error) {
	if pullErr == nil {
		items, unchanged, extractErr := diagnosticsFromPullReport(report)
		if extractErr != nil {
			return nil, extractErr
		}
		if unchanged {
			if waitErr := client.WaitForDiagnosticPublish(ctx, uri); waitErr != nil && !errors.Is(waitErr, context.Canceled) {
				return nil, fmt.Errorf("%w: waiting for published diagnostics: %v", ErrDiagnosticsUnavailable, waitErr)
			}
			return diagnosticsFromValidCache(client, uri)
		}

		if len(items) > 0 {
			client.UpdateFileDiagnostics(uri, items)
			return items, nil
		}

		if waitErr := client.WaitForDiagnosticPublish(ctx, uri); waitErr != nil && !errors.Is(waitErr, context.Canceled) {
			return nil, fmt.Errorf("%w: waiting for published diagnostics after empty pull: %v", ErrDiagnosticsUnavailable, waitErr)
		}
		if cached, cacheErr := diagnosticsFromValidCache(client, uri); cacheErr == nil && len(cached) > 0 {
			return cached, nil
		}

		client.UpdateFileDiagnostics(uri, items)
		return items, nil
	}

	toolsLogger.Error("Failed to get diagnostics: %v", pullErr)

	if waitErr := client.WaitForDiagnosticPublish(ctx, uri); waitErr != nil && !errors.Is(waitErr, context.Canceled) {
		return nil, fmt.Errorf("%w: textDocument/diagnostic failed (%v) and publish wait ended: %v", ErrDiagnosticsUnavailable, pullErr, waitErr)
	}

	diagnostics, cacheErr := diagnosticsFromValidCache(client, uri)
	if cacheErr != nil {
		return nil, fmt.Errorf("%w: textDocument/diagnostic failed: %v", ErrDiagnosticsUnavailable, pullErr)
	}
	return diagnostics, nil
}

func diagnosticsFromPullReport(report protocol.DocumentDiagnosticReport) ([]protocol.Diagnostic, bool, error) {
	switch typed := report.Value.(type) {
	case protocol.RelatedFullDocumentDiagnosticReport:
		return typed.Items, false, nil
	case protocol.RelatedUnchangedDocumentDiagnosticReport:
		return nil, true, nil
	default:
		return nil, false, fmt.Errorf("%w: unsupported diagnostic report type %T", ErrDiagnosticsUnavailable, report.Value)
	}
}

func diagnosticsFromValidCache(client *lsp.Client, uri protocol.DocumentUri) ([]protocol.Diagnostic, error) {
	entry, ok := client.GetFileDiagnosticCacheEntry(uri)
	if !ok {
		return nil, fmt.Errorf("%w: no cached diagnostics for current document version", ErrDiagnosticsUnavailable)
	}

	openVersion, openOk := client.GetOpenFileVersionByURI(uri)
	if !openOk {
		return nil, fmt.Errorf("%w: document is not open in LSP client", ErrDiagnosticsUnavailable)
	}

	if !diagnosticCacheValidForOpenVersion(entry, openVersion) {
		return nil, fmt.Errorf(
			"%w: cached diagnostics are stale (open version %d, cache version %d)",
			ErrDiagnosticsUnavailable,
			openVersion,
			entry.PublishVersion,
		)
	}

	return entry.Diagnostics, nil
}

func diagnosticCacheValidForOpenVersion(entry lsp.FileDiagnosticCache, openVersion int32) bool {
	if entry.PublishVersion == 0 {
		// Servers omitting publish version: accept cache only after an explicit publish notification.
		return true
	}
	return entry.PublishVersion == openVersion
}

func formatDiagnosticsOutput(
	ctx context.Context,
	client *lsp.Client,
	filePath string,
	normalizedPath string,
	uri protocol.DocumentUri,
	diagnostics []protocol.Diagnostic,
	contextLines int,
	showLineNumbers bool,
) (string, error) {
	fileInfo := fmt.Sprintf("%s\nDiagnostics in File: %d\n",
		filePath,
		len(diagnostics),
	)

	var diagSummaries []string
	var diagLocations []protocol.Location

	for _, diag := range diagnostics {
		severity := getSeverityString(diag.Severity)
		location := fmt.Sprintf("L%d:C%d",
			diag.Range.Start.Line+1,
			diag.Range.Start.Character+1)

		summary := fmt.Sprintf("%s at %s: %s",
			severity,
			location,
			diag.Message)

		if diag.Source != "" {
			summary += fmt.Sprintf(" (Source: %s", diag.Source)
			if diag.Code != nil {
				summary += fmt.Sprintf(", Code: %v", diag.Code)
			}
			summary += ")"
		} else if diag.Code != nil {
			summary += fmt.Sprintf(" (Code: %v)", diag.Code)
		}

		diagSummaries = append(diagSummaries, summary)
		diagLocations = append(diagLocations, protocol.Location{
			URI:   uri,
			Range: diag.Range,
		})
	}

	fileContent, err := os.ReadFile(normalizedPath)
	if err != nil {
		return fileInfo + "\nError reading file: " + err.Error(), nil
	}

	lines := strings.Split(string(fileContent), "\n")

	var linesToShow map[int]bool
	if contextLines > 0 {
		linesToShow, err = GetLineRangesToDisplay(ctx, client, diagLocations, len(lines), contextLines)
		if err != nil {
			linesToShow = make(map[int]bool)
			for _, diag := range diagnostics {
				linesToShow[int(diag.Range.Start.Line)] = true
			}
		}
	} else {
		linesToShow = make(map[int]bool)
		for _, diag := range diagnostics {
			linesToShow[int(diag.Range.Start.Line)] = true
		}
	}

	lineRanges := ConvertLinesToRanges(linesToShow, len(lines))

	result := fileInfo
	if len(diagSummaries) > 0 {
		result += strings.Join(diagSummaries, "\n") + "\n"
	}

	if showLineNumbers {
		result += "\n" + FormatLinesWithRanges(lines, lineRanges)
	}

	return result, nil
}

func getSeverityString(severity protocol.DiagnosticSeverity) string {
	switch severity {
	case protocol.SeverityError:
		return "ERROR"
	case protocol.SeverityWarning:
		return "WARNING"
	case protocol.SeverityInformation:
		return "INFO"
	case protocol.SeverityHint:
		return "HINT"
	default:
		return "UNKNOWN"
	}
}

// GetDiagnosticsForSymbol retorna diagnosticos relevantes para um simbolo pelo nome.
func GetDiagnosticsForSymbol(ctx context.Context, client *lsp.Client, filePath, symbolName string) (string, error) {
	allDiags, err := GetDiagnosticsForFile(ctx, client, filePath, 2, true)
	if err != nil {
		return "", err
	}

	if symbolName == "" {
		return allDiags, nil
	}

	var relevant []string
	lowerSymbol := strings.ToLower(symbolName)
	for _, line := range strings.Split(allDiags, "\n") {
		if strings.Contains(strings.ToLower(line), lowerSymbol) {
			relevant = append(relevant, line)
		}
	}

	if len(relevant) == 0 {
		return fmt.Sprintf("Nenhum diagnostico encontrado para o simbolo %q em %s", symbolName, filePath), nil
	}

	return strings.Join(relevant, "\n"), nil
}
