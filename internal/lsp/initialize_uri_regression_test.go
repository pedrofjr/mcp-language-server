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

const initializeURIFakeLSPEnv = "MCP_FAKE_LSP_INITIALIZE_URI_REGRESSION"
const initializeExpectedURIEnv = "MCP_FAKE_LSP_INITIALIZE_EXPECTED_URI"

func TestHelperProcessInitializeURIFakeLSP(t *testing.T) {
	if os.Getenv(initializeURIFakeLSPEnv) != "1" {
		return
	}

	runInitializeURIFakeLSP(os.Stdin, os.Stdout)
	os.Exit(0)
}

func TestInitializeLSPClient_SendsCanonicalWorkspaceURIs(t *testing.T) {
	workspaceDir := `C:\Users\alice\workspace`
	expectedURI := string(protocol.URIFromPath(workspaceDir))

	t.Setenv(initializeURIFakeLSPEnv, "1")
	t.Setenv(initializeExpectedURIEnv, expectedURI)

	execPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}

	client, err := NewClient(execPath, "-test.run=TestHelperProcessInitializeURIFakeLSP")
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
		t.Fatalf("expected InitializeLSPClient to send canonical RootURI and WorkspaceFolders URI, got error: %v", err)
	}
}

func runInitializeURIFakeLSP(stdin *os.File, stdout *os.File) {
	reader := bufio.NewReader(stdin)
	writer := stdout
	expectedURI := os.Getenv(initializeExpectedURIEnv)

	for {
		msg, err := ReadMessage(reader)
		if err != nil {
			return
		}

		switch msg.Method {
		case "initialize":
			var params struct {
				RootURI          string `json:"rootUri"`
				WorkspaceFolders []struct {
					URI string `json:"uri"`
				} `json:"workspaceFolders"`
			}
			_ = json.Unmarshal(msg.Params, &params)

			if params.RootURI != expectedURI {
				sendInitializeURIFakeResponse(writer, msg.ID, nil, &ResponseError{
					Code:    -32602,
					Message: fmt.Sprintf("invalid rootUri: got=%s expected=%s", params.RootURI, expectedURI),
				})
				continue
			}

			if strings.Contains(params.RootURI, "\\") {
				sendInitializeURIFakeResponse(writer, msg.ID, nil, &ResponseError{
					Code:    -32602,
					Message: fmt.Sprintf("rootUri contains backslash: %s", params.RootURI),
				})
				continue
			}

			if len(params.WorkspaceFolders) != 1 || params.WorkspaceFolders[0].URI != expectedURI {
				got := "<missing>"
				if len(params.WorkspaceFolders) > 0 {
					got = params.WorkspaceFolders[0].URI
				}
				sendInitializeURIFakeResponse(writer, msg.ID, nil, &ResponseError{
					Code:    -32602,
					Message: fmt.Sprintf("invalid workspaceFolders[0].uri: got=%s expected=%s", got, expectedURI),
				})
				continue
			}

			if strings.Contains(params.WorkspaceFolders[0].URI, "\\") {
				sendInitializeURIFakeResponse(writer, msg.ID, nil, &ResponseError{
					Code:    -32602,
					Message: fmt.Sprintf("workspaceFolders[0].uri contains backslash: %s", params.WorkspaceFolders[0].URI),
				})
				continue
			}

			result := map[string]any{
				"capabilities": map[string]any{},
			}
			sendInitializeURIFakeResponse(writer, msg.ID, result, nil)
		case "initialized":
			// no-op
		case "shutdown":
			sendInitializeURIFakeResponse(writer, msg.ID, nil, nil)
		case "exit":
			return
		default:
			if msg.ID != nil && msg.ID.Value != nil {
				sendInitializeURIFakeResponse(writer, msg.ID, nil, nil)
			}
		}
	}
}

func sendInitializeURIFakeResponse(w *os.File, id *MessageID, result any, rpcErr *ResponseError) {
	resp := &Message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
	}

	if rpcErr == nil {
		if result == nil {
			resp.Result = mustMarshalInitializeURIFake(struct{}{})
		} else {
			resp.Result = mustMarshalInitializeURIFake(result)
		}
	}

	_ = WriteMessage(w, resp)
}

func mustMarshalInitializeURIFake(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal fake LSP json: %v", err))
	}
	return b
}
