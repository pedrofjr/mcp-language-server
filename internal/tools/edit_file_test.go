package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

const editFileFakeLSPEnv = "MCP_FAKE_LSP_EDIT_FILE"

func TestHelperProcessEditFileFakeLSP(t *testing.T) {
	if os.Getenv(editFileFakeLSPEnv) != "1" {
		return
	}

	runEditFileFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestApplyTextEdits_ConvertsFilesystemPathToFileURIBeforeWorkspaceEdit(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "main.go")
	initialContent := "line one\nline two\n"
	if err := os.WriteFile(filePath, []byte(initialContent), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	t.Setenv(editFileFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessEditFileFakeLSP")
	if err != nil {
		t.Fatalf("failed to start fake LSP: %v", err)
	}
	defer func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		t.Fatalf("failed to initialize fake LSP client: %v", err)
	}

	_, err = ApplyTextEdits(ctx, client, filePath, []TextEdit{
		{
			StartLine: 1,
			EndLine:   1,
			NewText:   "updated line",
		},
	})
	if err != nil {
		t.Fatalf("expected ApplyTextEdits to accept filesystem path and convert to file URI, got error: %v", err)
	}

	updatedContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}

	expectedContent := "updated line\nline two\n"
	if string(updatedContent) != expectedContent {
		t.Fatalf("unexpected file content after edit; expected %q, got %q", expectedContent, string(updatedContent))
	}
}

func TestApplyTextEdits_AcceptsFileURIInput(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "main.go")
	initialContent := "line one\nline two\n"
	if err := os.WriteFile(filePath, []byte(initialContent), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	t.Setenv(editFileFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessEditFileFakeLSP")
	if err != nil {
		t.Fatalf("failed to start fake LSP: %v", err)
	}
	defer func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		t.Fatalf("failed to initialize fake LSP client: %v", err)
	}

	fileURI := string(protocol.URIFromPath(filePath))
	_, err = ApplyTextEdits(ctx, client, fileURI, []TextEdit{
		{
			StartLine: 1,
			EndLine:   1,
			NewText:   "updated line",
		},
	})
	if err != nil {
		t.Fatalf("expected ApplyTextEdits to accept file URI input, got error: %v", err)
	}

	updatedContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}

	expectedContent := "updated line\nline two\n"
	if string(updatedContent) != expectedContent {
		t.Fatalf("unexpected file content after URI edit; expected %q, got %q", expectedContent, string(updatedContent))
	}
}

func TestApplyTextEdits_ExternalAbsolutePathOutsideWorkspace_PersistsContentToDisk(t *testing.T) {
	workspaceDir := t.TempDir()
	externalDir := t.TempDir()
	filePath := filepath.Join(externalDir, "outside_workspace.go")
	initialContent := "line one\nline two\n"
	if err := os.WriteFile(filePath, []byte(initialContent), 0o644); err != nil {
		t.Fatalf("failed to write external fixture file: %v", err)
	}

	t.Setenv(editFileFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessEditFileFakeLSP")
	if err != nil {
		t.Fatalf("failed to start fake LSP: %v", err)
	}
	defer func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		t.Fatalf("failed to initialize fake LSP client: %v", err)
	}

	result, err := ApplyTextEdits(ctx, client, filePath, []TextEdit{{
		StartLine: 1,
		EndLine:   1,
		NewText:   "updated outside workspace",
	}})
	if err != nil {
		t.Fatalf("expected ApplyTextEdits to succeed for absolute path outside workspace, got error: %v", err)
	}

	if result == "" {
		t.Fatalf("expected non-empty success result message for external edit")
	}

	updatedContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read external file after edit: %v", err)
	}

	expectedContent := "updated outside workspace\nline two\n"
	if string(updatedContent) != expectedContent {
		t.Fatalf("expected external absolute path edit to persist on disk; expected %q, got %q", expectedContent, string(updatedContent))
	}
}

