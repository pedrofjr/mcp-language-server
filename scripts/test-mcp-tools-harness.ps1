# Smoke MCP harness: tools/list + tools/call em tools criticas.
$ErrorActionPreference = "Stop"
$root = Split-Path $PSScriptRoot -Parent
$harness = Join-Path $PSScriptRoot "mcp-tools-harness.mjs"

& node $harness list --workspace $root
if ($LASTEXITCODE -ne 0) {
    Write-Host "MCP_HARNESS_TEST FAIL: list" -ForegroundColor Red
    exit 1
}

$criticalCalls = @(
    @{ tool = "onboarding"; args = '{}' },
    @{ tool = "get_node_types"; args = '{}' }
)

foreach ($call in $criticalCalls) {
    & node $harness call --workspace $root --tool $call.tool --args-json $call.args --timeout-ms 120000
    if ($LASTEXITCODE -ne 0) {
        Write-Host "MCP_HARNESS_TEST FAIL: tools/call $($call.tool)" -ForegroundColor Red
        exit 1
    }
}

Write-Host "MCP_HARNESS_TEST OK (list + tools/call onboarding, get_node_types)"
exit 0
