package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/mark3labs/mcp-go/mcp"
)

const buildInitOptionsFakeLSPEnv = "MCP_FAKE_LSP_BUILD_INIT_OPTIONS"
const buildInitOptionsExpectedJSONEnv = "MCP_FAKE_LSP_BUILD_INIT_OPTIONS_EXPECTED_JSON"

func TestMCPServer_WhenLoggingSetLevelIsUnsupported_DoesNotAdvertiseLoggingCapability(t *testing.T) {
	svc := &mcpServer{
		ctx: context.Background(),
	}
	svc.mcpServer = newMCPServer(t.TempDir())

	if err := svc.registerTools(); err != nil {
		t.Fatalf("registerTools() returned error: %v", err)
	}

	request := mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      1,
		Request: mcp.Request{Method: string(mcp.MethodInitialize)},
		Params: map[string]any{
			"protocolVersion": mcp.LATEST_PROTOCOL_VERSION,
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "mcp-language-server-test",
				"version": "0.0.0",
			},
		},
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("json.Marshal(initialize request) returned error: %v", err)
	}

	response := svc.mcpServer.HandleMessage(context.Background(), requestBytes)
	initializeResponse, ok := response.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("HandleMessage() returned %T, want mcp.JSONRPCResponse", response)
	}

	initializeResult, ok := initializeResponse.Result.(mcp.InitializeResult)
	if !ok {
		t.Fatalf("initialize result has type %T, want mcp.InitializeResult", initializeResponse.Result)
	}

	if initializeResult.Capabilities.Tools == nil {
		t.Fatal("initialize result did not advertise tools capability")
	}

	if initializeResult.Capabilities.Logging != nil {
		t.Fatalf("initialize result advertised logging capability even though logging/setLevel is unsupported")
	}
}

func TestParseConfig_WithFlags(t *testing.T) {
	// Isolate global flag.CommandLine and os.Args to avoid polluting other tests
	oldFlag := flag.CommandLine
	defer func() { flag.CommandLine = oldFlag }()
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tmpDir := t.TempDir()
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	if err := os.Mkdir(workspaceRoot, 0o755); err != nil {
		t.Fatalf("failed to create workspace root: %v", err)
	}

	// Use the `go` executable which should be present in test environments
	os.Args = []string{"mcp-language-server", "--workspace", workspaceRoot, "--lsp", "go", "--search-path", "src;lib; vendor"}

	cfg, err := parseConfig()
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}

	want := []string{"src", "lib", "vendor"}
	got := cfg.initializationOptions.SearchPaths
	if len(got) != len(want) {
		t.Fatalf("unexpected searchPaths length: got=%d want=%d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected search path at %d: got=%q want=%q", i, got[i], want[i])
		}
	}
}

func TestBuildInitializeOptions_MergesFileAndFlags(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	if err := os.Mkdir(workspaceRoot, 0o755); err != nil {
		t.Fatalf("failed to create workspace root: %v", err)
	}

	initOptionsPath := filepath.Join(tmpDir, "initialization-options.json")
	initOptionsPayload := map[string]any{
		"searchPaths":            []string{"from-file", "   "},
		"delphiInstallationPath": "C:/DelphiFromFile",
		"workspaceSettings": map[string]any{
			workspaceRoot: map[string]any{
				"searchPaths":            []string{"ws-file", ""},
				"delphiInstallationPath": "   ",
			},
		},
	}
	initOptionsJSON, err := json.Marshal(initOptionsPayload)
	if err != nil {
		t.Fatalf("failed to marshal initialization options payload: %v", err)
	}

	if err := os.WriteFile(initOptionsPath, initOptionsJSON, 0o600); err != nil {
		t.Fatalf("failed to write initialization options file: %v", err)
	}

	options, err := buildInitializeOptions(initOptionsPath, []string{"from-flag", "\t"}, "C:/DelphiFromFlag")
	if err != nil {
		t.Fatalf("buildInitializeOptions returned error: %v", err)
	}

	if got, want := len(options.SearchPaths), 2; got != want {
		t.Fatalf("unexpected searchPaths length: got=%d want=%d", got, want)
	}

	if got, want := options.SearchPaths[0], "from-file"; got != want {
		t.Fatalf("unexpected first search path: got=%q want=%q", got, want)
	}

	if got, want := options.SearchPaths[1], "from-flag"; got != want {
		t.Fatalf("unexpected second search path: got=%q want=%q", got, want)
	}

	if got, want := options.DelphiInstallationPath, "C:/DelphiFromFlag"; got != want {
		t.Fatalf("unexpected delphiInstallationPath: got=%q want=%q", got, want)
	}

	workspaceURI := string(protocol.URIFromPath(workspaceRoot))
	wsSettings, ok := options.WorkspaceSettings[workspaceURI]
	if !ok {
		t.Fatalf("expected workspace settings for %q", workspaceURI)
	}

	if got, want := len(wsSettings.SearchPaths), 1; got != want {
		t.Fatalf("unexpected workspace searchPaths length: got=%d want=%d", got, want)
	}

	if got, want := wsSettings.SearchPaths[0], "ws-file"; got != want {
		t.Fatalf("unexpected workspace search path: got=%q want=%q", got, want)
	}

	if wsSettings.DelphiInstallationPath != "" {
		t.Fatalf("expected empty workspace delphiInstallationPath after normalization, got %q", wsSettings.DelphiInstallationPath)
	}
}

