package utilities

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/davecgh/go-spew/spew"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"golang.org/x/text/encoding/charmap"
)

var (
	osReadFile  = os.ReadFile
	osWriteFile = os.WriteFile
	osStat      = os.Stat
	osRemove    = os.Remove
	osRemoveAll = os.RemoveAll
	osRename    = os.Rename
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

type textFileEncoding int

const (
	textFileEncodingUTF8 textFileEncoding = iota
	textFileEncodingUTF8BOM
	textFileEncodingWindows1252
)

func documentURIToPath(uri protocol.DocumentUri) (string, error) {
	rawURI := string(uri)
	if rawURI == "" {
		return "", fmt.Errorf("invalid URI: empty URI")
	}

	parsedURI, err := protocol.ParseDocumentUri(rawURI)
	if err != nil {
		return "", fmt.Errorf("invalid URI %q: %w", rawURI, err)
	}

	path, pathErr := safeDocumentURIPath(parsedURI)
	if pathErr != nil {
		return "", fmt.Errorf("invalid URI %q: %w", rawURI, pathErr)
	}

	if path == "" {
		return "", fmt.Errorf("invalid URI %q: empty file path", rawURI)
	}

	normalizedPath := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		normalizedPath = filepath.ToSlash(normalizedPath)
	}

	return normalizedPath, nil
}

func safeDocumentURIPath(uri protocol.DocumentUri) (path string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("failed to convert document URI to path: %v", recovered)
		}
	}()

	return uri.Path(), nil
}

// ApplyTextEdits applies a sequence of text edits to a file specified by URI
func ApplyTextEdits(uri protocol.DocumentUri, edits []protocol.TextEdit) error {
	path, err := documentURIToPath(uri)
	if err != nil {
		return err
	}

	// Read the file content
	content, err := osReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	fileEncoding, bom, body := classifyTextFileEncoding(content)

	// Detect line ending style
	var lineEnding string
	if bytes.Contains(body, []byte("\r\n")) {
		lineEnding = "\r\n"
	} else {
		lineEnding = "\n"
	}

	// Track if file ends with a newline
	endsWithNewline := len(body) > 0 && bytes.HasSuffix(body, []byte(lineEnding))

	// Split into lines without the endings
	lines := strings.Split(string(body), lineEnding)

	// Check for overlapping edits
	for i, edit1 := range edits {
		for j := i + 1; j < len(edits); j++ {
			if RangesOverlap(edit1.Range, edits[j].Range) {
				return fmt.Errorf("overlapping edits detected between edit %d and %d", i, j)
			}
		}
	}

	// Sort edits in reverse order
	sortedEdits := make([]protocol.TextEdit, len(edits))
	copy(sortedEdits, edits)
	sort.Slice(sortedEdits, func(i, j int) bool {
		if sortedEdits[i].Range.Start.Line != sortedEdits[j].Range.Start.Line {
			return sortedEdits[i].Range.Start.Line > sortedEdits[j].Range.Start.Line
		}
		return sortedEdits[i].Range.Start.Character > sortedEdits[j].Range.Start.Character
	})

	// Apply each edit
	for _, edit := range sortedEdits {
		rawNewText, err := encodeTextForFile(edit.NewText, fileEncoding)
		if err != nil {
			return fmt.Errorf("failed to encode new text for %s: %w", path, err)
		}

		rawEdit := edit
		rawEdit.NewText = rawNewText

		newLines, err := ApplyTextEdit(lines, rawEdit, lineEnding)
		if err != nil {
			return fmt.Errorf("failed to apply edit: %w", err)
		}
		lines = newLines
	}

	// Join lines with proper line endings
	var newContent strings.Builder
	for i, line := range lines {
		if i > 0 {
			newContent.WriteString(lineEnding)
		}
		newContent.WriteString(line)
	}

	// Only add a newline if the original file had one and we haven't already added it
	if endsWithNewline && !strings.HasSuffix(newContent.String(), lineEnding) {
		newContent.WriteString(lineEnding)
	}

	updatedContent := make([]byte, 0, len(bom)+newContent.Len())
	updatedContent = append(updatedContent, bom...)
	updatedContent = append(updatedContent, newContent.String()...)

	if err := osWriteFile(path, updatedContent, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	persistedContent, err := osReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to verify persisted file content: %w", err)
	}

	if !bytes.Equal(persistedContent, updatedContent) {
		return fmt.Errorf("file write reported success but persisted content mismatch for %s", path)
	}

	return nil
}

