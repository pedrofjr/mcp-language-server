package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/utilities"
)

type TextEdit struct {
	StartLine int    `json:"startLine" jsonschema:"required,description=Start line to replace, inclusive"`
	EndLine   int    `json:"endLine" jsonschema:"required,description=End line to replace, inclusive"`
	NewText   string `json:"newText" jsonschema:"description=Replacement text. Replace with the new text. Leave blank to remove lines."`
}

func ApplyTextEdits(ctx context.Context, client *lsp.Client, filePath string, edits []TextEdit) (string, error) {
	normalizedPath, err := normalizeFilePathOrURI(filePath)
	if err != nil {
		return "", fmt.Errorf("invalid file path or URI: %v", err)
	}

	// Create a sorted copy of edits for reporting
	sortedEdits := make([]TextEdit, len(edits))
	copy(sortedEdits, edits)
	sort.Slice(sortedEdits, func(i, j int) bool {
		return sortedEdits[i].StartLine < sortedEdits[j].StartLine
	})

	// Track lines added and removed for sorted edits
	linesRemovedSorted := 0
	linesAddedSorted := 0
	for _, sortedEdit := range sortedEdits {
		// Calculate lines removed: end - start + 1
		removedLineCount := sortedEdit.EndLine - sortedEdit.StartLine + 1
		linesRemovedSorted += removedLineCount

		// Calculate lines added: count newlines in the replacement text + 1
		addedLineCount := 1
		if sortedEdit.NewText != "" {
			addedLineCount = strings.Count(sortedEdit.NewText, "\n") + 1
		} else if sortedEdit.NewText == "" {
			addedLineCount = 0
		}
		linesAddedSorted += addedLineCount
	}

	// Sort edits by line number in descending order to process from bottom to top
	// This way line numbers don't shift under us as we make edits
	orderedEdits := make([]TextEdit, len(edits))
	copy(orderedEdits, edits)
	sort.Slice(orderedEdits, func(i, j int) bool {
		return orderedEdits[i].StartLine > orderedEdits[j].StartLine
	})

	// Validate and convert all ranges before opening/changing the file.
	textEdits := make([]protocol.TextEdit, 0, len(orderedEdits))
	for _, requestedEdit := range orderedEdits {
		adjustedEdit, err := adjustEditForResidualDuplicate(normalizedPath, requestedEdit)
		if err != nil {
			return "", fmt.Errorf("invalid position: %v", err)
		}

		// Get the range covering the requested lines
		rng, err := getRange(adjustedEdit.StartLine, adjustedEdit.EndLine, normalizedPath)
		if err != nil {
			return "", fmt.Errorf("invalid position: %v", err)
		}

		// Always do a replacement
		textEdits = append(textEdits, protocol.TextEdit{
			Range:   rng,
			NewText: adjustedEdit.NewText,
		})
	}

	err = client.OpenFile(ctx, normalizedPath)
	if err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	edit := protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentUri][]protocol.TextEdit{
			protocol.URIFromPath(normalizedPath): textEdits,
		},
	}

	if err := utilities.ApplyWorkspaceEdit(edit); err != nil {
		return "", fmt.Errorf("failed to apply text edits: %v", err)
	}

	if err := client.NotifyChange(ctx, normalizedPath); err != nil {
		return "", fmt.Errorf("failed to sync edited file with LSP: %v", err)
	}

	return fmt.Sprintf("Successfully applied text edits. %d lines removed, %d lines added.", linesRemovedSorted, linesAddedSorted), nil
}

func adjustEditForResidualDuplicate(filePath string, edit TextEdit) (TextEdit, error) {
	if edit.StartLine >= edit.EndLine {
		return edit, nil
	}

	replacementLines := splitLinesAnyEnding(edit.NewText)
	lastReplacementLine := lastNonEmptyLine(replacementLines)
	if strings.TrimSpace(lastReplacementLine) == "" {
		return edit, nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return TextEdit{}, fmt.Errorf("failed to read file: %w", err)
	}

	fileLines := splitLinesAnyEnding(string(content))
	nextLineIndex := edit.EndLine
	if nextLineIndex < 0 || nextLineIndex >= len(fileLines) {
		return edit, nil
	}

	if strings.TrimSpace(fileLines[nextLineIndex]) == strings.TrimSpace(lastReplacementLine) {
		adjusted := edit
		adjusted.EndLine = edit.EndLine + 1
		return adjusted, nil
	}

	blockCloserLine := strings.TrimSpace(fileLines[nextLineIndex])
	lineAfterCloserIndex := nextLineIndex + 1
	if (blockCloserLine == "end;" || blockCloserLine == "end") && lineAfterCloserIndex < len(fileLines) {
		if strings.TrimSpace(fileLines[lineAfterCloserIndex]) == strings.TrimSpace(lastReplacementLine) {
			adjusted := edit
			adjusted.EndLine = edit.EndLine + 2
			adjusted.NewText = strings.TrimRight(adjusted.NewText, "\r\n") + "\n" + fileLines[nextLineIndex]
			return adjusted, nil
		}
	}

	return edit, nil
}

func splitLinesAnyEnding(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	return strings.Split(content, "\n")
}

func lastNonEmptyLine(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return lines[i]
		}
	}

	return ""
}

// getRange creates a protocol.Range that covers the specified start and end lines
func getRange(startLine, endLine int, filePath string) (protocol.Range, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return protocol.Range{}, fmt.Errorf("failed to read file: %w", err)
	}

	// Detect line ending style
	var lineEnding string
	if bytes.Contains(content, []byte("\r\n")) {
		lineEnding = "\r\n"
	} else {
		lineEnding = "\n"
	}

	// Split lines without the line endings
	lines := strings.Split(string(content), lineEnding)

	// Handle start line positioning
	if startLine < 1 {
		return protocol.Range{}, fmt.Errorf("start line must be >= 1, got %d", startLine)
	}
	if endLine < startLine {
		return protocol.Range{}, fmt.Errorf("end line must be >= start line, got start=%d end=%d", startLine, endLine)
	}

	// Convert to 0-based line numbers
	startIdx := startLine - 1
	endIdx := endLine - 1

	if startIdx >= len(lines) {
		return protocol.Range{}, fmt.Errorf("start line %d is outside file bounds (max line %d)", startLine, len(lines))
	}

	if endIdx >= len(lines) {
		return protocol.Range{}, fmt.Errorf("end line %d is outside file bounds (max line %d)", endLine, len(lines))
	}

	// Always use the full line range for consistency
	return protocol.Range{
		Start: protocol.Position{
			Line:      uint32(startIdx),
			Character: 0, // Always start at beginning of line
		},
		End: protocol.Position{
			Line:      uint32(endIdx),
			Character: uint32(len(lines[endIdx])), // Go to end of last line
		},
	}, nil
}
