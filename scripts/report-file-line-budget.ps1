# Relatorio de arquivos acima do limite de linhas (default 500). Exit 1 se violacao sem excecao documentada.
param(
    [string]$Root,
    [int]$MaxLines = 500,
    [string]$ExceptionsFile = ""
)

$ErrorActionPreference = "Stop"
if (-not $Root) {
    $Root = (Get-Location).Path
}
$Root = (Resolve-Path $Root).Path

$allowed = @{}
if ($ExceptionsFile -and (Test-Path $ExceptionsFile)) {
    Get-Content $ExceptionsFile -Encoding UTF8 | ForEach-Object {
        $t = $_.Trim()
        if ($t -and -not $t.StartsWith("#")) {
            $path = ($t -split '\s*\|\s*', 2)[0].Trim().Replace("\", "/")
            if ($path) { $allowed[$path] = $true }
        }
    }
}

$extensions = @(".go")
$violations = @()
$reported = @()

Get-ChildItem -Path $Root -Recurse -File | ForEach-Object {
    $rel = $_.FullName.Substring($Root.Length).TrimStart("\", "/").Replace("\", "/")
    if ($rel -match '(^|/)(target/|node_modules/|dist/|\.git/|third_party/)') {
        return
    }
    $ext = $_.Extension.ToLowerInvariant()
    if ($extensions -notcontains $ext) {
        return
    }
    $lineCount = (Get-Content $_.FullName -ErrorAction SilentlyContinue | Measure-Object -Line).Lines
    if ($lineCount -le $MaxLines) {
        return
    }
    $entry = @{ Path = $rel; Lines = $lineCount }
    $reported += $entry
    if (-not $allowed.ContainsKey($rel)) {
        $violations += $entry
    }
}

Write-Host "FILE_LINE_BUDGET root=$Root max=$MaxLines"
$reported | Sort-Object Lines -Descending | ForEach-Object {
    $tag = if ($allowed.ContainsKey($_.Path)) { "EXCEPT" } else { "OVER" }
    Write-Host ("  [{0}] {1,6} lines  {2}" -f $tag, $_.Lines, $_.Path)
}

if ($violations.Count -gt 0) {
    Write-Host "FILE_LINE_BUDGET FAIL: $($violations.Count) arquivo(s) acima de $MaxLines sem excecao" -ForegroundColor Red
    exit 1
}

Write-Host "FILE_LINE_BUDGET OK ($($reported.Count) arquivo(s) > $MaxLines, todos com excecao ou nenhum)"
exit 0