func TestApplyTextEdits_SingleLineEdit_PersistsContentToDisk(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "single_line.txt")
	originalContent := "original value"
	if err := os.WriteFile(filePath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("failed to write single-line fixture file: %v", err)
	}

	t.Setenv(editFileFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessEditFileFakeLSP")
	if err != nil {
		t.Fatalf("failed to start fake LSP: %v", err)
	}
	defer func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		t.Fatalf("failed to initialize fake LSP client: %v", err)
	}

	_, err = ApplyTextEdits(ctx, client, filePath, []TextEdit{{
		StartLine: 1,
		EndLine:   1,
		NewText:   "updated value",
	}})
	if err != nil {
		t.Fatalf("expected single-line edit to succeed and persist on disk, got error: %v", err)
	}

	updatedContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read single-line file after edit: %v", err)
	}

	if string(updatedContent) != "updated value" {
		t.Fatalf("expected single-line edit to persist updated disk content; got %q", string(updatedContent))
	}
}

func TestApplyTextEdits_InvalidRangeReturnsErrorAndKeepsOriginalContent(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "main.go")
	originalContent := "line one\nline two\n"
	if err := os.WriteFile(filePath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	t.Setenv(editFileFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessEditFileFakeLSP")
	if err != nil {
		t.Fatalf("failed to start fake LSP: %v", err)
	}
	defer func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		t.Fatalf("failed to initialize fake LSP client: %v", err)
	}

	_, err = ApplyTextEdits(ctx, client, filePath, []TextEdit{
		{
			StartLine: 100000,
			EndLine:   100000,
			NewText:   "unexpected mutation",
		},
	})

	contentAfterEdit, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("failed to read file after applying invalid range edit: %v", readErr)
	}

	if string(contentAfterEdit) != originalContent {
		t.Fatalf("expected original content to remain unchanged for invalid range; expected %q, got %q", originalContent, string(contentAfterEdit))
	}

	if err == nil {
		t.Fatalf("expected ApplyTextEdits to return an error for out-of-file range, got nil")
	}
}

func TestApplyTextEdits_CRLFMultilineReplacementInPathWithSpaces_DoesNotInsertBlankLine(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "unit with space.pas")
	originalContent := "unit Sample;\r\nbegin\r\nend.\r\n"
	if err := os.WriteFile(filePath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("failed to write CRLF fixture file: %v", err)
	}

	t.Setenv(editFileFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessEditFileFakeLSP")
	if err != nil {
		t.Fatalf("failed to start fake LSP: %v", err)
	}
	defer func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		t.Fatalf("failed to initialize fake LSP client: %v", err)
	}

	_, err = ApplyTextEdits(ctx, client, filePath, []TextEdit{{
		StartLine: 2,
		EndLine:   2,
		NewText:   "begin\r\n  Added;\r\n",
	}})
	if err != nil {
		t.Fatalf("expected ApplyTextEdits to accept CRLF multiline replacement for path with spaces, got error: %v", err)
	}

	updatedContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read updated CRLF fixture file: %v", err)
	}

	want := "unit Sample;\r\nbegin\r\n  Added;\r\nend.\r\n"
	if string(updatedContent) != want {
		t.Fatalf("expected CRLF multiline replacement in path with spaces to avoid blank line and preserve footer; got %q, want %q", string(updatedContent), want)
	}
}

