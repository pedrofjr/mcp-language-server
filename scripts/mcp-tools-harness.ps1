# Wrapper CLI First para mcp-tools-harness.mjs
param(
    [Parameter(Position = 0)]
    [ValidateSet("list", "call", "")]
    [string]$Subcommand = "",
    [switch]$Help,
    [string]$Workspace = "",
    [string]$Tool = "",
    [string]$ArgsJson = "{}",
    [int]$TimeoutMs = 120000
)

$ErrorActionPreference = "Stop"
$nodeScript = Join-Path $PSScriptRoot "mcp-tools-harness.mjs"
$nodeArgs = @($nodeScript)

if ($Help -or -not $Subcommand) {
    & node $nodeScript --help
    exit $(if ($Help) { 0 } else { 1 })
}

$nodeArgs += $Subcommand
if ($Workspace) { $nodeArgs += @("--workspace", $Workspace) }
if ($TimeoutMs) { $nodeArgs += @("--timeout-ms", $TimeoutMs) }
if ($Subcommand -eq "call") {
    $nodeArgs += @("--tool", $Tool, "--args-json", $ArgsJson)
}

& node @nodeArgs
exit $LASTEXITCODE
