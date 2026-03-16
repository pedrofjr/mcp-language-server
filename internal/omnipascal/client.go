package omnipascal

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/logging"
)

var (
	omniLogger     = logging.NewLogger(logging.Core)
	omniWireLogger = logging.NewLogger(logging.LSPWire)
	omniProcLogger = logging.NewLogger(logging.LSPProcess)
)

type Client struct {
	Cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr io.ReadCloser

	nextSeq atomic.Int32

	handlers   map[int32]chan response
	handlersMu sync.RWMutex

	syntaxDiagnostics   map[string][]Diagnostic
	semanticDiagnostics map[string][]Diagnostic
	diagnosticsMu       sync.RWMutex

	openFiles   map[string]struct{}
	openFilesMu sync.RWMutex
}

func NewClient(command string, args ...string) (*Client, error) {
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	client := &Client{
		Cmd:                 cmd,
		stdin:               stdin,
		stdout:              bufio.NewReader(stdout),
		stderr:              stderr,
		handlers:            make(map[int32]chan response),
		syntaxDiagnostics:   make(map[string][]Diagnostic),
		semanticDiagnostics: make(map[string][]Diagnostic),
		openFiles:           make(map[string]struct{}),
	}

	if err := cmd.Start(); err != nil {
		return nil, newProcessStartError("OmniPascal server", command, err)
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			omniProcLogger.Info("%s", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			omniLogger.Error("error reading OmniPascal stderr: %v", err)
		}
	}()

	go client.handleMessages()

	return client, nil
}

func (c *Client) SynchronizeConfig(ctx context.Context, config map[string]any) error {
	return c.SetConfig(ctx, config)
}

func (c *Client) SetConfig(ctx context.Context, config map[string]any) error {
	var result json.RawMessage
	return c.Call(ctx, "setConfig", config, &result)
}