func classifyTextFileEncoding(content []byte) (textFileEncoding, []byte, []byte) {
	if bytes.HasPrefix(content, utf8BOM) {
		return textFileEncodingUTF8BOM, utf8BOM, content[len(utf8BOM):]
	}

	if utf8.Valid(content) {
		return textFileEncodingUTF8, nil, content
	}

	return textFileEncodingWindows1252, nil, content
}

func encodeTextForFile(newText string, encoding textFileEncoding) (string, error) {
	if encoding != textFileEncodingWindows1252 {
		return newText, nil
	}

	encodedText, err := charmap.Windows1252.NewEncoder().String(newText)
	if err != nil {
		return "", fmt.Errorf("text is not representable in Windows-1252: %w", err)
	}

	return encodedText, nil
}

// ApplyTextEdit applies a single text edit to a set of lines
func ApplyTextEdit(lines []string, edit protocol.TextEdit, lineEnding string) ([]string, error) {
	startLine := int(edit.Range.Start.Line)
	endLine := int(edit.Range.End.Line)
	startChar := int(edit.Range.Start.Character)
	endChar := int(edit.Range.End.Character)

	// Validate positions
	if startLine < 0 || startLine >= len(lines) {
		return nil, fmt.Errorf("invalid start line: %d", startLine)
	}
	if endLine < 0 || endLine >= len(lines) {
		endLine = len(lines) - 1
	}

	// Create result slice with initial capacity
	result := make([]string, 0, len(lines))

	// Copy lines before edit
	result = append(result, lines[:startLine]...)

	// Get the prefix of the start line
	startLineContent := lines[startLine]
	if startChar < 0 || startChar > len(startLineContent) {
		startChar = len(startLineContent)
	}
	prefix := startLineContent[:startChar]

	// Get the suffix of the end line
	endLineContent := lines[endLine]
	if endChar < 0 || endChar > len(endLineContent) {
		endChar = len(endLineContent)
	}
	suffix := endLineContent[endChar:]

	// Handle the edit
	if edit.NewText == "" {
		if prefix+suffix != "" {
			result = append(result, prefix+suffix)
		}
	} else {
		newLines, endsWithLineBreak := splitEditNewText(edit.NewText)
		needsTrailingEmptyLineAtEOF := endsWithLineBreak && suffix == "" && endLine+1 >= len(lines)

		if len(newLines) == 1 && !endsWithLineBreak {
			result = append(result, prefix+newLines[0]+suffix)
		} else {
			result = append(result, prefix+newLines[0])

			if len(newLines) > 2 {
				result = append(result, newLines[1:len(newLines)-1]...)
			}

			if len(newLines) > 1 {
				lastLine := newLines[len(newLines)-1]
				if endsWithLineBreak {
					result = append(result, lastLine)
					if suffix != "" {
						result = append(result, suffix)
					}
				} else {
					result = append(result, lastLine+suffix)
				}
			} else if suffix != "" {
				result = append(result, suffix)
			}
		}

		if needsTrailingEmptyLineAtEOF {
			result = append(result, "")
		}
	}

	// Add remaining lines
	if endLine+1 < len(lines) {
		result = append(result, lines[endLine+1:]...)
	}

	return result, nil
}

