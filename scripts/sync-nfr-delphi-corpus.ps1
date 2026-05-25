# Sincroniza o corpus Delphi vendored em third_party/ a partir do monorepo Oracle (opcional).
param(
    [string]$SourceRoot = ""
)

$repoRoot = Split-Path $PSScriptRoot -Parent
if ([string]::IsNullOrWhiteSpace($SourceRoot)) {
    $SourceRoot = Join-Path (Split-Path $repoRoot -Parent) "Delphi_Oracle\oracle-lsp\test-fixtures"
}

$ErrorActionPreference = "Stop"
$dest = Join-Path $repoRoot "third_party\nfr-delphi-corpus"

if (-not (Test-Path $SourceRoot)) {
    Write-Error "Fonte nao encontrada: $SourceRoot (defina -SourceRoot se o layout do monorepo for diferente)"
}

if (Test-Path $dest) {
    Remove-Item -Recurse -Force $dest
}
New-Item -ItemType Directory -Force -Path $dest | Out-Null
Copy-Item -Path (Join-Path $SourceRoot "*") -Destination $dest -Recurse -Force

Write-Host "Sincronizado: $SourceRoot -> $dest"
