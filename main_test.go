package main

import (
	"context"
	"encoding/json"
	"testing"

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
