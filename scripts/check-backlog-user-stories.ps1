# Autocontido: valida user stories em backlog_mcp (sibling Delphi_Oracle ou local).
param(
    [string]$McpRoot = (Split-Path $PSScriptRoot -Parent)
)

$ErrorActionPreference = "Stop"
$failures = @()
$files = @()
$sibling = Join-Path (Split-Path $McpRoot -Parent) "Delphi_Oracle\backlog_mcp.md"
if (Test-Path $sibling) { $files += $sibling }
$local = Join-Path $McpRoot "backlog_mcp.md"
if (Test-Path $local) { $files += $local }

if ($files.Count -eq 0) {
    Write-Host "BACKLOG_USER_STORY_CHECK OK (MCP: nenhum backlog_mcp.md no clone isolado)"
    exit 0
}

foreach ($file in $files) {
    $lines = Get-Content -Path $file -Encoding UTF8
    for ($i = 0; $i -lt $lines.Count; $i++) {
        $line = $lines[$i]
        if ($line -notmatch '^\s*-\s+\[ \]\s+\*\*') { continue }
        $title = $line -replace '^\s*-\s+\[ \]\s+\*\*', '' -replace '\*\*\s*$', ''
        if ($title -notmatch 'Como\s+' -or $title -notmatch 'quero\s+') {
            $failures += "${file}:$($i + 1): titulo aberto sem user story: $title"
        }
    }
}

if ($failures.Count -gt 0) {
    Write-Host "BACKLOG_USER_STORY_CHECK FAIL ($($failures.Count)):" -ForegroundColor Red
    $failures | ForEach-Object { Write-Host "  $_" }
    exit 1
}

Write-Host "BACKLOG_USER_STORY_CHECK OK ($($files.Count) arquivo(s))"
exit 0