func splitEditNewText(newText string) ([]string, bool) {
	lines := make([]string, 0, strings.Count(newText, "\n")+1)
	start := 0
	endsWithLineBreak := false

	for i := 0; i < len(newText); i++ {
		switch newText[i] {
		case '\r':
			if i+1 < len(newText) && newText[i+1] == '\n' {
				continue
			}
			lines = append(lines, newText[start:i])
			start = i + 1
			endsWithLineBreak = true
		case '\n':
			lineEnd := i
			if i > start && newText[i-1] == '\r' {
				lineEnd = i - 1
			}
			lines = append(lines, newText[start:lineEnd])
			start = i + 1
			endsWithLineBreak = true
		default:
			endsWithLineBreak = false
		}
	}

	if start < len(newText) || !endsWithLineBreak {
		lines = append(lines, newText[start:])
	}

	return lines, endsWithLineBreak
}

// ApplyDocumentChange applies a DocumentChange (create/rename/delete operations)
func ApplyDocumentChange(change protocol.DocumentChange) error {
	if change.CreateFile != nil {
		path, err := documentURIToPath(change.CreateFile.URI)
		if err != nil {
			return err
		}
		if change.CreateFile.Options != nil {
			if change.CreateFile.Options.Overwrite {
				// Proceed with overwrite
			} else if change.CreateFile.Options.IgnoreIfExists {
				if _, err := osStat(path); err == nil {
					return nil // File exists and we're ignoring it
				}
			}
		}
		if err := osWriteFile(path, []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
	}

	if change.DeleteFile != nil {
		path, err := documentURIToPath(change.DeleteFile.URI)
		if err != nil {
			return err
		}
		if change.DeleteFile.Options != nil && change.DeleteFile.Options.Recursive {
			if err := osRemoveAll(path); err != nil {
				return fmt.Errorf("failed to delete directory recursively: %w", err)
			}
		} else {
			if err := osRemove(path); err != nil {
				return fmt.Errorf("failed to delete file: %w", err)
			}
		}
	}

	if change.RenameFile != nil {
		oldPath, err := documentURIToPath(change.RenameFile.OldURI)
		if err != nil {
			return err
		}

		newPath, err := documentURIToPath(change.RenameFile.NewURI)
		if err != nil {
			return err
		}
		if change.RenameFile.Options != nil {
			if !change.RenameFile.Options.Overwrite {
				if _, err := osStat(newPath); err == nil {
					return fmt.Errorf("target file already exists and overwrite is not allowed: %s", newPath)
				}
			}
		}
		if err := osRename(oldPath, newPath); err != nil {
			return fmt.Errorf("failed to rename file: %w", err)
		}
	}

	if change.TextDocumentEdit != nil {
		textEdits := make([]protocol.TextEdit, len(change.TextDocumentEdit.Edits))
		for i, edit := range change.TextDocumentEdit.Edits {
			var err error
			textEdits[i], err = edit.AsTextEdit()
			if err != nil {
				return fmt.Errorf("invalid edit type: %w", err)
			}
		}
		return ApplyTextEdits(change.TextDocumentEdit.TextDocument.URI, textEdits)
	}

	return nil
}

// ApplyWorkspaceEdit applies the given WorkspaceEdit to the filesystem
func ApplyWorkspaceEdit(edit protocol.WorkspaceEdit) error {
	// Handle Changes field
	for uri, textEdits := range edit.Changes {
		if err := ApplyTextEdits(uri, textEdits); err != nil {
			return fmt.Errorf("failed to apply text edits: %w", err)
		}
	}

	// Handle DocumentChanges field
	for _, change := range edit.DocumentChanges {
		coreLogger.Warn("Document change: %v", spew.Sdump(change))
		if err := ApplyDocumentChange(change); err != nil {
			return fmt.Errorf("failed to apply document change: %w", err)
		}
	}

	return nil
}

// RangesOverlap checks if two ranges overlap in position
func RangesOverlap(r1, r2 protocol.Range) bool {
	if r1.Start.Line > r2.End.Line || r2.Start.Line > r1.End.Line {
		return false
	}
	if r1.Start.Line == r2.End.Line && r1.Start.Character > r2.End.Character {
		return false
	}
	if r2.Start.Line == r1.End.Line && r2.Start.Character > r1.End.Character {
		return false
	}
	return true
}
