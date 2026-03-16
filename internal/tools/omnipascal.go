package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/omnipascal"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/utilities"
)

func OmniPascalSetConfig(ctx context.Context, client *omnipascal.Client, config map[string]any) (string, error) {
	if err := client.SetConfig(ctx, config); err != nil {
		return "", err
	}

	keys := make([]string, 0, len(config))
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	return fmt.Sprintf("OmniPascal configuration synchronized: %s", strings.Join(keys, ", ")), nil
}

func OmniPascalOpenFile(ctx context.Context, client *omnipascal.Client, filePath string) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", err
	}
	return fmt.Sprintf("Opened %s in OmniPascal.", filePath), nil
}

func OmniPascalCloseFile(ctx context.Context, client *omnipascal.Client, filePath string) (string, error) {
	if err := client.CloseFile(ctx, filePath); err != nil {
		return "", err
	}
	return fmt.Sprintf("Closed %s in OmniPascal.", filePath), nil
}

func OmniPascalChangeFile(ctx context.Context, client *omnipascal.Client, args omnipascal.ChangeArgs) (string, error) {
	args.InsertString = encodeURIComponent(args.InsertString)
	if err := client.ChangeFile(ctx, args); err != nil {
		return "", err
	}
	return fmt.Sprintf("Sent change notification for %s at L%d:C%d.", args.File, args.Line, args.Offset), nil
}

func OmniPascalGetDiagnostics(ctx context.Context, client *omnipascal.Client, filePaths []string, delayMs, waitMs int) (string, error) {
	before := make(map[string]string, len(filePaths))
	for _, filePath := range filePaths {
		if err := client.OpenFile(ctx, filePath); err != nil {
			return "", fmt.Errorf("failed to open %s before geterr: %w", filePath, err)
		}
		before[filePath] = diagnosticsSignature(client.GetFileDiagnostics(filePath))
	}

	if err := client.GetErr(ctx, filePaths, delayMs); err != nil {
		return "", err
	}

	if waitMs > 0 {
		deadline := time.Now().Add(time.Duration(waitMs) * time.Millisecond)
		for {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}

			changed := false
			for _, filePath := range filePaths {
				after := diagnosticsSignature(client.GetFileDiagnostics(filePath))
				if after != before[filePath] {
					changed = true
					break
				}
			}
			if changed || time.Now().After(deadline) {
				break
			}

			// Poll frequently enough to catch async event propagation without
			// introducing long fixed sleeps in LLM workflows.
			timer := time.NewTimer(100 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			case <-timer.C:
			}
		}
	}

	var sections []string
	for _, filePath := range filePaths {
		diagnostics := client.GetFileDiagnostics(filePath)
		if len(diagnostics) == 0 {
			sections = append(sections, fmt.Sprintf("%s\nNo diagnostics.", filePath))
			continue
		}

		var lines []string
		for _, diagnostic := range diagnostics {
			lines = append(lines, fmt.Sprintf("%s L%d:C%d-L%d:C%d %s",
				severityLabel(diagnostic.Severity),
				diagnostic.Start.Line,
				diagnostic.Start.Offset,
				diagnostic.End.Line,
				diagnostic.End.Offset,
				diagnostic.Text,
			))
		}
		sections = append(sections, fmt.Sprintf("%s\n%s", filePath, strings.Join(lines, "\n")))
	}

	return strings.Join(sections, "\n\n"), nil
}

func diagnosticsSignature(diagnostics []omnipascal.Diagnostic) string {
	if len(diagnostics) == 0 {
		return ""
	}

	var b strings.Builder
	for _, diagnostic := range diagnostics {
		b.WriteString(fmt.Sprintf("%d|%d|%d|%d|%d|%s\n",
			diagnostic.Severity,
			diagnostic.Start.Line,
			diagnostic.Start.Offset,
			diagnostic.End.Line,
			diagnostic.End.Offset,
			diagnostic.Text,
		))
	}

	return b.String()
}

func OmniPascalCompletions(ctx context.Context, client *omnipascal.Client, filePath string, line, column int) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", err
	}

	var completions []omnipascal.Completion
	if err := client.Call(ctx, "completions", map[string]any{
		"file":   filePath,
		"line":   line,
		"offset": column,
	}, &completions); err != nil {
		return "", err
	}

	if len(completions) == 0 {
		return "No completions found.", nil
	}

	var lines []string
	for _, completion := range completions {
		entry := fmt.Sprintf("%s | kind=%d", completion.Name, completion.Kind)
		if completion.TypeLabel != "" {
			entry += " | " + completion.TypeLabel
		}
		if completion.Snippet != "" {
			entry += " | snippet=" + completion.Snippet
		}
		lines = append(lines, entry)
	}

	return strings.Join(lines, "\n"), nil
}

