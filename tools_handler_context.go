package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// criticalLSPHandlerTimeout limita operações LSP-backed críticas por request.
var criticalLSPHandlerTimeout = 15 * time.Second

var runQueryHandlerTimeout = criticalLSPHandlerTimeout

var definitionReferencesHandlerTimeout = criticalLSPHandlerTimeout

func handlerOperationContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, criticalLSPHandlerTimeout)
}

func opTokenForToolEvent(toolName, event string) string {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(toolName), "-", "_"))
	return fmt.Sprintf("OP_%s_%s", normalized, event)
}

func deterministicHandlerContextError(toolName string, opCtx context.Context, err error) *mcp.CallToolResult {
	if errors.Is(err, context.Canceled) || (opCtx != nil && errors.Is(opCtx.Err(), context.Canceled)) {
		token := opTokenForToolEvent(toolName, "CANCELED")
		msg := fmt.Sprintf("failed: %s canceled: context canceled | action: retry when context is active", toolName)
		return mcp.NewToolResultError(opErrMsgWithRecovery(token, msg))
	}

	if errors.Is(err, context.DeadlineExceeded) || (opCtx != nil && errors.Is(opCtx.Err(), context.DeadlineExceeded)) {
		token := opTokenForToolEvent(toolName, "DEADLINE")
		msg := fmt.Sprintf("failed: %s deadline exceeded: context deadline exceeded | action: retry with longer timeout", toolName)
		return mcp.NewToolResultError(opErrMsgWithRecovery(token, msg))
	}

	return nil
}

func deterministicDefinitionReferencesContextError(toolName string, opCtx context.Context, err error) *mcp.CallToolResult {
	return deterministicHandlerContextError(toolName, opCtx, err)
}

func handleLSPBackedToolError(toolName string, opCtx context.Context, err error, action string) (*mcp.CallToolResult, error) {
	if err == nil {
		return nil, nil
	}
	if deterministic := deterministicHandlerContextError(toolName, opCtx, err); deterministic != nil {
		return deterministic, nil
	}
	return OpToolFailedError(toolName, err.Error(), action)
}
