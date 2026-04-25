package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestMCPServer_WhenLoggingSetLevelIsUnsupported_DoesNotAdvertiseLoggingCapability(t *testing.T) {
	svc := &mcpServer{
		ctx: context.Background(),
	}
	svc.mcpServer = newMCPServer()

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