func TestApplyTextEdits_CP1252NoBOMLineSplit_PreservesWindows1252BytesAndInsertsBananaLine(t *testing.T) {
	workspaceDir := t.TempDir()
	filePath := filepath.Join(workspaceDir, "sample.pas")

	var fixture bytes.Buffer
	for lineNumber := 1; lineNumber <= 60; lineNumber++ {
		if lineNumber == 55 {
			fixture.Write([]byte{'/', '/', ' ', 'T', 'e', 's', 't', 'e', ' ', 0xe7, ' ', '~', ' ', 0xe3, ' ', 0xf5, '\n'})
			continue
		}

		fmt.Fprintf(&fixture, "line %02d\n", lineNumber)
	}

	if err := os.WriteFile(filePath, fixture.Bytes(), 0o644); err != nil {
		t.Fatalf("failed to write cp1252 fixture file: %v", err)
	}

	t.Setenv(editFileFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessEditFileFakeLSP")
	if err != nil {
		t.Fatalf("failed to start fake LSP: %v", err)
	}
	defer func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		t.Fatalf("failed to initialize fake LSP client: %v", err)
	}

	_, err = ApplyTextEdits(ctx, client, filePath, []TextEdit{{
		StartLine: 55,
		EndLine:   55,
		NewText:   "// Teste \u00e7 ~ \u00e3 \u00f5\n//banana",
	}})
	if err != nil {
		t.Fatalf("expected ApplyTextEdits to split cp1252 line without changing ansi bytes, got error: %v", err)
	}

	updatedContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read updated cp1252 fixture file: %v", err)
	}

	lines := bytes.Split(updatedContent, []byte("\n"))
	if len(lines) < 56 {
		t.Fatalf("expected edited cp1252 fixture to contain at least 56 lines, got %d", len(lines))
	}

	expectedLine55 := []byte{'/', '/', ' ', 'T', 'e', 's', 't', 'e', ' ', 0xe7, ' ', '~', ' ', 0xe3, ' ', 0xf5}
	if !bytes.Equal(lines[54], expectedLine55) {
		t.Fatalf("expected line 55 to preserve windows-1252 bytes % x, got % x", expectedLine55, lines[54])
	}

	if !bytes.Equal(lines[55], []byte("//banana")) {
		t.Fatalf("expected inserted line 56 to be //banana, got %q", lines[55])
	}
	if !bytes.Equal(lines[56], []byte("line 56")) {
		t.Fatalf("expected original content after edited cp1252 line to shift to line 57, got %q", lines[56])
	}
}

func TestDelphiEditFileMultilineReportsSuccessButDoesNotPersistWhenFileIsClosed(t *testing.T) {
	workspaceDir := t.TempDir()
	spacedDir := filepath.Join(workspaceDir, "samples with space")
	if err := os.MkdirAll(spacedDir, 0o755); err != nil {
		t.Fatalf("failed to create fixture directory with spaces: %v", err)
	}

	filePath := filepath.Join(spacedDir, "sample - long loop.pas")
	originalContent := strings.Join([]string{
		"unit Sample;",
		"begin",
		"  i := 0;",
		"  while i < 3 do",
		"  begin",
		"    Inc(i);",
		"  end;",
		"end.",
		"",
	}, "\r\n")
	if err := os.WriteFile(filePath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("failed to write multiline fixture file: %v", err)
	}

	t.Setenv(editFileFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessEditFileFakeLSP")
	if err != nil {
		t.Fatalf("failed to start fake LSP: %v", err)
	}
	defer func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		t.Fatalf("failed to initialize fake LSP client: %v", err)
	}

	replacement := strings.Join([]string{
		"  i := 1;",
		"  while i < 5 do",
		"  begin",
		"    Inc(i, 2);",
		"",
	}, "\r\n")

	result, err := ApplyTextEdits(ctx, client, filePath, []TextEdit{{
		StartLine: 3,
		EndLine:   5,
		NewText:   replacement,
	}})

	updatedContent, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("failed to read multiline fixture file after edit attempt: %v", readErr)
	}

	if err != nil {
		if string(updatedContent) != originalContent {
			t.Fatalf("expected file content to remain unchanged when edit returns error; got %q", string(updatedContent))
		}
		return
	}

	if !strings.Contains(result, "Successfully applied text edits") {
		t.Fatalf("expected success result message to indicate applied text edits, got %q", result)
	}

	expectedContent := strings.Join([]string{
		"unit Sample;",
		"begin",
		"  i := 1;",
		"  while i < 5 do",
		"  begin",
		"    Inc(i, 2);",
		"    Inc(i);",
		"  end;",
		"end.",
		"",
	}, "\r\n")

	if string(updatedContent) != expectedContent {
		t.Fatalf("edit reported success but persisted content did not match expected output; expected %q, got %q", expectedContent, string(updatedContent))
	}
}

