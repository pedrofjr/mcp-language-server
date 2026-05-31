# Autocontido: valida user stories em backlog_mcp versionado neste repo.
param(
    [string]$McpRoot = (Split-Path $PSScriptRoot -Parent)
)

$ErrorActionPreference = "Stop"
$local = Join-Path $McpRoot "backlog_mcp.md"

if (-not (Test-Path $local)) {
    Write-Host "BACKLOG_USER_STORY_CHECK FAIL: versione backlog_mcp.md na raiz do MCP ou execute gate no monorepo Delphi_Oracle" -ForegroundColor Red
    exit 1
}

$failures = @()
$lines = Get-Content -Path $local -Encoding UTF8
for ($i = 0; $i -lt $lines.Count; $i++) {
    $line = $lines[$i]
    if ($line -notmatch '^\s*-\s+\[ \]\s+\*\*') { continue }
    $title = $line -replace '^\s*-\s+\[ \]\s+\*\*', '' -replace '\*\*\s*$', ''
    if ($title -notmatch 'Como\s+' -or $title -notmatch 'quero\s+') {
        $failures += "${local}:$($i + 1): titulo aberto sem user story: $title"
    }
}

if ($failures.Count -gt 0) {
    Write-Host "BACKLOG_USER_STORY_CHECK FAIL ($($failures.Count)):" -ForegroundColor Red
    $failures | ForEach-Object { Write-Host "  $_" }
    exit 1
}

Write-Host "BACKLOG_USER_STORY_CHECK OK (backlog_mcp.md local)"
exit 0
