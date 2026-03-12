# Project Guidelines

## Code Style

- This repository targets Go 1.24.0. Keep changes compatible with the version declared in go.mod.
- Prefer the recipes in justfile over ad hoc shell commands: use `just fmt`, `just test`, `just check`, `just build`, and `just generate`.
- Follow the existing package split under `internal/` and keep changes local to the layer that owns the behavior.
- Use the component logger from `internal/logging` instead of introducing direct use of the standard library logger.
- Keep MCP tool output stable and deliberate. Integration snapshots assert exact output shape, including file paths, ranges, and formatted text blocks.

## Architecture

- `main.go` parses CLI flags, validates the workspace and LSP command, starts the LSP client, registers MCP tools, and owns shutdown behavior.
- `internal/lsp/` manages the language server subprocess, JSON-RPC/LSP transport, request handlers, and language detection.
- `internal/protocol/` contains generated LSP protocol types and compatibility helpers. Prefer regeneration over manual edits when changing protocol surface.
- `internal/tools/` implements the MCP-facing tools such as definition, references, diagnostics, hover, rename, and file edits.
- `internal/utilities/` contains lower-level helpers such as text edit application and line-ending preservation.
- `internal/watcher/` tracks workspace file changes and syncs them with the connected language server.
- `integrationtests/` runs real language servers against fixture workspaces and validates results with snapshots.

## Build and Test

- Run `just fmt` before finalizing Go changes.
- Run `just test` for normal verification.
- Run `just check` when touching shared infrastructure, generated code, or anything likely to affect repo-wide quality gates.
- Run `just generate` after changes to protocol generation inputs in `cmd/generate/`, `internal/protocol/`, or generated LSP methods.
- Run `just snapshot` only when an output change is intentional. Review snapshot diffs instead of updating them mechanically.
- Snapshot and integration tests require the relevant language servers to be installed locally: `gopls`, `pyright`, `rust-analyzer`, `typescript-language-server`, and `clangd` depending on the suite being exercised.

## Conventions

- Do not hand-edit generated protocol code unless the change is deliberately outside the generator flow; prefer updating generator inputs and regenerating.
- Preserve CRLF/LF handling when working on file edit logic. The utilities layer is designed to keep original line endings intact.
- Keep logging changes aligned with existing component names and `LOG_LEVEL` behavior so debugging remains consistent across packages.
- Integration tests copy fixture workspaces into temporary directories and normalize paths in snapshots. When investigating failures, check `integrationtests/test-output/` and any generated `.diff` files.
- The server expects a real workspace path and an LSP executable available on `PATH`. Avoid changes that weaken those validations unless the behavior is intentionally being redesigned.