func TestDelphiEditFile_LongLoopRange7To9_DoesNotLeaveResidualResultLineOutsideLoop(t *testing.T) {
	workspaceDir := t.TempDir()
	spacedDir := filepath.Join(workspaceDir, "samples with space")
	if err := os.MkdirAll(spacedDir, 0o755); err != nil {
		t.Fatalf("failed to create fixture directory with spaces: %v", err)
	}

	filePath := filepath.Join(spacedDir, "sample - long loop.pas")
	originalContent := strings.Join([]string{
		"unit SampleLongLoop;",
		"",
		"procedure RunLongLoop;",
		"var",
		"  i: Integer;",
		"begin",
		"  for i := 1 to 1000 do",
		"  // result update",
		"  Result := square(i);",
		"  end;",
		"  Result := square(i);",
		"end;",
		"",
	}, "\r\n")
	if err := os.WriteFile(filePath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("failed to write long loop fixture file: %v", err)
	}

	t.Setenv(editFileFakeLSPEnv, "1")

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessEditFileFakeLSP")
	if err != nil {
		t.Fatalf("failed to start fake LSP: %v", err)
	}
	defer func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		t.Fatalf("failed to initialize fake LSP client: %v", err)
	}

	_, err = ApplyTextEdits(ctx, client, filePath, []TextEdit{{
		StartLine: 7,
		EndLine:   9,
		NewText: strings.Join([]string{
			"  for i := 1 to 1000 do",
			"    Result := square(i);",
		}, "\n"),
	}})
	if err != nil {
		t.Fatalf("expected multiline edit 7..9 to succeed, got error: %v", err)
	}

	updatedContentBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read long loop fixture after edit: %v", err)
	}

	updatedContent := string(updatedContentBytes)
	expectedFragment := strings.Join([]string{
		"  for i := 1 to 1000 do",
		"    Result := square(i);",
		"  end;",
	}, "\r\n")
	if !strings.Contains(updatedContent, expectedFragment) {
		t.Fatalf("expected edited long-loop block to keep for/result/end sequence; expected fragment %q in content %q", expectedFragment, updatedContent)
	}

	if strings.Count(updatedContent, "  Result := square(i);") != 1 {
		t.Fatalf("expected no residual duplicated 'Result := square(i);' line outside edited loop; got content %q", updatedContent)
	}
}

func runEditFileFakeLSP(stdin *os.File, stdout *os.File) {
	reader := bufio.NewReader(stdin)
	writer := stdout

	for {
		msg, err := lsp.ReadMessage(reader)
		if err != nil {
			return
		}

		switch msg.Method {
		case "initialize":
			result := map[string]any{
				"capabilities": map[string]any{
					"textDocumentSync": 1,
				},
			}
			sendEditFileFakeResponse(writer, msg.ID, result, nil)
		case "initialized", "textDocument/didOpen":
			// No-op.
		case "shutdown":
			sendEditFileFakeResponse(writer, msg.ID, nil, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendEditFileFakeResponse(writer, msg.ID, nil, nil)
			}
		}
	}
}

func sendEditFileFakeResponse(w *os.File, id *lsp.MessageID, result any, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
	}

	if rpcErr == nil {
		if result == nil {
			resp.Result = mustMarshalEditFileFake(struct{}{})
		} else {
			resp.Result = mustMarshalEditFileFake(result)
		}
	}

	_ = lsp.WriteMessage(w, resp)
}

func mustMarshalEditFileFake(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal fake LSP json: %v", err))
	}
	return b
}
