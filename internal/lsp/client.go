package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

type Client struct {
	Cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr io.ReadCloser

	// Capabilities returned by server initialize response.
	capabilities   protocol.ServerCapabilities
	capabilitiesMu sync.RWMutex

	// initializationOptions contains user-configurable initialize payload fields.
	initializationOptions   InitializeOptions
	initializationOptionsMu sync.RWMutex

	// Request ID counter
	nextID atomic.Int32

	// Response handlers
	handlers   map[string]chan *Message
	handlersMu sync.RWMutex

	// Server request handlers
	serverRequestHandlers map[string]ServerRequestHandler
	serverHandlersMu      sync.RWMutex

	// Notification handlers
	notificationHandlers map[string]NotificationHandler
	notificationMu       sync.RWMutex

	// Diagnostic cache
	diagnostics   map[protocol.DocumentUri][]protocol.Diagnostic
	diagnosticsMu sync.RWMutex

	// Files are currently opened by the LSP
	openFiles   map[string]*OpenFileInfo
	openFilesMu sync.RWMutex
}

func NewClient(command string, args ...string) (*Client, error) {
	cmd := exec.Command(command, args...)
	// Copy env
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
		Cmd:                   cmd,
		stdin:                 stdin,
		stdout:                bufio.NewReader(stdout),
		stderr:                stderr,
		handlers:              make(map[string]chan *Message),
		notificationHandlers:  make(map[string]NotificationHandler),
		serverRequestHandlers: make(map[string]ServerRequestHandler),
		diagnostics:           make(map[protocol.DocumentUri][]protocol.Diagnostic),
		openFiles:             make(map[string]*OpenFileInfo),
	}

	// Start the LSP server process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start LSP server: %w", err)
	}

	// Handle stderr in a separate goroutine with proper logging
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			processLogger.Info("%s", line)
		}
		if err := scanner.Err(); err != nil {
			lspLogger.Error("Error reading LSP server stderr: %v", err)
		}
	}()

	// Start message handling loop
	go client.handleMessages()

	return client, nil
}

func (c *Client) RegisterNotificationHandler(method string, handler NotificationHandler) {
	c.notificationMu.Lock()
	defer c.notificationMu.Unlock()
	c.notificationHandlers[method] = handler
}

func (c *Client) RegisterServerRequestHandler(method string, handler ServerRequestHandler) {
	c.serverHandlersMu.Lock()
	defer c.serverHandlersMu.Unlock()
	c.serverRequestHandlers[method] = handler
}

// SetInitializationOptions stores normalized initializationOptions payload fields
// that will be merged into initialize.initializationOptions.
func (c *Client) SetInitializationOptions(options InitializeOptions) error {
	normalized, err := NormalizeInitializeOptions(options)
	if err != nil {
		return err
	}

	c.initializationOptionsMu.Lock()
	c.initializationOptions = cloneInitializeOptions(normalized)
	c.initializationOptionsMu.Unlock()

	return nil
}

func (c *Client) buildInitializationOptionsPayload() map[string]any {
	payload := make(map[string]any)

	c.initializationOptionsMu.RLock()
	userOptions := cloneInitializeOptions(c.initializationOptions)
	c.initializationOptionsMu.RUnlock()

	mergeInitializationExtraFields(payload, userOptions.extraFields)

	if len(userOptions.SearchPaths) > 0 {
		payload["searchPaths"] = append([]string(nil), userOptions.SearchPaths...)
	}

	if userOptions.DelphiInstallationPath != "" {
		payload["delphiInstallationPath"] = userOptions.DelphiInstallationPath
	}

	if len(userOptions.WorkspaceSettings) > 0 {
		workspaceSettings := make(map[string]any, len(userOptions.WorkspaceSettings))
		for root, settings := range userOptions.WorkspaceSettings {
			settingMap := make(map[string]any)
			mergeInitializationExtraFields(settingMap, settings.extraFields)
			if len(settings.SearchPaths) > 0 {
				settingMap["searchPaths"] = append([]string(nil), settings.SearchPaths...)
			}
			if settings.DelphiInstallationPath != "" {
				settingMap["delphiInstallationPath"] = settings.DelphiInstallationPath
			}
			if len(settingMap) == 0 {
				continue
			}
			workspaceSettings[root] = settingMap
		}

		if len(workspaceSettings) > 0 {
			payload["workspaceSettings"] = workspaceSettings
		}
	}

	payload["codelenses"] = defaultCodeLensesInitializationOptions()

	return payload
}