func TestBuildInitializeOptions_FromJSONFile_PreservesUnknownKeysUntilInitializePayload(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	if err := os.Mkdir(workspaceRoot, 0o755); err != nil {
		t.Fatalf("failed to create workspace root: %v", err)
	}

	initOptionsPath := filepath.Join(tmpDir, "initialization-options.json")
	initOptionsPayload := map[string]any{
		"searchPaths": []string{" from-file "},
		"unknownTopLevel": map[string]any{
			"dialect": "oracle",
			"strict":  true,
		},
		"workspaceSettings": map[string]any{
			workspaceRoot: map[string]any{
				"searchPaths": []string{" ws-file "},
				"unknownWorkspaceField": map[string]any{
					"mode": "extended",
				},
			},
		},
	}
	initOptionsJSON, err := json.Marshal(initOptionsPayload)
	if err != nil {
		t.Fatalf("failed to marshal initialization options payload: %v", err)
	}

	if err := os.WriteFile(initOptionsPath, initOptionsJSON, 0o600); err != nil {
		t.Fatalf("failed to write initialization options file: %v", err)
	}

	options, err := buildInitializeOptions(initOptionsPath, nil, "")
	if err != nil {
		t.Fatalf("buildInitializeOptions returned error: %v", err)
	}

	expectedPayload := map[string]any{
		"searchPaths": []string{"from-file"},
		"unknownTopLevel": map[string]any{
			"dialect": "oracle",
			"strict":  true,
		},
		"workspaceSettings": map[string]any{
			string(protocol.URIFromPath(workspaceRoot)): map[string]any{
				"searchPaths": []string{"ws-file"},
				"unknownWorkspaceField": map[string]any{
					"mode": "extended",
				},
			},
		},
		"codelenses": expectedInitializeCodeLenses(),
	}

	if err := assertInitializeOptionsPayload(t, options, workspaceRoot, expectedPayload); err != nil {
		t.Fatalf("expected unknown keys from initialization-options file to survive until initialize payload: %v", err)
	}
}