func OmniPascalDefinition(ctx context.Context, client *omnipascal.Client, filePath string, line, column int) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", err
	}

	var locations []omnipascal.Location
	if err := client.Call(ctx, "definition", map[string]any{
		"file":   filePath,
		"line":   line,
		"offset": column,
	}, &locations); err != nil {
		return "", err
	}

	if len(locations) == 0 {
		return "Definition not found.", nil
	}

	sort.Slice(locations, func(i, j int) bool {
		if locations[i].File != locations[j].File {
			return locations[i].File < locations[j].File
		}
		if locations[i].Start.Line != locations[j].Start.Line {
			return locations[i].Start.Line < locations[j].Start.Line
		}
		return locations[i].Start.Offset < locations[j].Start.Offset
	})

	var lines []string
	for _, location := range locations {
		lines = append(lines, fmt.Sprintf("%s L%d:C%d-L%d:C%d",
			location.File,
			location.Start.Line,
			location.Start.Offset,
			location.End.Line,
			location.End.Offset,
		))
	}

	return strings.Join(lines, "\n"), nil
}

func OmniPascalQuickInfo(ctx context.Context, client *omnipascal.Client, filePath string, line, column int) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", err
	}

	var info omnipascal.QuickInfo
	if err := client.Call(ctx, "quickinfo", map[string]any{
		"file":   filePath,
		"line":   line,
		"offset": column,
	}, &info); err != nil {
		return "", err
	}

	var parts []string
	if info.DisplayString != "" {
		parts = append(parts, info.DisplayString)
	}
	parts = append(parts, fmt.Sprintf("Range: L%d:C%d-L%d:C%d", info.Start.Line, info.Start.Offset, info.End.Line, info.End.Offset))
	if info.Documentation != "" {
		parts = append(parts, info.Documentation)
	}

	return strings.Join(parts, "\n\n"), nil
}

func OmniPascalSignatureHelp(ctx context.Context, client *omnipascal.Client, filePath string, line, column int) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", err
	}

	var info omnipascal.SignatureHelp
	if err := client.Call(ctx, "signatureHelp", map[string]any{
		"file":   filePath,
		"line":   line,
		"offset": column,
	}, &info); err != nil {
		return "", err
	}

	if len(info.Items) == 0 {
		return "No signature help available.", nil
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Active signature: %d", info.SelectedItemIndex))
	lines = append(lines, fmt.Sprintf("Active parameter: %d", info.ArgumentIndex))
	for index, item := range info.Items {
		var parameters []string
		for _, parameter := range item.Parameters {
			parameters = append(parameters, parameter.Label)
		}
		lines = append(lines, fmt.Sprintf("[%d] %s(%s)", index, item.Name, strings.Join(parameters, "; ")))
	}

	return strings.Join(lines, "\n"), nil
}

func OmniPascalDocumentSymbols(ctx context.Context, client *omnipascal.Client, filePath string) (string, error) {
	if err := client.OpenFile(ctx, filePath); err != nil {
		return "", err
	}

	var symbols []omnipascal.DocumentSymbol
	if err := client.Call(ctx, "textDocument/documentSymbol", map[string]any{
		"file": filePath,
	}, &symbols); err != nil {
		return "", err
	}

	if len(symbols) == 0 {
		return "No document symbols found.", nil
	}

	var lines []string
	for _, symbol := range symbols {
		appendDocumentSymbol(&lines, symbol, 0)
	}
	return strings.Join(lines, "\n"), nil
}

