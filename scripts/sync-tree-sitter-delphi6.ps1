# Sincroniza o parser Delphi vendored em third_party/ a partir do monorepo Oracle (opcional).
param(
    [string]$SourceRoot = ""
)

$repoRoot = Split-Path $PSScriptRoot -Parent
if ([string]::IsNullOrWhiteSpace($SourceRoot)) {
    $SourceRoot = Join-Path (Split-Path $repoRoot -Parent) "Delphi_Oracle\tree-sitter-delphi6"
}

$ErrorActionPreference = "Stop"
$dest = Join-Path $repoRoot "third_party\tree-sitter-delphi6"

if (-not (Test-Path (Join-Path $SourceRoot "src\parser.c"))) {
    Write-Error "Fonte nao encontrada: $SourceRoot (defina -SourceRoot se o layout do monorepo for diferente)"
}

New-Item -ItemType Directory -Force -Path $dest | Out-Null
Remove-Item -Recurse -Force (Join-Path $dest "src"), (Join-Path $dest "bindings") -ErrorAction SilentlyContinue
Copy-Item -Recurse -Force (Join-Path $SourceRoot "src") (Join-Path $dest "src")
Copy-Item -Recurse -Force (Join-Path $SourceRoot "bindings\go") (Join-Path $dest "bindings\go")

Write-Host "Sincronizado: $SourceRoot -> $dest"