func (c *Client) Call(ctx context.Context, command string, args any, result any) error {
	seq := c.nextSeq.Add(1)
	if args == nil {
		args = map[string]any{}
	}

	msg := request{
		Seq:       seq,
		Type:      "request",
		Command:   command,
		Arguments: args,
	}

	ch := make(chan response, 1)
	c.handlersMu.Lock()
	c.handlers[seq] = ch
	c.handlersMu.Unlock()

	defer func() {
		c.handlersMu.Lock()
		delete(c.handlers, seq)
		c.handlersMu.Unlock()
	}()

	if err := c.writeRequest(msg); err != nil {
		return err
	}

	select {
	case resp := <-ch:
		if !resp.Success {
			if len(resp.Body) == 0 {
				return fmt.Errorf("omnipascal command %q failed", command)
			}
			return fmt.Errorf("omnipascal command %q failed: %s", command, strings.TrimSpace(string(resp.Body)))
		}

		if result == nil || len(resp.Body) == 0 || string(resp.Body) == "null" {
			return nil
		}

		if raw, ok := result.(*json.RawMessage); ok {
			*raw = append((*raw)[:0], resp.Body...)
			return nil
		}

		if err := json.Unmarshal(resp.Body, result); err != nil {
			return fmt.Errorf("failed to decode omnipascal response for %q: %w", command, err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) Notify(ctx context.Context, command string, args any) error {
	seq := c.nextSeq.Add(1)
	if args == nil {
		args = map[string]any{}
	}

	return c.writeRequest(request{
		Seq:       seq,
		Type:      "request",
		Command:   command,
		Arguments: args,
	})
}

func (c *Client) OpenFile(ctx context.Context, filePath string) error {
	filePath = normalizePath(filePath)

	c.openFilesMu.RLock()
	_, exists := c.openFiles[filePath]
	c.openFilesMu.RUnlock()
	if exists {
		return nil
	}

	if err := c.Notify(ctx, "open", map[string]any{"file": filePath}); err != nil {
		return err
	}

	c.openFilesMu.Lock()
	c.openFiles[filePath] = struct{}{}
	c.openFilesMu.Unlock()

	return nil
}

func (c *Client) CloseFile(ctx context.Context, filePath string) error {
	filePath = normalizePath(filePath)

	c.openFilesMu.RLock()
	_, exists := c.openFiles[filePath]
	c.openFilesMu.RUnlock()
	if !exists {
		return nil
	}

	if err := c.Notify(ctx, "close", map[string]any{"file": filePath}); err != nil {
		return err
	}

	c.openFilesMu.Lock()
	delete(c.openFiles, filePath)
	c.openFilesMu.Unlock()

	// Once a file is closed, cached diagnostics are no longer authoritative
	// for future reopen/geterr cycles.
	c.InvalidateFileDiagnostics(filePath)

	return nil
}

func (c *Client) ReopenFile(ctx context.Context, filePath string) error {
	// Force subsequent geterr calls to rely on fresh server events.
	c.InvalidateFileDiagnostics(filePath)

	if err := c.CloseFile(ctx, filePath); err != nil {
		return err
	}
	return c.OpenFile(ctx, filePath)
}

func (c *Client) ChangeFile(ctx context.Context, args ChangeArgs) error {
	args.File = normalizePath(args.File)
	// A buffer change can invalidate previous diagnostics immediately.
	c.InvalidateFileDiagnostics(args.File)
	return c.Notify(ctx, "change", args)
}

func (c *Client) GetErr(ctx context.Context, files []string, delayMs int) error {
	normalized := make([]string, 0, len(files))
	for _, filePath := range files {
		normalized = append(normalized, normalizePath(filePath))
	}

	return c.Notify(ctx, "geterr", map[string]any{
		"delay": delayMs,
		"files": normalized,
	})
}

func (c *Client) IsFileOpen(filePath string) bool {
	filePath = normalizePath(filePath)
	c.openFilesMu.RLock()
	defer c.openFilesMu.RUnlock()
	_, exists := c.openFiles[filePath]
	return exists
}

func (c *Client) GetFileDiagnostics(filePath string) []Diagnostic {
	filePath = normalizePath(filePath)

	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()

	semantic := c.semanticDiagnostics[filePath]
	syntax := c.syntaxDiagnostics[filePath]
	result := make([]Diagnostic, 0, len(syntax)+len(semantic))
	result = append(result, syntax...)
	result = append(result, semantic...)
	return result
}

func (c *Client) InvalidateFileDiagnostics(filePath string) {
	filePath = normalizePath(filePath)

	c.diagnosticsMu.Lock()
	delete(c.syntaxDiagnostics, filePath)
	delete(c.semanticDiagnostics, filePath)
	c.diagnosticsMu.Unlock()
}

func (c *Client) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	c.openFilesMu.RLock()
	openFiles := make([]string, 0, len(c.openFiles))
	for path := range c.openFiles {
		openFiles = append(openFiles, path)
	}
	c.openFilesMu.RUnlock()

	for _, filePath := range openFiles {
		if err := c.CloseFile(ctx, filePath); err != nil {
			omniLogger.Warn("failed to close OmniPascal file %s: %v", filePath, err)
		}
	}

	if err := c.stdin.Close(); err != nil {
		omniLogger.Error("failed to close OmniPascal stdin: %v", err)
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- c.Cmd.Wait()
	}()

	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	select {
	case err := <-waitDone:
		return err
	case <-timer.C:
		omniLogger.Warn("OmniPascal process did not exit within timeout, forcing kill")
		if c.Cmd.Process != nil {
			if err := c.Cmd.Process.Kill(); err != nil {
				omniLogger.Error("failed to kill OmniPascal process: %v", err)
				return err
			}
		}
		return <-waitDone
	}
}

func (c *Client) handleMessages() {
	for {
		payload, err := readMessage(c.stdout)
		if err != nil {
			if strings.Contains(err.Error(), "EOF") {
				omniLogger.Info("OmniPascal connection closed (EOF)")
			} else {
				omniLogger.Error("error reading OmniPascal message: %v", err)
			}
			return
		}

		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(payload, &envelope); err != nil {
			omniLogger.Error("failed to decode OmniPascal envelope: %v", err)
			continue
		}

		if _, ok := envelope["request_seq"]; ok {
			var resp response
			if err := json.Unmarshal(payload, &resp); err != nil {
				omniLogger.Error("failed to decode OmniPascal response: %v", err)
				continue
			}

			c.handlersMu.RLock()
			ch, exists := c.handlers[resp.RequestSeq]
			c.handlersMu.RUnlock()
			if exists {
				ch <- resp
			}
			continue
		}

		if _, ok := envelope["event"]; ok {
			var event eventMessage
			if err := json.Unmarshal(payload, &event); err != nil {
				omniLogger.Error("failed to decode OmniPascal event: %v", err)
				continue
			}

			c.handleEvent(event)
			continue
		}

		omniLogger.Debug("ignoring unknown OmniPascal message: %s", string(payload))
	}
}

func (c *Client) handleEvent(event eventMessage) {
	omniLogger.Debug("received OmniPascal event: %s", event.Event)

	switch event.Event {
	case "syntaxDiag":
		var body DiagnosticsEvent
		if err := json.Unmarshal(event.Body, &body); err != nil {
			omniLogger.Error("failed to decode syntaxDiag body: %v", err)
			return
		}
		c.diagnosticsMu.Lock()
		c.syntaxDiagnostics[normalizePath(body.File)] = body.Diagnostics
		c.diagnosticsMu.Unlock()
	case "semanticDiag":
		var body DiagnosticsEvent
		if err := json.Unmarshal(event.Body, &body); err != nil {
			omniLogger.Error("failed to decode semanticDiag body: %v", err)
			return
		}
		c.diagnosticsMu.Lock()
		c.semanticDiagnostics[normalizePath(body.File)] = body.Diagnostics
		c.diagnosticsMu.Unlock()
	default:
		omniLogger.Debug("unhandled OmniPascal event %s: %s", event.Event, string(event.Body))
	}
}

func (c *Client) writeRequest(msg request) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal OmniPascal request: %w", err)
	}

	omniLogger.Debug("sending OmniPascal request: command=%s seq=%d", msg.Command, msg.Seq)
	omniWireLogger.Debug("-> OmniPascal: %s", string(data))

	if _, err := c.stdin.Write(append(data, '\r', '\n')); err != nil {
		return fmt.Errorf("failed to write OmniPascal request: %w", err)
	}

	return nil
}