func cloneInitializeOptions(options InitializeOptions) InitializeOptions {
	cloned := InitializeOptions{
		SearchPaths:            append([]string(nil), options.SearchPaths...),
		DelphiInstallationPath: options.DelphiInstallationPath,
		extraFields:            cloneRawMessages(options.extraFields),
	}

	if len(options.WorkspaceSettings) > 0 {
		cloned.WorkspaceSettings = make(map[string]WorkspaceInitializationOptions, len(options.WorkspaceSettings))
		for root, settings := range options.WorkspaceSettings {
			cloned.WorkspaceSettings[root] = WorkspaceInitializationOptions{
				SearchPaths:            append([]string(nil), settings.SearchPaths...),
				DelphiInstallationPath: settings.DelphiInstallationPath,
				extraFields:            cloneRawMessages(settings.extraFields),
			}
		}
	}

	return cloned
}

func mergeInitializationExtraFields(target map[string]any, extraFields map[string]json.RawMessage) {
	for key, rawValue := range extraFields {
		var decodedValue any
		if err := json.Unmarshal(rawValue, &decodedValue); err != nil {
			continue
		}

		target[key] = decodedValue
	}
}

func defaultCodeLensesInitializationOptions() map[string]bool {
	return map[string]bool{
		"generate":           true,
		"regenerate_cgo":     true,
		"test":               true,
		"tidy":               true,
		"upgrade_dependency": true,
		"vendor":             true,
		"vulncheck":          false,
	}
}

func (c *Client) InitializeLSPClient(ctx context.Context, workspaceDir string) (*protocol.InitializeResult, error) {
	workspaceURI := protocol.URIFromPath(workspaceDir)
	initializationOptions := c.buildInitializationOptionsPayload()

	initParams := &protocol.InitializeParams{
		WorkspaceFoldersInitializeParams: protocol.WorkspaceFoldersInitializeParams{
			WorkspaceFolders: []protocol.WorkspaceFolder{
				{
					URI:  protocol.URI(workspaceURI),
					Name: workspaceDir,
				},
			},
		},

		XInitializeParams: protocol.XInitializeParams{
			ProcessID: int32(os.Getpid()),
			ClientInfo: &protocol.ClientInfo{
				Name:    "mcp-language-server",
				Version: "0.1.0",
			},
			RootPath: workspaceDir,
			RootURI:  workspaceURI,
			Capabilities: protocol.ClientCapabilities{
				Workspace: protocol.WorkspaceClientCapabilities{
					Configuration: true,
					DidChangeConfiguration: protocol.DidChangeConfigurationClientCapabilities{
						DynamicRegistration: true,
					},
					DidChangeWatchedFiles: protocol.DidChangeWatchedFilesClientCapabilities{
						DynamicRegistration:    true,
						RelativePatternSupport: true,
					},
				},
				TextDocument: protocol.TextDocumentClientCapabilities{
					Synchronization: &protocol.TextDocumentSyncClientCapabilities{
						DynamicRegistration: true,
						DidSave:             true,
					},
					Completion: protocol.CompletionClientCapabilities{
						CompletionItem: protocol.ClientCompletionItemOptions{},
					},
					CodeLens: &protocol.CodeLensClientCapabilities{
						DynamicRegistration: true,
					},
					DocumentSymbol: protocol.DocumentSymbolClientCapabilities{},
					CodeAction: protocol.CodeActionClientCapabilities{
						CodeActionLiteralSupport: protocol.ClientCodeActionLiteralOptions{
							CodeActionKind: protocol.ClientCodeActionKindOptions{
								ValueSet: []protocol.CodeActionKind{},
							},
						},
					},
					PublishDiagnostics: protocol.PublishDiagnosticsClientCapabilities{
						VersionSupport: true,
					},
					SemanticTokens: protocol.SemanticTokensClientCapabilities{
						Requests: protocol.ClientSemanticTokensRequestOptions{
							Range: &protocol.Or_ClientSemanticTokensRequestOptions_range{},
							Full:  &protocol.Or_ClientSemanticTokensRequestOptions_full{},
						},
						TokenTypes:     []string{},
						TokenModifiers: []string{},
						Formats:        []protocol.TokenFormat{},
					},
				},
				Window: protocol.WindowClientCapabilities{},
			},
			InitializationOptions: initializationOptions,
		},
	}

	var result protocol.InitializeResult
	if err := c.Call(ctx, "initialize", initParams, &result); err != nil {
		return nil, fmt.Errorf("initialize failed: %w", err)
	}

	c.capabilitiesMu.Lock()
	c.capabilities = result.Capabilities
	c.capabilitiesMu.Unlock()

	if err := c.Notify(ctx, "initialized", struct{}{}); err != nil {
		return nil, fmt.Errorf("initialized notification failed: %w", err)
	}

	// Register handlers
	c.RegisterServerRequestHandler("workspace/applyEdit", HandleApplyEdit)
	c.RegisterServerRequestHandler("workspace/configuration", HandleWorkspaceConfiguration)
	c.RegisterServerRequestHandler("client/registerCapability", HandleRegisterCapability)
	c.RegisterNotificationHandler("window/showMessage", HandleServerMessage)
	c.RegisterNotificationHandler("textDocument/publishDiagnostics",
		func(params json.RawMessage) { HandleDiagnostics(c, params) })

	// Notify the LSP server
	err := c.Initialized(ctx, protocol.InitializedParams{})
	if err != nil {
		return nil, fmt.Errorf("initialization failed: %w", err)
	}

	// LSP sepecific Initialization
	path := strings.ToLower(c.Cmd.Path)
	switch {
	case strings.Contains(path, "typescript-language-server"):
		err := initializeTypescriptLanguageServer(ctx, c, workspaceDir)
		if err != nil {
			return nil, err
		}
	}

	return &result, nil
}

