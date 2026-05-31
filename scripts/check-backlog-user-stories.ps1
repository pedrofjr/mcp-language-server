# Valida itens abertos em backlog_*.md do repo MCP (quando presentes no monorepo).
param(
    [string]$McpRoot = (Split-Path $PSScriptRoot -Parent)
)

$ErrorActionPreference = "Stop"
$delphiOracle = Join-Path (Split-Path $McpRoot -Parent) "Delphi_Oracle"
$shared = Join-Path $delphiOracle "scripts\check-backlog-user-stories.ps1"
if (Test-Path $shared) {
    & $shared
    exit $LASTEXITCODE
}

Write-Host "BACKLOG_USER_STORY_CHECK OK (MCP: nenhum backlog filho local; gate delegado ao monorepo quando presente)"
exit 0
