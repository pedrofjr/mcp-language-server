# Fixtures negativas: edit_file sem mutacao e safe_delete com simbolo ainda em disco.
$ErrorActionPreference = "Stop"
$harness = Join-Path $PSScriptRoot "mcp-tools-harness.mjs"

$workspaceDir = Join-Path $env:TEMP ("mcp-harness-neg-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $workspaceDir -Force | Out-Null

function Invoke-HarnessExpectFail([string]$tool, [string]$argsJson, [string]$pattern) {
    $argsFile = Join-Path $env:TEMP ("mcp-neg-" + [Guid]::NewGuid().ToString("N") + ".json")
    [System.IO.File]::WriteAllText($argsFile, $argsJson, [System.Text.UTF8Encoding]::new($false))
    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $out = & node $harness call --workspace $workspaceDir --tool $tool --args-file $argsFile --timeout-ms 60000 2>&1 | Out-String
    $code = $LASTEXITCODE
    $ErrorActionPreference = $prevEap
    if ($code -eq 0) {
        Write-Host "MCP_HARNESS_NEG FAIL: $tool deveria falhar (exit 0)" -ForegroundColor Red
        Write-Host $out
        exit 1
    }
    if ($pattern -and $out -notmatch $pattern) {
        Write-Host "MCP_HARNESS_NEG FAIL: $tool sem mensagem esperada ($pattern)" -ForegroundColor Red
        Write-Host $out
        exit 1
    }
}

try {
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
    $editJson = '{"filePath":"' + $editAbs + '","edits":[{"startLine":5,"startColumn":1,"endLine":5,"endColumn":20,"newText":"  Marker = 1;"}]}'
    Invoke-HarnessExpectFail "edit_file" $editJson "Marker = 42|mutacao"

    & node $harness --test-safe-delete-negative
    if ($LASTEXITCODE -ne 0) {
        Write-Host "MCP_HARNESS_NEG FAIL: --test-safe-delete-negative" -ForegroundColor Red
        exit 1
    }
}
finally {
    Remove-Item -Recurse -Force $workspaceDir -ErrorAction SilentlyContinue
}

Write-Host "MCP_HARNESS_NEG OK"
exit 0