func (c *Client) SupportsDocumentSymbol() bool {
	c.capabilitiesMu.RLock()
	provider := c.capabilities.DocumentSymbolProvider
	c.capabilitiesMu.RUnlock()

	return supportsDocumentSymbolProvider(provider)
}

func (c *Client) SupportsWorkspaceSymbol() bool {
	c.capabilitiesMu.RLock()
	provider := c.capabilities.WorkspaceSymbolProvider
	c.capabilitiesMu.RUnlock()

	return supportsWorkspaceSymbolProvider(provider)
}

func supportsDocumentSymbolProvider(provider *protocol.Or_ServerCapabilities_documentSymbolProvider) bool {
	if provider == nil {
		return false
	}

	v := provider.Value
	switch typed := v.(type) {
	case nil:
		return false
	case bool:
		return typed
	default:
		return true
	}
}

func supportsWorkspaceSymbolProvider(provider *protocol.Or_ServerCapabilities_workspaceSymbolProvider) bool {
	if provider == nil {
		return false
	}

	v := provider.Value
	switch typed := v.(type) {
	case nil:
		return false
	case bool:
		return typed
	default:
		return true
	}
}

func (c *Client) Close() error {
	// Try to close all open files first
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt to close files but continue shutdown regardless
	c.CloseAllFiles(ctx)

	// Force kill the LSP process if it doesn't exit within timeout
	forcedKill := make(chan struct{})
	var closeForcedKillOnce sync.Once
	closeForcedKill := func() {
		closeForcedKillOnce.Do(func() {
			close(forcedKill)
		})
	}

	go func() {
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()

		select {
		case <-timer.C:
			lspLogger.Warn("LSP process did not exit within timeout, forcing kill")
			if c.Cmd.Process != nil {
				if err := c.Cmd.Process.Kill(); err != nil {
					lspLogger.Error("Failed to kill process: %v", err)
				} else {
					lspLogger.Info("Process killed successfully")
				}
			}
			closeForcedKill()
		case <-forcedKill:
			// Channel closed from completion path
			return
		}
	}()

	// Close stdin to signal the server
	if err := c.stdin.Close(); err != nil {
		lspLogger.Error("Failed to close stdin: %v", err)
	}

	// Wait for process to exit
	err := c.Cmd.Wait()
	closeForcedKill() // Stop the force kill goroutine

	return err
}

type ServerState int

const (
	StateStarting ServerState = iota
	StateReady
	StateError
)

func (c *Client) WaitForServerReady(ctx context.Context) error {
	// TODO: wait for specific messages or poll workspace/symbol
	time.Sleep(time.Second * 1)
	return nil
}

type OpenFileInfo struct {
	Version int32
	URI     protocol.DocumentUri
}

