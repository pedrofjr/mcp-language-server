# Fixtures negativas: rename sem WorkspaceEdit e edit_file sem mutacao em disco.
$ErrorActionPreference = "Stop"
$harness = Join-Path $PSScriptRoot "mcp-tools-harness.mjs"
$root = Split-Path $PSScriptRoot -Parent

$workspaceDir = Join-Path $env:TEMP ("mcp-harness-neg-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $workspaceDir -Force | Out-Null
$editTarget = Join-Path $workspaceDir "edit-target.pas"
@(
    "unit EditHarness;",
    "interface",
    "implementation",
    "const",
    "  Marker = 1;",
    "end."
) | Set-Content -Path $editTarget -Encoding UTF8
$editAbs = $editTarget.Replace('\', '/')

try {
    $argsFile = Join-Path $env:TEMP ("mcp-neg-args-" + [Guid]::NewGuid().ToString("N") + ".json")
    '{"filePath":"' + $editAbs + '","edits":[{"startLine":5,"startColumn":1,"endLine":5,"endColumn":20,"newText":"  Marker = 1;"}]}' | Set-Content -Path $argsFile -Encoding UTF8
    $out = & node $harness call --workspace $workspaceDir --tool edit_file --args-file $argsFile --timeout-ms 60000 2>&1 | Out-String
    if ($LASTEXITCODE -eq 0) {
        Write-Host "MCP_HARNESS_NEG FAIL: edit_file deveria falhar sem Marker = 42" -ForegroundColor Red
        Write-Host $out
        exit 1
    }
    if ($out -notmatch 'Marker = 42|mutacao') {
        Write-Host "MCP_HARNESS_NEG FAIL: edit_file negativo sem mensagem de mutacao" -ForegroundColor Red
        Write-Host $out
        exit 1
    }
}
finally {
    Remove-Item -Recurse -Force $workspaceDir -ErrorAction SilentlyContinue
}

Write-Host "MCP_HARNESS_NEG OK"
exit 0
