# Gates de estilo MCP: gofmt (changed .go only) + go vet + orcamento de linhas.
$ErrorActionPreference = "Stop"
$mcpRoot = Split-Path $PSScriptRoot -Parent
$exceptions = Join-Path $mcpRoot "docs\file-size-exceptions.txt"
$reportLines = Join-Path $mcpRoot "scripts\report-file-line-budget.ps1"
$checkBacklog = Join-Path $mcpRoot "scripts\check-backlog-user-stories.ps1"
if (-not (Test-Path $exceptions)) {
    $delphiOracle = Join-Path (Split-Path $mcpRoot -Parent) "Delphi_Oracle"
    $exceptions = Join-Path $delphiOracle "docs\file-size-exceptions.txt"
}
$git = "git"

Set-Location $mcpRoot

Write-Host "=== gofmt (changed .go files only) ==="
$prevEap = $ErrorActionPreference
$ErrorActionPreference = "Continue"
$changed = @()
$changed += (& git diff --name-only HEAD 2>&1 | Where-Object { $_ -and $_ -notmatch '^\s*warning:' })
$changed += (& git diff --name-only --cached 2>&1 | Where-Object { $_ -and $_ -notmatch '^\s*warning:' })
$ErrorActionPreference = $prevEap
$goFiles = $changed | Where-Object { $_ -and $_ -match '\.go$' } | Select-Object -Unique
$fmtBad = @()
foreach ($rel in $goFiles) {
    $full = Join-Path $mcpRoot $rel
    if (-not (Test-Path $full)) { continue }
    $out = & gofmt -l $full 2>$null
    if ($out) { $fmtBad += $out }
}
if ($fmtBad.Count -gt 0) {
    Write-Host "gofmt would change:" -ForegroundColor Red
    $fmtBad | ForEach-Object { Write-Host "  $_" }
    exit 1
}

Write-Host "=== go vet ==="
& go vet ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "=== backlog user stories ==="
& $checkBacklog
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "=== file line budget (MCP root, documented exceptions) ==="
& $reportLines -Root $mcpRoot -ExceptionsFile $exceptions
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "MCP validate-style OK"
exit 0