func TestBuildInitializeOptions_FromJSONFile_ResolvesRelativeWorkspaceSettingsRootAgainstOptionsFileDir(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	workspaceRoot := filepath.Join(configDir, "workspace-relative")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatalf("failed to create workspace root: %v", err)
	}

	initOptionsPath := filepath.Join(configDir, "initialization-options.json")
	initOptionsPayload := map[string]any{
		"workspaceSettings": map[string]any{
			"./workspace-relative": map[string]any{
				"searchPaths": []string{"./pkg"},
			},
		},
	}
	initOptionsJSON, err := json.Marshal(initOptionsPayload)
	if err != nil {
		t.Fatalf("failed to marshal initialization options payload: %v", err)
	}

	if err := os.WriteFile(initOptionsPath, initOptionsJSON, 0o600); err != nil {
		t.Fatalf("failed to write initialization options file: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	otherWD := filepath.Join(tmpDir, "other-cwd")
	if err := os.Mkdir(otherWD, 0o755); err != nil {
		t.Fatalf("failed to create alternate working directory: %v", err)
	}
	if err := os.Chdir(otherWD); err != nil {
		t.Fatalf("failed to switch working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWD); err != nil {
			t.Fatalf("failed to restore working directory: %v", err)
		}
	}()

	options, err := buildInitializeOptions(initOptionsPath, nil, "")
	if err != nil {
		t.Fatalf("buildInitializeOptions returned error: %v", err)
	}

	expectedPayload := map[string]any{
		"workspaceSettings": map[string]any{
			string(protocol.URIFromPath(workspaceRoot)): map[string]any{
				"searchPaths": []string{"./pkg"},
			},
		},
		"codelenses": expectedInitializeCodeLenses(),
	}

	if err := assertInitializeOptionsPayload(t, options, workspaceRoot, expectedPayload); err != nil {
		t.Fatalf("expected relative workspaceSettings roots to resolve against the initialization-options file directory while preserving relative searchPaths: %v", err)
	}
}

func TestBuildInitializeOptions_InvalidJSON_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	initOptionsPath := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(initOptionsPath, []byte("{"), 0o600); err != nil {
		t.Fatalf("failed to write invalid json file: %v", err)
	}

	if _, err := buildInitializeOptions(initOptionsPath, nil, ""); err == nil {
		t.Fatal("expected buildInitializeOptions to fail on invalid JSON")
	}
}

func TestBuildInitializeOptions_SplitsSemicolonInFlags(t *testing.T) {
	options, err := buildInitializeOptions("", []string{" src ; lib ; ; vendor "}, "")
	if err != nil {
		t.Fatalf("buildInitializeOptions returned error: %v", err)
	}

	if got, want := len(options.SearchPaths), 3; got != want {
		t.Fatalf("unexpected searchPaths length: got=%d want=%d", got, want)
	}

	wantSlice := []string{"src", "lib", "vendor"}
	for i := range wantSlice {
		if options.SearchPaths[i] != wantSlice[i] {
			t.Fatalf("unexpected search path at %d: got=%q want=%q", i, options.SearchPaths[i], wantSlice[i])
		}
	}
}

func TestBuildInitializeOptions_MergesFileAndFlags_WithSemicolonExpansionOrderPreserved(t *testing.T) {
	tmpDir := t.TempDir()
	initOptionsPath := filepath.Join(tmpDir, "initialization-options.json")

	initOptionsPayload := map[string]any{
		"searchPaths": []string{"from-file"},
	}
	initOptionsJSON, err := json.Marshal(initOptionsPayload)
	if err != nil {
		t.Fatalf("failed to marshal initialization options payload: %v", err)
	}
	if err := os.WriteFile(initOptionsPath, initOptionsJSON, 0o600); err != nil {
		t.Fatalf("failed to write initialization options file: %v", err)
	}

	options, err := buildInitializeOptions(initOptionsPath, []string{" a ", "b ; c"}, "")
	if err != nil {
		t.Fatalf("buildInitializeOptions returned error: %v", err)
	}

	want := []string{"from-file", "a", "b", "c"}
	if got := options.SearchPaths; len(got) != len(want) {
		t.Fatalf("unexpected searchPaths length: got=%d want=%d", len(got), len(want))
	}
	for i := range want {
		if options.SearchPaths[i] != want[i] {
			t.Fatalf("unexpected search path at %d: got=%q want=%q", i, options.SearchPaths[i], want[i])
		}
	}
}

func TestBuildInitializeOptions_SearchPathsStringInFileFails(t *testing.T) {
	tmpDir := t.TempDir()
	initOptionsPath := filepath.Join(tmpDir, "bad_searchpaths.json")
	// searchPaths as a single string should be considered invalid JSON schema for our parser
	if err := os.WriteFile(initOptionsPath, []byte(`{"searchPaths":"a;b;c"}`), 0o600); err != nil {
		t.Fatalf("failed to write initialization options file: %v", err)
	}

	if _, err := buildInitializeOptions(initOptionsPath, nil, ""); err == nil {
		t.Fatal("expected buildInitializeOptions to fail when searchPaths in file is a string")
	}
}

