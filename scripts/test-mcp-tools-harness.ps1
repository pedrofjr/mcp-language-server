# Smoke do harness MCP (fake LSP minimal por default).
$ErrorActionPreference = "Stop"
$root = Split-Path $PSScriptRoot -Parent
$harness = Join-Path $PSScriptRoot "mcp-tools-harness.mjs"

& node $harness list --workspace $root
if ($LASTEXITCODE -ne 0) {
    Write-Host "MCP_HARNESS_TEST FAIL: list (default --lsp fake)" -ForegroundColor Red
    exit 1
}

Write-Host "MCP_HARNESS_TEST OK"
exit 0
