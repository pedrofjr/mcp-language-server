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
        $null = Invoke-McpCallCapture $tool $argsJson
        if ($expectPattern -and $script:lastMcpOut -notmatch $expectPattern) {
            Write-Host "MCP_HARNESS_TEST FAIL: $tool sem payload esperado ($expectPattern)" -ForegroundColor Red
            Write-Host $script:lastMcpOut
            exit 1
        }
    }

    function Invoke-McpCallCapture([string]$tool, [string]$argsJson) {
        $argsFile = Join-Path $argsDir ($tool + ".json")
        [System.IO.File]::WriteAllText($argsFile, $argsJson, [System.Text.UTF8Encoding]::new($false))
        $prevEap = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        $script:lastMcpOut = & node $harness call --workspace $workspaceDir --tool $tool --args-file $argsFile --timeout-ms 120000 2>&1 | Out-String
        $callExit = $LASTEXITCODE
        $ErrorActionPreference = $prevEap
        if ($callExit -ne 0) {
            Write-Host "MCP_HARNESS_TEST FAIL: tools/call $tool" -ForegroundColor Red
            Write-Host $script:lastMcpOut
            exit 1
        }
        return $script:lastMcpOut
    }

    function Get-DelphiFileUri([string]$absPath) {
        $normalized = (Resolve-Path $absPath).Path.Replace('\', '/')
        return "file:///$normalized"
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

    $fixtureUri = Get-DelphiFileUri (Join-Path $workspaceDir "smoke.pas")
    $smokeAbsWin = (Resolve-Path (Join-Path $workspaceDir "smoke.pas")).Path

    Invoke-McpCall "get_symbols_overview" (@{ query = "TSmoke" } | ConvertTo-Json -Compress) 'uri|symbol|TSmoke'
    Invoke-McpCall "dependency_tree" (@{ uri = $fixtureUri } | ConvertTo-Json -Compress) 'tree|treeBySection'
    Invoke-McpCall "graph_query" (@{ uri = $fixtureUri; direction = "both"; depth = 1 } | ConvertTo-Json -Compress) 'graph|nodes|edges'
    Invoke-McpCall "semantic_search" (@{ query = "TSmoke"; limit = 5 } | ConvertTo-Json -Compress) 'semantic|TSmoke|symbol|match'
    Invoke-McpCall "run_query" (@{ query = "TSmoke"; filePath = $smokeAbsWin; limit = 5 } | ConvertTo-Json -Compress) 'totalMatches|matches'
    Invoke-McpCall "get_diagnostics_for_symbol" (@{ filePath = $smokeAbsWin; symbolName = "TSmoke" } | ConvertTo-Json -Compress) 'diagnostic|symbol|No diagnostic|E001'
    Invoke-McpCall "get_node_at_position" (@{ filePath = $smokeAbsWin; line = 6; column = 11 } | ConvertTo-Json -Compress) 'TSmoke|token|identifier|node'

    Invoke-McpCall "onboarding" (@{ projectPath = $workspaceDir } | ConvertTo-Json -Compress) 'unit|Delphi|onboarding|entry|smoke'
    Invoke-McpCall "check_onboarding_performed" (@{ projectPath = $workspaceDir } | ConvertTo-Json -Compress) 'executado|Onboarding executado|performed'

    $memDir = Join-Path $workspaceDir ".oracle-memory"
    New-Item -ItemType Directory -Path $memDir -Force | Out-Null
    $prevMem = $env:ORACLE_MEMORY_DIR
    $env:ORACLE_MEMORY_DIR = $memDir
    try {
        $memTitle = "harness-mem-" + [Guid]::NewGuid().ToString("N").Substring(0, 8)
        $memWriteOut = Invoke-McpCallCapture "memory_write" (
            @{ title = $memTitle; content = "harness memory content"; tags = @("harness") } | ConvertTo-Json -Compress
        )
        if ($memWriteOut -notmatch 'memory|id|title') {
            Write-Host "MCP_HARNESS_TEST FAIL: memory_write sem payload util" -ForegroundColor Red
            Write-Host $memWriteOut
            exit 1
        }
        $memId = $null
        if ($memWriteOut -match '"id"\s*:\s*"([0-9a-fA-F-]{36})"') {
            $memId = $Matches[1]
        } elseif ($memWriteOut -match '\b([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})\b') {
            $memId = $Matches[1]
        }
        if (-not $memId) {
            Write-Host "MCP_HARNESS_TEST FAIL: memory_write sem id capturavel" -ForegroundColor Red
            Write-Host $memWriteOut
            exit 1
        }
        Invoke-McpCall "memory_read" (@{ id = $memId } | ConvertTo-Json -Compress) ($memTitle + '|harness memory')
        Invoke-McpCall "memory_list" '{}' ($memTitle + '|entries|memory')
    } finally {
        if ($null -ne $prevMem) { $env:ORACLE_MEMORY_DIR = $prevMem } else { Remove-Item Env:ORACLE_MEMORY_DIR -ErrorAction SilentlyContinue }
    }

    if ($repoSnapshot -ne (Get-Content -Path $fixtureSrc -Raw)) {
        Write-Host "MCP_HARNESS_TEST FAIL: repo fixture smoke.pas foi mutado" -ForegroundColor Red
        exit 1
    }
}
finally {
    Remove-Item -Recurse -Force $workspaceDir, $argsDir -ErrorAction SilentlyContinue
}

Write-Host "MCP_HARNESS_TEST OK (workspace hermetico + mutacoes + matriz critica completa)"
exit 0
