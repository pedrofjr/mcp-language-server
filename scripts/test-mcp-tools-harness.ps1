# Smoke MCP harness: tools/list + tools/call LSP-backed criticas.
$ErrorActionPreference = "Stop"
$root = Split-Path $PSScriptRoot -Parent
$harness = Join-Path $PSScriptRoot "mcp-tools-harness.mjs"
$fixture = Join-Path $root "test-fixtures\harness-cli\smoke.pas"
if (-not (Test-Path $fixture)) {
    Write-Host "MCP_HARNESS_TEST FAIL: fixture $fixture ausente" -ForegroundColor Red
    exit 1
}
$fixtureAbs = (Resolve-Path $fixture).Path.Replace('\', '/')

& node $harness list --workspace $root
if ($LASTEXITCODE -ne 0) {
    Write-Host "MCP_HARNESS_TEST FAIL: list" -ForegroundColor Red
    exit 1
}

$argsDir = Join-Path $env:TEMP ("mcp-harness-args-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $argsDir -Force | Out-Null
try {
    $calls = @(
        @("run_query", '{"query":"class"}'),
        @("hover", ('{"filePath":"' + $fixtureAbs + '","line":8,"column":4}'))
    )
    foreach ($pair in $calls) {
        $tool = $pair[0]
        $argsFile = Join-Path $argsDir ($tool + ".json")
        [System.IO.File]::WriteAllText($argsFile, $pair[1])
        & node $harness call --workspace $root --tool $tool --args-file $argsFile --timeout-ms 120000
        if ($LASTEXITCODE -ne 0) {
            Write-Host "MCP_HARNESS_TEST FAIL: tools/call $tool" -ForegroundColor Red
            exit 1
        }
    }
}
finally {
    Remove-Item -Recurse -Force $argsDir -ErrorAction SilentlyContinue
}

Write-Host "MCP_HARNESS_TEST OK (list + tools/call run_query, hover)"
exit 0