func (c *Client) OpenFile(ctx context.Context, filepath string) error {
	uri := string(protocol.URIFromPath(filepath))

	c.openFilesMu.Lock()
	if _, exists := c.openFiles[uri]; exists {
		c.openFilesMu.Unlock()
		return nil // Already open
	}
	c.openFilesMu.Unlock()

	// Skip files that do not exist or cannot be read
	content, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	params := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        protocol.DocumentUri(uri),
			LanguageID: DetectLanguageID(uri),
			Version:    1,
			Text:       string(content),
		},
	}

	if err := c.Notify(ctx, "textDocument/didOpen", params); err != nil {
		return err
	}

	c.openFilesMu.Lock()
	c.openFiles[uri] = &OpenFileInfo{
		Version: 1,
		URI:     protocol.DocumentUri(uri),
	}
	c.openFilesMu.Unlock()

	lspLogger.Debug("Opened file: %s", filepath)

	return nil
}

func (c *Client) NotifyChange(ctx context.Context, filepath string) error {
	uri := string(protocol.URIFromPath(filepath))

	content, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	c.openFilesMu.Lock()
	fileInfo, isOpen := c.openFiles[uri]
	if !isOpen {
		c.openFilesMu.Unlock()
		return fmt.Errorf("cannot notify change for unopened file: %s", filepath)
	}

	// Increment version
	fileInfo.Version++
	version := fileInfo.Version
	c.openFilesMu.Unlock()

	params := protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentUri(uri),
			},
			Version: version,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Value: protocol.TextDocumentContentChangeWholeDocument{
					Text: string(content),
				},
			},
		},
	}

	return c.Notify(ctx, "textDocument/didChange", params)
}

func (c *Client) CloseFile(ctx context.Context, filepath string) error {
	uri := string(protocol.URIFromPath(filepath))

	c.openFilesMu.Lock()
	if _, exists := c.openFiles[uri]; !exists {
		c.openFilesMu.Unlock()
		return nil // Already closed
	}
	c.openFilesMu.Unlock()

	params := protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.DocumentUri(uri),
		},
	}
	lspLogger.Debug("Closing file: %s", params.TextDocument.URI.Dir())
	if err := c.Notify(ctx, "textDocument/didClose", params); err != nil {
		return err
	}

	c.openFilesMu.Lock()
	delete(c.openFiles, uri)
	c.openFilesMu.Unlock()

	return nil
}

func (c *Client) IsFileOpen(filepath string) bool {
	uri := string(protocol.URIFromPath(filepath))
	c.openFilesMu.RLock()
	defer c.openFilesMu.RUnlock()
	_, exists := c.openFiles[uri]
	return exists
}

// GetOpenFilesSnapshot returns a sorted copy of currently opened file URIs.
func (c *Client) GetOpenFilesSnapshot() []string {
	c.openFilesMu.RLock()
	defer c.openFilesMu.RUnlock()

	openFiles := make([]string, 0, len(c.openFiles))
	for uri := range c.openFiles {
		openFiles = append(openFiles, uri)
	}

	sort.Strings(openFiles)
	return openFiles
}

// CloseAllFiles closes all currently open files
func (c *Client) CloseAllFiles(ctx context.Context) {
	c.openFilesMu.Lock()
	filesToClose := make([]string, 0, len(c.openFiles))

	// First collect all URIs that need to be closed
	for uri := range c.openFiles {
		filePath, err := parseDocumentURIPath(uri)
		if err != nil {
			lspLogger.Error("Skipping close for invalid open file URI %q: %v", uri, err)
			continue
		}
		filesToClose = append(filesToClose, filePath)
	}
	c.openFilesMu.Unlock()

	// Then close them all
	for _, filePath := range filesToClose {
		err := c.CloseFile(ctx, filePath)
		if err != nil {
			lspLogger.Error("Error closing file %s: %v", filePath, err)
		}
	}

	lspLogger.Debug("Closed %d files", len(filesToClose))
}

func parseDocumentURIPath(rawURI string) (string, error) {
	parsedURI, err := protocol.ParseDocumentUri(rawURI)
	if err != nil {
		return "", fmt.Errorf("failed to parse document URI %q: %w", rawURI, err)
	}

	path, err := safeDocumentURIPath(parsedURI)
	if err != nil {
		return "", fmt.Errorf("failed to resolve document URI path %q: %w", rawURI, err)
	}

	if path == "" {
		return "", fmt.Errorf("document URI %q resolved to empty path", rawURI)
	}

	return path, nil
}

func safeDocumentURIPath(uri protocol.DocumentUri) (path string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("failed to convert document URI to path: %v", recovered)
		}
	}()

	return uri.Path(), nil
}

func (c *Client) GetFileDiagnostics(uri protocol.DocumentUri) []protocol.Diagnostic {
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()

	return c.diagnostics[uri]
}
