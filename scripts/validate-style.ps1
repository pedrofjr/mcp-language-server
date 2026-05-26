# Gates de estilo MCP: gofmt + go vet + orcamento de linhas.
$ErrorActionPreference = "Stop"
$root = Split-Path $PSScriptRoot -Parent
$delphiOracle = Join-Path (Split-Path $root -Parent) "Delphi_Oracle"
$exceptions = Join-Path $delphiOracle "docs\file-size-exceptions.txt"
$reportLines = Join-Path $delphiOracle "scripts\report-file-line-budget.ps1"
$checkBacklog = Join-Path $delphiOracle "scripts\check-backlog-user-stories.ps1"

Set-Location $root
Write-Host "=== gofmt (product sources) ==="
$fmtOut = & gofmt -l (Get-ChildItem -Path $root -Recurse -Filter "*.go" -File |
    Where-Object { $_.FullName -notmatch '\\test-output\\|\\third_party\\tree-sitter-delphi6\\' } |
    ForEach-Object { $_.FullName })
if ($fmtOut) {
    Write-Host "gofmt would change:" -ForegroundColor Red
    $fmtOut | ForEach-Object { Write-Host "  $_" }
    exit 1
}

Write-Host "=== go vet ==="
Push-Location $root
try {
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
    Pop-Location
}

Write-Host "=== backlog user stories ==="
& $checkBacklog
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "=== file line budget (documented exceptions) ==="
& $reportLines -Root $delphiOracle -ExceptionsFile $exceptions
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "MCP validate-style OK"
exit 0
