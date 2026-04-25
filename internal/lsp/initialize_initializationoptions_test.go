package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

const initOptionsFakeLSPEnv = "MCP_FAKE_LSP_INITOPTIONS"
const initOptionsExpectedJSONEnv = "MCP_FAKE_LSP_INITOPTIONS_EXPECTED_JSON"
const initOptionsExpectedWorkspaceSettingsEnv = "MCP_FAKE_LSP_INITOPTIONS_EXPECTED_WORKSPACE_SETTINGS_JSON"

// Helper process entrypoint for fake LSP that validates initialize.initializationOptions
func TestHelperProcessInitializeInitOptionsFakeLSP(t *testing.T) {
	if os.Getenv(initOptionsFakeLSPEnv) != "1" {
		return
	}

	runInitializeInitOptionsFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestInitializeLSPClient_SendsInitializationOptions_Global(t *testing.T) {
	workspaceDir := `C:\Users\alice\workspace`
	inputOptions := InitializeOptions{
		SearchPaths:            []string{"src", "lib", "C:/absolute/path"},
		DelphiInstallationPath: "C:/Program Files/Embarcadero/Delphi",
	}

	// expected payload (JSON) passed to helper process for validation
	expected := map[string]any{
		"searchPaths":            []string{"src", "lib", "C:/absolute/path"},
		"delphiInstallationPath": "C:/Program Files/Embarcadero/Delphi",
		"codelenses":             defaultCodeLensesInitializationOptions(),
	}
	b, _ := json.Marshal(expected)

	t.Setenv(initOptionsFakeLSPEnv, "1")
	t.Setenv(initOptionsExpectedJSONEnv, string(b))

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := NewClient(execPath, "-test.run=TestHelperProcessInitializeInitOptionsFakeLSP")
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

	if err := client.SetInitializationOptions(inputOptions); err != nil {
		t.Fatalf("expected SetInitializationOptions to accept global options, got error: %v", err)
	}

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		t.Fatalf("expected InitializeLSPClient to send initializationOptions, got error: %v", err)
	}
}

func TestInitializeLSPClient_SendsInitializationOptions_WorkspaceScoped(t *testing.T) {
	workspaceDir := `C:\Users\bob\project`
	inputOptions := InitializeOptions{
		SearchPaths: []string{"global1"},
		WorkspaceSettings: map[string]WorkspaceInitializationOptions{
			workspaceDir: {
				SearchPaths: []string{"mod", "ext"},
			},
		},
	}

	// workspace-scoped settings: map keyed by file:// URI
	wsRoot := string(protocol.URIFromPath(workspaceDir))
	workspaceSettings := map[string]any{
		wsRoot: map[string]any{
			"searchPaths": []string{"mod", "ext"},
		},
	}

	expected := map[string]any{
		"searchPaths":       []string{"global1"},
		"workspaceSettings": workspaceSettings,
		"codelenses":        defaultCodeLensesInitializationOptions(),
	}
	b, _ := json.Marshal(expected)

	t.Setenv(initOptionsFakeLSPEnv, "1")
	t.Setenv(initOptionsExpectedJSONEnv, string(b))

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := NewClient(execPath, "-test.run=TestHelperProcessInitializeInitOptionsFakeLSP")
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

	if err := client.SetInitializationOptions(inputOptions); err != nil {
		t.Fatalf("expected SetInitializationOptions to accept workspace-scoped options, got error: %v", err)
	}

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		t.Fatalf("expected InitializeLSPClient to send workspace-scoped initializationOptions, got error: %v", err)
	}
}

func TestInitializeLSPClient_NormalizesEmptyEntries(t *testing.T) {
	workspaceDir := `C:\Users\carol\repo`
	inputOptions := InitializeOptions{
		SearchPaths:            []string{"", "   ", "keep"},
		DelphiInstallationPath: "  ",
		WorkspaceSettings: map[string]WorkspaceInitializationOptions{
			"   ": {
				SearchPaths: []string{"ignored"},
			},
			workspaceDir: {
				SearchPaths:            []string{" ", ""},
				DelphiInstallationPath: "\t",
			},
		},
	}

	// expected normalized: empty/whitespace-only removed
	expected := map[string]any{
		"searchPaths": []string{"keep"},
		"codelenses":  defaultCodeLensesInitializationOptions(),
	}
	b, _ := json.Marshal(expected)

	t.Setenv(initOptionsFakeLSPEnv, "1")
	t.Setenv(initOptionsExpectedJSONEnv, string(b))

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := NewClient(execPath, "-test.run=TestHelperProcessInitializeInitOptionsFakeLSP")
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

	if err := client.SetInitializationOptions(inputOptions); err != nil {
		t.Fatalf("expected SetInitializationOptions to normalize empty entries, got error: %v", err)
	}

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		t.Fatalf("expected InitializeLSPClient to normalize and send initializationOptions, got error: %v", err)
	}
}

