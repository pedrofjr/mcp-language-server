# Smoke MCP harness: tools/list + tools/call criticas em workspace temporario hermetico.
$ErrorActionPreference = "Stop"
$root = Split-Path $PSScriptRoot -Parent
$harness = Join-Path $PSScriptRoot "mcp-tools-harness.mjs"
$fixtureSrc = Join-Path $root "test-fixtures\harness-cli\smoke.pas"
$symbolMutateSrc = Join-Path $root "test-fixtures\harness-cli\symbol_mutate.pas"
if (-not (Test-Path $fixtureSrc)) {
    Write-Host "MCP_HARNESS_TEST FAIL: fixture $fixtureSrc ausente" -ForegroundColor Red
    exit 1
}

$repoSnapshot = Get-Content -Path $fixtureSrc -Raw
$workspaceDir = Join-Path $env:TEMP ("mcp-harness-ws-" + [Guid]::NewGuid().ToString("N"))
$argsDir = Join-Path $env:TEMP ("mcp-harness-args-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $workspaceDir, $argsDir -Force | Out-Null

try {
    Copy-Item -Path $fixtureSrc -Destination (Join-Path $workspaceDir "smoke.pas") -Force
    if (Test-Path $symbolMutateSrc) {
        Copy-Item -Path $symbolMutateSrc -Destination (Join-Path $workspaceDir "symbol_mutate.pas") -Force
    }
    $fixtureAbs = (Join-Path $workspaceDir "smoke.pas").Replace('\', '/')
    $symbolMutateAbs = (Join-Path $workspaceDir "symbol_mutate.pas").Replace('\', '/')

    & node $harness list --workspace $workspaceDir
    if ($LASTEXITCODE -ne 0) {
        Write-Host "MCP_HARNESS_TEST FAIL: list" -ForegroundColor Red
        exit 1
    }

    function Invoke-McpCall([string]$tool, [string]$argsJson, [string]$expectPattern) {
        $argsFile = Join-Path $argsDir ($tool + ".json")
        [System.IO.File]::WriteAllText($argsFile, $argsJson)
        $prevEap = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        $out = & node $harness call --workspace $workspaceDir --tool $tool --args-file $argsFile --timeout-ms 120000 2>&1 | Out-String
        $callExit = $LASTEXITCODE
        $ErrorActionPreference = $prevEap
        if ($callExit -ne 0) {
            Write-Host "MCP_HARNESS_TEST FAIL: tools/call $tool" -ForegroundColor Red
            Write-Host $out
            exit 1
        }
        if ($expectPattern -and $out -notmatch $expectPattern) {
            Write-Host "MCP_HARNESS_TEST FAIL: $tool sem payload esperado ($expectPattern)" -ForegroundColor Red
            Write-Host $out
            exit 1
        }
    }

    $editTarget = Join-Path $workspaceDir "edit-target.pas"
    @(
        "unit EditHarness;",
        "interface",
        "implementation",
        "const",
        "  Marker = 1;",
        "end."
    ) | Set-Content -Path $editTarget -Encoding UTF8
    $editAbs = (Resolve-Path $editTarget).Path.Replace('\', '/')

    Invoke-McpCall "hover" ('{"filePath":"' + $fixtureAbs + '","line":8,"column":4}') 'TSmoke|harness'
    Invoke-McpCall "definition" '{"symbolName":"TSmoke.Consume"}' 'procedure|Consume'
    Invoke-McpCall "diagnostics" ('{"filePath":"' + $fixtureAbs + '"}') 'E001|diagnostic'
    Invoke-McpCall "references" '{"symbolName":"TSmoke.Consume"}' 'TSmoke|References'
    Invoke-McpCall "workspace_symbols" '{"query":"TSmoke"}' 'TSmoke'
    Invoke-McpCall "code_actions" ('{"filePath":"' + $fixtureAbs + '","line":8,"column":4}') 'quickfix|Trim'

    Invoke-McpCall "edit_file" ('{"filePath":"' + $editAbs + '","edits":[{"startLine":5,"startColumn":1,"endLine":5,"endColumn":20,"newText":"  Marker = 42;"}]}') 'Successfully applied'
    if ((Get-Content -Path $editTarget -Raw) -notmatch 'Marker = 42') {
        Write-Host "MCP_HARNESS_TEST FAIL: edit_file sem mutacao em disco" -ForegroundColor Red
        exit 1
    }

    Invoke-McpCall "rename_symbol" ('{"filePath":"' + $fixtureAbs + '","line":6,"column":4,"newName":"TSmokeRenamed"}') 'Successfully renamed|TSmokeRenamed|Updated'
    if ((Get-Content -Path (Join-Path $workspaceDir "smoke.pas") -Raw) -notmatch 'TSmokeRenamed') {
        Write-Host "MCP_HARNESS_TEST FAIL: rename_symbol sem TSmokeRenamed em disco" -ForegroundColor Red
        exit 1
    }

    if (Test-Path (Join-Path $workspaceDir "symbol_mutate.pas")) {
        $replaceArgs = (@{
            filePath = $symbolMutateAbs
            symbolName = 'TMutate.Target'
            newBody = "begin`n  // HARNESS_BODY_REPLACED`nend;"
        } | ConvertTo-Json -Compress)
        Invoke-McpCall "replace_symbol_body" $replaceArgs 'Successfully|applied|replaced|HARNESS_BODY_REPLACED'
        Invoke-McpCall "insert_after_symbol" ('{"filePath":"' + $symbolMutateAbs + '","symbolName":"TMutate.Target","text":"// HARNESS_AFTER"}') 'Successfully|inserted'
        Invoke-McpCall "insert_before_symbol" ('{"filePath":"' + $symbolMutateAbs + '","symbolName":"TMutate.Target","text":"// HARNESS_BEFORE"}') 'Successfully|inserted'
        Invoke-McpCall "safe_delete_symbol" ('{"filePath":"' + $symbolMutateAbs + '","symbolName":"DeleteMe","force":true}') 'Successfully|deleted|removed|safe'
        $mutateDisk = Get-Content -Path (Join-Path $workspaceDir "symbol_mutate.pas") -Raw
        if ($mutateDisk -notmatch 'HARNESS_BODY_REPLACED|HARNESS_AFTER|HARNESS_BEFORE') {
            Write-Host "MCP_HARNESS_TEST FAIL: symbol_mutate sem marcadores de mutacao" -ForegroundColor Red
            exit 1
        }
        if ($mutateDisk -match 'procedure DeleteMe') {
            Write-Host "MCP_HARNESS_TEST FAIL: DeleteMe ainda presente apos safe_delete" -ForegroundColor Red
            exit 1
        }
    }

    if ($repoSnapshot -ne (Get-Content -Path $fixtureSrc -Raw)) {
        Write-Host "MCP_HARNESS_TEST FAIL: repo fixture smoke.pas foi mutado" -ForegroundColor Red
        exit 1
    }
}
finally {
    Remove-Item -Recurse -Force $workspaceDir, $argsDir -ErrorAction SilentlyContinue
}

Write-Host "MCP_HARNESS_TEST OK (workspace hermetico + mutacoes + matriz critica)"
exit 0