func readMessage(r *bufio.Reader) ([]byte, error) {
	var contentLength int
	headerStarted := false
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read OmniPascal header: %w", err)
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" && !headerStarted {
			continue
		}

		if trimmed == "" {
			break
		}
		headerStarted = true

		omniWireLogger.Debug("<- OmniPascal header: %s", trimmed)

		if strings.HasPrefix(trimmed, "Content-Length: ") {
			if _, err := fmt.Sscanf(trimmed, "Content-Length: %d", &contentLength); err != nil {
				return nil, fmt.Errorf("invalid OmniPascal Content-Length: %w", err)
			}
		}
	}

	if contentLength <= 0 {
		return nil, fmt.Errorf("missing OmniPascal Content-Length header")
	}

	payload := make([]byte, contentLength)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, fmt.Errorf("failed to read OmniPascal payload: %w", err)
	}

	omniWireLogger.Debug("<- OmniPascal: %s", string(payload))
	return payload, nil
}

func normalizePath(path string) string {
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		// OmniPascal can emit diagnostics with different drive-letter casing.
		// Normalize to a single key form to avoid stale/empty cache reads.
		path = strings.ToLower(path)
	}
	return path
}

func newProcessStartError(kind, command string, err error) error {
	if !isPermissionLikeError(err) {
		return fmt.Errorf("failed to start %s: %w", kind, err)
	}

	absCommand := command
	if resolved, resolveErr := exec.LookPath(command); resolveErr == nil {
		absCommand = resolved
	}

	hints := []string{
		fmt.Sprintf("failed to start %s: %v", kind, err),
		fmt.Sprintf("executable: %s", absCommand),
	}

	if runtime.GOOS == "windows" {
		hints = append(hints,
			"Windows denied process spawn. If this binary is newly downloaded/compiled, unblock it and retry:",
			fmt.Sprintf("  Unblock-File -Path \"%s\"", absCommand),
			"If the executable is under Downloads, move/copy it to a trusted path (for example C:\\Users\\<you>\\go\\bin) and update MCP config.",
		)
	}

	return errors.New(strings.Join(hints, "\n"))
}

func isPermissionLikeError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, os.ErrPermission) {
		return true
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "acesso negado") ||
		strings.Contains(message, "access is denied") ||
		strings.Contains(message, "permission denied") ||
		strings.Contains(message, "operation not permitted") ||
		strings.Contains(message, "eperm")
}