func TestInitializeLSPClient_SendsInitializationOptions_WhenFlagsExpandedFromSemicolon(t *testing.T) {
	// This test ensures the client sends an array payload to the LSP even when
	// the original MCP flags would have been expanded from a semicolon-separated value.
	workspaceDir := `C:\Users\dave\repo`
	inputOptions := InitializeOptions{
		SearchPaths: []string{"src", "lib", "vendor"},
	}

	expected := map[string]any{
		"searchPaths": []string{"src", "lib", "vendor"},
		"codelenses":  defaultCodeLensesInitializationOptions(),
	}
	b, _ := json.Marshal(expected)

	t.Setenv(initOptionsFakeLSPEnv, "1")
	t.Setenv(initOptionsExpectedJSONEnv, string(b))

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := NewClient(execPath, "-test.run=TestHelperProcessInitializeInitOptionsFakeLSP")
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

	if err := client.SetInitializationOptions(inputOptions); err != nil {
		t.Fatalf("expected SetInitializationOptions to accept options, got error: %v", err)
	}

	if _, err := client.InitializeLSPClient(ctx, workspaceDir); err != nil {
		t.Fatalf("expected InitializeLSPClient to send initializationOptions, got error: %v", err)
	}
}

// Fake LSP implementation that inspects the initialize params and validates
// the presence and shape of initializationOptions according to expectations
func runInitializeInitOptionsFakeLSP(stdin *os.File, stdout *os.File) {
	reader := bufio.NewReader(stdin)
	writer := stdout

	// read expected JSON from env
	expectedJSON := os.Getenv(initOptionsExpectedJSONEnv)
	var expected any
	if expectedJSON != "" {
		_ = json.Unmarshal([]byte(expectedJSON), &expected)
	}

	for {
		msg, err := ReadMessage(reader)
		if err != nil {
			return
		}

		switch msg.Method {
		case "initialize":
			var params map[string]any
			_ = json.Unmarshal(msg.Params, &params)

			// Ensure workspaceFolders present
			wfOK := false
			if v, ok := params["workspaceFolders"]; ok {
				if arr, ok := v.([]any); ok && len(arr) > 0 {
					wfOK = true
				}
			}
			if !wfOK {
				sendInitOptionsFakeResponse(writer, msg.ID, nil, &ResponseError{Code: -32602, Message: "missing workspaceFolders"})
				continue
			}

			initOptsRaw, ok := params["initializationOptions"]
			if !ok {
				sendInitOptionsFakeResponse(writer, msg.ID, nil, &ResponseError{Code: -32602, Message: "missing initializationOptions"})
				continue
			}

			initOpts, ok := initOptsRaw.(map[string]any)
			if !ok {
				sendInitOptionsFakeResponse(writer, msg.ID, nil, &ResponseError{Code: -32602, Message: "initializationOptions must be object"})
				continue
			}

			// compare against expected if provided
			if expected != nil {
				if !deepEqualJSONLike(initOpts, expected) {
					sendInitOptionsFakeResponse(writer, msg.ID, nil, &ResponseError{Code: -32602, Message: fmt.Sprintf("initializationOptions mismatch: got=%v expected=%v", initOpts, expected)})
					continue
				}
			}

			// basic validation: searchPaths not a single string with semicolons
			if sp, ok := initOpts["searchPaths"]; ok {
				switch typed := sp.(type) {
				case string:
					if strings.Contains(typed, ";") {
						sendInitOptionsFakeResponse(writer, msg.ID, nil, &ResponseError{Code: -32602, Message: "searchPaths must be an array, not semicolon-separated string"})
						continue
					}
				}
			}

			result := map[string]any{"capabilities": map[string]any{}}
			sendInitOptionsFakeResponse(writer, msg.ID, result, nil)

		case "initialized":
			// no-op
		case "shutdown":
			sendInitOptionsFakeResponse(writer, msg.ID, nil, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendInitOptionsFakeResponse(writer, msg.ID, nil, nil)
			}
		}
	}
}

func sendInitOptionsFakeResponse(w *os.File, id *MessageID, result any, rpcErr *ResponseError) {
	resp := &Message{JSONRPC: "2.0", ID: id, Error: rpcErr}
	if rpcErr == nil {
		if result == nil {
			resp.Result = mustMarshalInitOptionsFake(struct{}{})
		} else {
			resp.Result = mustMarshalInitOptionsFake(result)
		}
	}
	_ = WriteMessage(w, resp)
}

func mustMarshalInitOptionsFake(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal fake LSP json: %v", err))
	}
	return b
}

// deepEqualJSONLike compares two values in a JSON-like loose manner (maps/arrays)
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
