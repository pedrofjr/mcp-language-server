package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	toolOutcomeSuccess = "success"
	toolOutcomeError   = "error"
)

var toolCallSeq atomic.Uint64

type toolObsContextKey struct{}

type toolObsContext struct {
	WorkspaceDir string
}

// criticalMCPTools lists tools that must emit the full structured log contract (v1.1+ P3/NFR).
var criticalMCPTools = map[string]struct{}{
	"edit_file":                  {},
	"definition":                 {},
	"references":                 {},
	"diagnostics":                {},
	"hover":                      {},
	"rename_symbol":              {},
	"run_query":                  {},
	"semantic_search":            {},
	"code_actions":               {},
	"dependency_tree":            {},
	"memory_write":               {},
	"memory_read":                {},
	"memory_list":                {},
	"onboarding":                 {},
	"check_onboarding_performed": {},
	"get_node_at_position":       {},
	"workspace_symbols":          {},
	"graph_query":                {},
	"replace_symbol_body":        {},
	"safe_delete_symbol":         {},
}

func contextWithToolObs(ctx context.Context, workspaceDir string) context.Context {
	return context.WithValue(ctx, toolObsContextKey{}, toolObsContext{WorkspaceDir: workspaceDir})
}

func workspaceDirFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(toolObsContextKey{}).(toolObsContext); ok {
		return v.WorkspaceDir
	}
	return ""
}

func toolObservabilityMiddleware(workspaceDir string) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return next(contextWithToolObs(ctx, workspaceDir), req)
		}
	}
}

func formatRequestID(req mcp.CallToolRequest) string {
	if req.Params.Meta != nil {
		switch token := req.Params.Meta.ProgressToken.(type) {
		case string:
			if strings.TrimSpace(token) != "" {
				return "pt:" + strings.TrimSpace(token)
			}
		case float64:
			return fmt.Sprintf("pt:%v", token)
		case int:
			return fmt.Sprintf("pt:%d", token)
		case int64:
			return fmt.Sprintf("pt:%d", token)
		}
	}
	return fmt.Sprintf("seq:%d", toolCallSeq.Add(1))
}

func extractProjectPath(req mcp.CallToolRequest, fallbackWorkspace string) string {
	if req.Params.Arguments == nil {
		return fallbackWorkspace
	}
	args := req.Params.Arguments

	for _, key := range []string{"projectPath", "project_path", "workspace", "workspaceDir", "workspace_dir"} {
		if v, ok := args[key].(string); ok {
			if trimmed := strings.TrimSpace(v); trimmed != "" {
				if abs, err := filepath.Abs(trimmed); err == nil {
					return abs
				}
				return trimmed
			}
		}
	}

	for _, key := range []string{"filePath", "file_path", "path", "uri"} {
		v, ok := args[key].(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "file:") {
			if path := protocol.DocumentUri(trimmed).Path(); path != "" {
				if abs, err := filepath.Abs(path); err == nil {
					return filepath.Dir(abs)
				}
				return filepath.Dir(path)
			}
		}
		if abs, err := filepath.Abs(trimmed); err == nil {
			return filepath.Dir(abs)
		}
		return filepath.Dir(trimmed)
	}

	return fallbackWorkspace
}

func classifyToolOutcome(result *mcp.CallToolResult, err error) string {
	if err != nil || (result != nil && result.IsError) {
		return toolOutcomeError
	}
	return toolOutcomeSuccess
}

func logStructuredToolEvent(tool, outcome string, durationMs int64, requestID, projectPath string) {
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	line := fmt.Sprintf(
		"tool=%s outcome=%s duration_ms=%d request_id=%s project_path=%s timestamp=%s",
		tool, outcome, durationMs, requestID, projectPath, timestamp,
	)
	if outcome == toolOutcomeError {
		coreLogger.Error(line)
	} else {
		coreLogger.Info(line)
	}
}

func isCriticalMCPTool(name string) bool {
	_, ok := criticalMCPTools[name]
	return ok
}
