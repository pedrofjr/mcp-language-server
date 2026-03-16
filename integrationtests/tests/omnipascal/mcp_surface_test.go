package omnipascal_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestOmniPascalMCPSurfaceTools(t *testing.T) {
	mcpBinary := strings.TrimSpace(os.Getenv("OMNIPASCAL_MCP_BINARY"))
	if mcpBinary == "" {
		mcpBinary = filepath.Join(".", "mcp-language-server.exe")
	}
	if _, err := os.Stat(mcpBinary); err != nil {
		t.Skipf("mcp binary not found: %s", mcpBinary)
	}

	omniServer := strings.TrimSpace(os.Getenv("OMNIPASCAL_SERVER"))
	if omniServer == "" {
		t.Skip("set OMNIPASCAL_SERVER to run MCP public surface tests")
	}

	workspace := strings.TrimSpace(os.Getenv("OMNIPASCAL_WORKSPACE"))
	if workspace == "" {
		workspace = `C:\Users\pedro.ailton\Downloads\mcp-language-server\SelecaoTimCFOPLote`
	}

	args := []string{
		"--workspace", workspace,
		"--backend", "omnipascal",
		"--lsp", omniServer,
	}
	if delphi := strings.TrimSpace(os.Getenv("OMNIPASCAL_DELPHI_PATH")); delphi != "" {
		args = append(args, "--omnipascal-delphi-installation-path", delphi)
	}
	if search := strings.TrimSpace(os.Getenv("OMNIPASCAL_SEARCH_PATH")); search != "" {
		args = append(args, "--omnipascal-search-path", search)
	}

	cmdDir := filepath.Dir(mcpBinary)
	client, err := newMCPStdioClient(cmdDir, mcpBinary, args...)
	allowGoRunFallback := strings.EqualFold(strings.TrimSpace(os.Getenv("OMNIPASCAL_ALLOW_GO_RUN_FALLBACK")), "1")
	if err != nil && isPermissionLikeError(err) && allowGoRunFallback {
		t.Logf("mcp binary execution denied, trying go run fallback: %v", err)
		fallbackArgs := append([]string{"run", "."}, args...)
		client, err = newMCPStdioClient(cmdDir, "go", fallbackArgs...)
	}
	if err != nil {
		t.Fatalf("failed to start mcp stdio client: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := client.request(ctx, 1, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"clientInfo": map[string]any{
			"name":    "omnipascal-mcp-surface-test",
			"version": "1.0.0",
		},
		"capabilities": map[string]any{},
	}); err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	if err := client.notify(ctx, "notifications/initialized", nil); err != nil {
		t.Fatalf("initialized notification failed: %v", err)
	}

	listResult, err := client.request(ctx, 2, "tools/list", map[string]any{})
	if err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}

	toolNames, err := extractToolNames(listResult)
	if err != nil {
		t.Fatalf("failed to decode tools/list result: %v", err)
	}

	expected := []string{
		"edit_file",
		"omnipascal_add_uses",
		"omnipascal_change",
		"omnipascal_close",
		"omnipascal_completions",
		"omnipascal_definition",
		"omnipascal_get_all_units",
		"omnipascal_get_code_actions",
		"omnipascal_get_possible_uses_sections",
		"omnipascal_geterr",
		"omnipascal_load_project",
		"omnipascal_open",
		"omnipascal_quickinfo",
		"omnipascal_run_code_action",
		"omnipascal_signature_help",
	}

	missing := make([]string, 0)
	for _, name := range expected {
		if _, ok := toolNames[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("tools/list missing expected tools: %v", missing)
	}

	filePath := strings.TrimSpace(os.Getenv("OMNIPASCAL_TARGET_FILE"))
	if filePath == "" {
		filePath = filepath.Join(workspace, "Unit1.pas")
	}

	callResult, err := client.request(ctx, 3, "tools/call", map[string]any{
		"name": "omnipascal_get_possible_uses_sections",
		"arguments": map[string]any{
			"filePath": filePath,
		},
	})
	if err != nil {
		t.Fatalf("tools/call omnipascal_get_possible_uses_sections failed: %v", err)
	}

	callText, err := extractCallText(callResult)
	if err != nil {
		t.Fatalf("failed to parse tools/call result: %v", err)
	}
	if !strings.Contains(callText, "interface") {
		t.Fatalf("unexpected uses sections output: %s", callText)
	}
}