func TestHelperProcessBuildInitializeOptionsFakeLSP(t *testing.T) {
	if os.Getenv(buildInitOptionsFakeLSPEnv) != "1" {
		return
	}

	runBuildInitializeOptionsFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func assertInitializeOptionsPayload(t *testing.T, options lsp.InitializeOptions, workspaceDir string, expectedPayload map[string]any) error {
	t.Helper()

	expectedJSON, err := json.Marshal(expectedPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal expected initializationOptions payload: %w", err)
	}

	t.Setenv(buildInitOptionsFakeLSPEnv, "1")
	t.Setenv(buildInitOptionsExpectedJSONEnv, string(expectedJSON))

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve test binary path: %w", err)
	}

	client, err := lsp.NewClient(execPath, "-test.run=TestHelperProcessBuildInitializeOptionsFakeLSP")
	if err != nil {
		return fmt.Errorf("failed to start fake LSP: %w", err)
	}
	defer func() {
		if client.Cmd != nil && client.Cmd.Process != nil {
			_ = client.Cmd.Process.Kill()
			_, _ = client.Cmd.Process.Wait()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := client.SetInitializationOptions(options); err != nil {
		return fmt.Errorf("SetInitializationOptions returned error: %w", err)
	}

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		return fmt.Errorf("InitializeLSPClient returned error: %w", err)
	}

	return nil
}

func expectedInitializeCodeLenses() map[string]bool {
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

func runBuildInitializeOptionsFakeLSP(stdin *os.File, stdout *os.File) {
	reader := bufio.NewReader(stdin)
	writer := stdout

	expectedJSON := os.Getenv(buildInitOptionsExpectedJSONEnv)
	var expected any
	if expectedJSON != "" {
		_ = json.Unmarshal([]byte(expectedJSON), &expected)
	}

	for {
		msg, err := lsp.ReadMessage(reader)
		if err != nil {
			return
		}

		switch msg.Method {
		case "initialize":
			var params map[string]any
			_ = json.Unmarshal(msg.Params, &params)

			initOptsRaw, ok := params["initializationOptions"]
			if !ok {
				sendBuildInitializeOptionsFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32602, Message: "missing initializationOptions"})
				continue
			}

			initOpts, ok := initOptsRaw.(map[string]any)
			if !ok {
				sendBuildInitializeOptionsFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32602, Message: "initializationOptions must be object"})
				continue
			}

			if expected != nil && !deepEqualJSONLike(initOpts, expected) {
				sendBuildInitializeOptionsFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32602, Message: fmt.Sprintf("initializationOptions mismatch: got=%v expected=%v", initOpts, expected)})
				continue
			}

			if searchPaths, ok := initOpts["searchPaths"]; ok {
				if singleValue, ok := searchPaths.(string); ok && strings.Contains(singleValue, ";") {
					sendBuildInitializeOptionsFakeResponse(writer, msg.ID, nil, &lsp.ResponseError{Code: -32602, Message: "searchPaths must be an array, not semicolon-separated string"})
					continue
				}
			}

			sendBuildInitializeOptionsFakeResponse(writer, msg.ID, map[string]any{"capabilities": map[string]any{}}, nil)

		case "initialized":
			continue
		case "shutdown":
			sendBuildInitializeOptionsFakeResponse(writer, msg.ID, nil, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendBuildInitializeOptionsFakeResponse(writer, msg.ID, nil, nil)
			}
		}
	}
}

func sendBuildInitializeOptionsFakeResponse(w *os.File, id *lsp.MessageID, result any, rpcErr *lsp.ResponseError) {
	resp := &lsp.Message{JSONRPC: "2.0", ID: id, Error: rpcErr}
	if rpcErr == nil {
		if result == nil {
			resp.Result = mustMarshalBuildInitializeOptionsFake(struct{}{})
		} else {
			resp.Result = mustMarshalBuildInitializeOptionsFake(result)
		}
	}
	_ = lsp.WriteMessage(w, resp)
}

func mustMarshalBuildInitializeOptionsFake(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal fake LSP json: %v", err))
	}
	return b
}

func deepEqualJSONLike(a any, b any) bool {
	ab, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bb, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(ab) == string(bb)
}