func OmniPascalWorkspaceSymbols(ctx context.Context, client *omnipascal.Client, query string) (string, error) {
	var symbols []omnipascal.WorkspaceSymbol
	if err := client.Call(ctx, "workspace/symbol", map[string]any{
		"query": query,
	}, &symbols); err != nil {
		return "", err
	}

	if len(symbols) == 0 {
		return "No workspace symbols found.", nil
	}

	var lines []string
	for _, symbol := range symbols {
		line := fmt.Sprintf("%s | kind=%d | %s L%d:C%d",
			symbol.Name,
			symbol.Kind,
			symbol.Location.File,
			symbol.Location.Line,
			symbol.Location.Offset,
		)
		if symbol.ContainerName != "" {
			line += " | container=" + symbol.ContainerName
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n"), nil
}

func OmniPascalGetProjectFiles(ctx context.Context, client *omnipascal.Client, filePath string) (string, error) {
	args := map[string]any{}
	if filePath != "" {
		args["file"] = filePath
	}

	var projects []omnipascal.ProjectFile
	if err := client.Call(ctx, "getProjectFiles", args, &projects); err != nil {
		return "", err
	}

	if len(projects) == 0 {
		return "No project files found.", nil
	}

	var lines []string
	for _, project := range projects {
		lines = append(lines, fmt.Sprintf("%s | %s", project.Name, project.TypeLabel))
	}
	return strings.Join(lines, "\n"), nil
}

func OmniPascalLoadProject(ctx context.Context, client *omnipascal.Client, filePath string) (string, error) {
	var result json.RawMessage
	if err := client.Call(ctx, "loadProject", map[string]any{"file": filePath}, &result); err != nil {
		return "", err
	}
	return fmt.Sprintf("Loaded project %s.", filePath), nil
}

func OmniPascalGetAllUnits(ctx context.Context, client *omnipascal.Client, filePath string) (string, error) {
	var units []omnipascal.Unit
	if err := client.Call(ctx, "getAllUnits", map[string]any{"file": filePath}, &units); err != nil {
		return "", err
	}

	if len(units) == 0 {
		return "No units found.", nil
	}

	var names []string
	for _, unit := range units {
		names = append(names, unit.Name)
	}
	sort.Strings(names)
	return strings.Join(names, "\n"), nil
}

func OmniPascalGetPossibleUsesSections(ctx context.Context, client *omnipascal.Client, filePath string) (string, error) {
	var response omnipascal.UsesSectionsResponse
	if err := client.Call(ctx, "getPossibleUsesSections", map[string]any{"filename": filePath}, &response); err != nil {
		return "", err
	}

	if len(response.Sections) == 0 {
		return "No uses sections available.", nil
	}

	return strings.Join(response.Sections, "\n"), nil
}

func OmniPascalGetCodeActions(ctx context.Context, client *omnipascal.Client, filePath string, selection omnipascal.TextSpan) (string, error) {
	var response omnipascal.CodeActionList
	if err := client.Call(ctx, "getCodeActions", map[string]any{
		"filename":  filePath,
		"selection": selection,
	}, &response); err != nil {
		return "", err
	}

	if len(response.CodeActions) == 0 {
		return "No code actions available.", nil
	}

	var lines []string
	for _, action := range response.CodeActions {
		lines = append(lines, fmt.Sprintf("%s | %s", action.Title, action.Command))
	}

	return strings.Join(lines, "\n"), nil
}

func OmniPascalRunCodeAction(ctx context.Context, client *omnipascal.Client, filePath, identifier string, selection omnipascal.TextSpan, wantsTextChanges, applyChanges bool) (string, error) {
	var response omnipascal.CodeActionResponse
	if err := client.Call(ctx, "runCodeAction", map[string]any{
		"filename":         filePath,
		"identifier":       identifier,
		"selection":        selection,
		"wantsTextChanges": wantsTextChanges,
	}, &response); err != nil {
		return "", err
	}

	if applyChanges {
		return applyOmniPascalChanges(ctx, client, response)
	}

	return formatJSON(response)
}

func OmniPascalAddUses(ctx context.Context, client *omnipascal.Client, filePath, usesSection, unitToAdd string, applyChanges bool) (string, error) {
	var response omnipascal.CodeActionResponse
	if err := client.Call(ctx, "addUses", map[string]any{
		"filename":    filePath,
		"usesSection": usesSection,
		"unitToAdd":   unitToAdd,
	}, &response); err != nil {
		return "", err
	}

	if applyChanges {
		return applyOmniPascalChanges(ctx, client, response)
	}

	return formatJSON(response)
}

func OmniPascalApplyTextEdits(ctx context.Context, client *omnipascal.Client, filePath string, edits []TextEdit) (string, error) {
	// Reuse the existing line-based edit tool logic, then force the backend to reread the file from disk.
	textEdits := make([]TextEdit, len(edits))
	copy(textEdits, edits)

	response, err := ApplyTextEditsWithoutClient(filePath, textEdits)
	if err != nil {
		return "", err
	}

	if err := client.ReopenFile(ctx, filePath); err != nil {
		return "", fmt.Errorf("applied edits but failed to resync OmniPascal: %w", err)
	}

	refreshChar, err := firstFileCharacter(filePath)
	if err != nil {
		return "", err
	}
	if refreshChar != "" {
		if err := client.ChangeFile(ctx, omnipascal.ChangeArgs{
			File:         filePath,
			Line:         1,
			Offset:       1,
			EndLine:      1,
			EndOffset:    2,
			InsertString: encodeURIComponent(refreshChar),
		}); err != nil {
			return "", fmt.Errorf("applied edits but failed to refresh OmniPascal buffer for %s: %w", filePath, err)
		}
	}

	// Trigger a diagnostics refresh request so subsequent geterr reads are less
	// likely to observe stale cache entries from a previous buffer state.
	if err := client.GetErr(ctx, []string{filePath}, 0); err != nil {
		return "", fmt.Errorf("applied edits but failed to refresh diagnostics for %s: %w", filePath, err)
	}

	return response, nil
}

func firstFileCharacter(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	if len(content) == 0 {
		return "", nil
	}
	return string(content[:1]), nil
}

func ApplyTextEditsWithoutClient(filePath string, edits []TextEdit) (string, error) {
	sortedEdits := make([]TextEdit, len(edits))
	copy(sortedEdits, edits)
	sort.Slice(sortedEdits, func(i, j int) bool {
		return sortedEdits[i].StartLine < sortedEdits[j].StartLine
	})

	linesRemoved := 0
	linesAdded := 0
	protocolEdits := make([]protocol.TextEdit, 0, len(edits))
	for _, edit := range edits {
		rng, err := getRange(edit.StartLine, edit.EndLine, filePath)
		if err != nil {
			return "", fmt.Errorf("invalid position: %v", err)
		}

		protocolEdits = append(protocolEdits, protocol.TextEdit{
			Range:   rng,
			NewText: edit.NewText,
		})

		linesRemoved += edit.EndLine - edit.StartLine + 1
		if edit.NewText != "" {
			linesAdded += strings.Count(edit.NewText, "\n") + 1
		}
	}

	if err := utilities.ApplyTextEdits(protocol.URIFromPath(filePath), protocolEdits); err != nil {
		return "", fmt.Errorf("failed to apply text edits: %w", err)
	}

	return fmt.Sprintf("Successfully applied text edits. %d lines removed, %d lines added.", linesRemoved, linesAdded), nil
}

func appendDocumentSymbol(lines *[]string, symbol omnipascal.DocumentSymbol, depth int) {
	indent := strings.Repeat("  ", depth)
	entry := fmt.Sprintf("%s- %s | kind=%d | L%d:C%d-L%d:C%d",
		indent,
		symbol.Name,
		symbol.Kind,
		symbol.Range.Start.Line,
		symbol.Range.Start.Offset,
		symbol.Range.End.Line,
		symbol.Range.End.Offset,
	)
	if symbol.Detail != "" {
		entry += " | " + symbol.Detail
	}
	*lines = append(*lines, entry)
	for _, child := range symbol.Children {
		appendDocumentSymbol(lines, child, depth+1)
	}
}

func applyOmniPascalChanges(ctx context.Context, client *omnipascal.Client, response omnipascal.CodeActionResponse) (string, error) {
	if len(response.Changes) == 0 {
		return "No text changes returned by OmniPascal.", nil
	}

	edit := protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentUri][]protocol.TextEdit{},
	}

	changeCount := 0
	files := make([]string, 0, len(response.Changes))
	for _, fileChange := range response.Changes {
		uri := protocol.URIFromPath(fileChange.FilePath)
		files = append(files, fileChange.FilePath)
		for _, change := range fileChange.Changes {
			edit.Changes[uri] = append(edit.Changes[uri], protocol.TextEdit{
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      toZeroBased(change.StartLine),
						Character: toZeroBased(change.StartColumn),
					},
					End: protocol.Position{
						Line:      toZeroBased(change.EndLine),
						Character: toZeroBased(change.EndColumn),
					},
				},
				NewText: change.InsertString,
			})
			changeCount++
		}
	}

	if err := utilities.ApplyWorkspaceEdit(edit); err != nil {
		return "", err
	}

	for _, filePath := range files {
		if err := client.ReopenFile(ctx, filePath); err != nil {
			return "", fmt.Errorf("applied changes but failed to resync %s: %w", filePath, err)
		}
	}

	message := fmt.Sprintf("Applied %d text changes across %d files.", changeCount, len(response.Changes))
	if response.ResultingCursorPosition != nil {
		message += fmt.Sprintf(" Cursor: L%d:C%d.", response.ResultingCursorPosition.Line, response.ResultingCursorPosition.Offset)
	}
	return message, nil
}

func encodeURIComponent(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

func toZeroBased(value int) uint32 {
	if value <= 1 {
		return 0
	}
	return uint32(value - 1)
}

func severityLabel(severity int) string {
	switch severity {
	case 0, 1:
		return "ERROR"
	case 2:
		return "WARNING"
	case 3:
		return "INFO"
	case 4:
		return "HINT"
	default:
		return fmt.Sprintf("SEVERITY(%d)", severity)
	}
}

func formatJSON(value any) (string, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSpace(buffer.String()), nil
}