type mcpStdioClient struct {
	cmd    *exec.Cmd
	stdin  *bufio.Writer
	stdout *bufio.Scanner
}

func newMCPStdioClient(workDir, command string, args ...string) (*mcpStdioClient, error) {
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	if workDir != "" {
		cmd.Dir = workDir
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &mcpStdioClient{
		cmd:    cmd,
		stdin:  bufio.NewWriter(stdinPipe),
		stdout: bufio.NewScanner(stdoutPipe),
	}, nil
}

func isPermissionLikeError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, os.ErrPermission) {
		return true
	}

	message := strings.ToLower(err.Error())
	if strings.Contains(message, "acesso negado") ||
		strings.Contains(message, "access is denied") ||
		strings.Contains(message, "permission denied") ||
		strings.Contains(message, "operation not permitted") ||
		strings.Contains(message, "eperm") {
		return true
	}

	if runtime.GOOS == "windows" && strings.Contains(message, "fork/exec") {
		return strings.Contains(message, ".exe")
	}

	return false
}

func (c *mcpStdioClient) Close() error {
	if c == nil || c.cmd == nil {
		return nil
	}
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	_, _ = c.cmd.Process.Wait()
	return nil
}

func (c *mcpStdioClient) notify(ctx context.Context, method string, params map[string]any) error {
	message := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		message["params"] = params
	}
	return c.send(ctx, message)
}

func (c *mcpStdioClient) request(ctx context.Context, id int, method string, params map[string]any) (map[string]any, error) {
	message := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		message["params"] = params
	}

	if err := c.send(ctx, message); err != nil {
		return nil, err
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !c.stdout.Scan() {
			return nil, fmt.Errorf("mcp process closed stdout")
		}

		line := strings.TrimSpace(c.stdout.Text())
		if line == "" {
			continue
		}

		var envelope map[string]any
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			continue
		}

		if _, isNotification := envelope["method"]; isNotification {
			continue
		}

		msgID, ok := envelope["id"].(float64)
		if !ok || int(msgID) != id {
			continue
		}

		if rpcErr, exists := envelope["error"]; exists {
			return nil, fmt.Errorf("mcp request %s failed: %v", method, rpcErr)
		}

		return envelope, nil
	}
}

func (c *mcpStdioClient) send(ctx context.Context, message map[string]any) error {
	_ = ctx
	encoded, err := json.Marshal(message)
	if err != nil {
		return err
	}

	if _, err := c.stdin.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return c.stdin.Flush()
}

func extractToolNames(response map[string]any) (map[string]struct{}, error) {
	result, ok := response["result"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing result")
	}

	toolsList, ok := result["tools"].([]any)
	if !ok {
		return nil, fmt.Errorf("missing tools list")
	}

	names := make(map[string]struct{}, len(toolsList))
	for _, entry := range toolsList {
		toolMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		name, ok := toolMap["name"].(string)
		if !ok || name == "" {
			continue
		}
		names[name] = struct{}{}
	}

	return names, nil
}

func extractCallText(response map[string]any) (string, error) {
	result, ok := response["result"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("missing result")
	}

	content, ok := result["content"].([]any)
	if !ok {
		return "", fmt.Errorf("missing content")
	}

	parts := make([]string, 0, len(content))
	for _, entry := range content {
		item, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		text, ok := item["text"].(string)
		if ok && text != "" {
			parts = append(parts, text)
		}
	}

	if len(parts) == 0 {
		return "", fmt.Errorf("no text content returned")
	}

	return strings.Join(parts, "\n"), nil
}
