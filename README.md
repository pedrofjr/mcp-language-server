# MCP Language Server

[![Go Tests](https://github.com/isaacphi/mcp-language-server/actions/workflows/go.yml/badge.svg)](https://github.com/isaacphi/mcp-language-server/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/isaacphi/mcp-language-server)](https://goreportcard.com/report/github.com/isaacphi/mcp-language-server)
[![GoDoc](https://pkg.go.dev/badge/github.com/isaacphi/mcp-language-server)](https://pkg.go.dev/github.com/isaacphi/mcp-language-server)
[![Go Version](https://img.shields.io/github/go-mod/go-version/isaacphi/mcp-language-server)](https://github.com/isaacphi/mcp-language-server/blob/main/go.mod)

This is an [MCP](https://modelcontextprotocol.io/introduction) server that runs and exposes a [language server](https://microsoft.github.io/language-server-protocol/) to LLMs. Not a language server for MCP, whatever that would be.

## Demo

`mcp-language-server` helps MCP enabled clients navigate codebases more easily by giving them access semantic tools like get definition, references, rename, and diagnostics.

![Demo](demo.gif)

## Setup

1. **Install Go**: Follow instructions at <https://golang.org/doc/install>
2. **Install or update this server** (pick a supported path):

   | Need | Supported install |
   |------|-------------------|
   | **Delphi / `run_query` tree-sitter** (recommended) | Clone this repo, then from the repo root: `go install .` or `just install`. The parser is vendored in `third_party/tree-sitter-delphi6/` — no sibling `Delphi_Oracle` checkout. |
   | **Generic MCP without Delphi structural tools** | `go install github.com/isaacphi/mcp-language-server@latest` (tags before `v0.1.2` do not bundle the Delphi parser). |
   | **Remote install with Delphi parser** | **Not validated in CI until a published tag is exercised.** Use clone + `go install .` for Delphi. Do not document `@tag` install as supported until `release_install_test.go` covers `go install github.com/isaacphi/mcp-language-server@<tag>` in a clean module cache. |

   **CLI reference:** run `mcp-language-server --help` for flags (`--workspace`, `--lsp`, `--search-path`, `--delphi-installation-path`, …). Exit code **0** on clean shutdown; **1** on startup or fatal server error. MCP tool responses use operational `OP_*` codes with `action:` and `recovery:` hints (see `tools_nfr_inventory.go` and `TestNFRGates_*` in this repo).

3. **Install a language server**: _follow one of the guides below_
4. **Configure your MCP client**: _follow one of the guides below_

<details>
  <summary>Go (gopls)</summary>
  <div>
    <p><strong>Install gopls</strong>: <code>go install golang.org/x/tools/gopls@latest</code></p>
    <p><strong>Configure your MCP client</strong>: This will be different but similar for each client. For Claude Desktop, add the following to <code>~/Library/Application\ Support/Claude/claude_desktop_config.json</code></p>

<pre>
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": ["--workspace", "/Users/you/dev/yourproject/", "--lsp", "gopls"],
      "env": {
        "PATH": "/opt/homebrew/bin:/Users/you/go/bin",
        "GOPATH": "/users/you/go",
        "GOCACHE": "/users/you/Library/Caches/go-build",
        "GOMODCACHE": "/Users/you/go/pkg/mod"
      }
    }
  }
}
</pre>

<p><strong>Note</strong>: Not all clients will need these environment variables. For Claude Desktop you will need to update the environment variables above based on your machine and username:</p>
<ul>
  <li><code>PATH</code> needs to contain the path to <code>go</code> and to <code>gopls</code>. Get this with <code>echo $(which go):$(which gopls)</code></li>
  <li><code>GOPATH</code>, <code>GOCACHE</code>, and <code>GOMODCACHE</code> may be different on your machine. These are the defaults.</li>
</ul>

  </div>
</details>
<details>
  <summary>Rust (rust-analyzer)</summary>
  <div>
    <p><strong>Install rust-analyzer</strong>: <code>rustup component add rust-analyzer</code></p>
    <p><strong>Configure your MCP client</strong>: This will be different but similar for each client. For Claude Desktop, add the following to <code>~/Library/Application\ Support/Claude/claude_desktop_config.json</code></p>

<pre>
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": [
        "--workspace",
        "/Users/you/dev/yourproject/",
        "--lsp",
        "rust-analyzer"
      ]
    }
  }
}
</pre>
  </div>
</details>
<details>
  <summary>Python (pyright)</summary>
  <div>
    <p><strong>Install pyright</strong>: <code>npm install -g pyright</code></p>
    <p><strong>Configure your MCP client</strong>: This will be different but similar for each client. For Claude Desktop, add the following to <code>~/Library/Application\ Support/Claude/claude_desktop_config.json</code></p>

<pre>
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": [
        "--workspace",
        "/Users/you/dev/yourproject/",
        "--lsp",
        "pyright-langserver",
        "--",
        "--stdio"
      ]
    }
  }
}
</pre>
  </div>
</details>
<details>
  <summary>Typescript (typescript-language-server)</summary>
  <div>
    <p><strong>Install typescript-language-server</strong>: <code>npm install -g typescript typescript-language-server</code></p>
    <p><strong>Configure your MCP client</strong>: This will be different but similar for each client. For Claude Desktop, add the following to <code>~/Library/Application\ Support/Claude/claude_desktop_config.json</code></p>

<pre>
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": [
        "--workspace",
        "/Users/you/dev/yourproject/",
        "--lsp",
        "typescript-language-server",
        "--",
        "--stdio"
      ]
    }
  }
}
</pre>
  </div>
</details>
<details>
  <summary>C/C++ (clangd)</summary>
  <div>
    <p><strong>Install clangd</strong>: Download prebuilt binaries from the <a href="https://github.com/clangd/clangd/releases">official LLVM releases page</a> or install via your system's package manager (e.g., <code>apt install clangd</code>, <code>brew install clangd</code>).</p>
    <p><strong>Configure your MCP client</strong>: This will be different but similar for each client. For Claude Desktop, add the following to <code>~/Library/Application\\ Support/Claude/claude_desktop_config.json</code></p>

<pre>
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": [
        "--workspace",
        "/Users/you/dev/yourproject/",
        "--lsp",
        "/path/to/your/clangd_binary",
        "--",
        "--compile-commands-dir=/path/to/yourproject/build_or_compile_commands_dir"
      ]
    }
  }
}
</pre>
    <p><strong>Note</strong>:</p>
    <ul>
      <li>Replace <code>/path/to/your/clangd_binary</code> with the actual path to your clangd executable.</li>
      <li><code>--compile-commands-dir</code> should point to the directory containing your <code>compile_commands.json</code> file (e.g., <code>./build</code>, <code>./cmake-build-debug</code>).</li>
      <li>Ensure <code>compile_commands.json</code> is generated for your project for clangd to work effectively.</li>
    </ul>
  </div>
</details>
<details>
  <summary>Other</summary>
  <div>
    <p>I have only tested this repo with the servers above but it should be compatible with many more. Note:</p>
    <ul>
      <li>The language server must communicate over stdio.</li>
      <li>Any aruments after <code>--</code> are sent as arguments to the language server.</li>
      <li>Any env variables are passed on to the language server.</li>
    </ul>
  </div>
</details>

## Tools

- `definition`: Retrieves the complete source code definition of a symbol. Names can be unqualified, but package/type/unit-qualified names may be required or more precise depending on the language server.
- `references`: Locates usages and references of a symbol. Names can be unqualified, but package/type/unit-qualified names may be required or more precise depending on the language server.
- `diagnostics`: Provides diagnostic information for a specific file, including warnings and errors.
- `hover`: Display documentation, type hints, or other hover information for a given location.
- `rename_symbol`: Rename a symbol across a project.
- `edit_file`: Allows making multiple text edits to a file based on line numbers. Provides a more reliable and context-economical way to edit files compared to search and replace based edit tools.
- `get_symbols_overview`: Aggregates `workspace/symbol` results by file/unit and returns a compact JSON summary with totals and grouped symbols. Each symbol now includes `trace` with `preferredSymbolName`, `symbolNameCandidates`, and ready-to-call `definition.symbolName` / `references.symbolName` bridge values.
- `run_query`: **Fallback estrutural degradado** (tree-sitter local) quando o Oracle LSP não está disponível — não substitui análise semântica Delphi. Consulta em `.pas/.pp/.dpr/.dpk/.lpr/.inc` com `node_type`, fallback textual, `captureName` e `symbolName`.
- `dependency_tree`: Grafo de dependências de units/símbolos via LSP (requer `--lsp` configurado).
- `graph_query`, `semantic_search`, `replace_symbol_body`, `insert_after_symbol`, `insert_before_symbol`, `safe_delete_symbol`, `get_diagnostics_for_symbol`, `get_node_at_position`: LSP-backed; ver `tools_inventory.go`.
- `workspace_symbols`, `code_actions`, `memory_read`, `memory_write`, `memory_list`, `onboarding`, `check_onboarding_performed`: ver inventário completo em `tools_inventory.go` / `TestToolInventory_MatchesToolsList`.

## Fluxo inicial recomendado

Para explorar um projeto de forma orientada:

1. Rode `check_onboarding_performed`.
2. Se ainda nao estiver pronto, rode `onboarding` e depois `check_onboarding_performed` novamente.
   - Por padrao o estado de onboarding persiste fora do workspace (`%UserConfigDir%/oracle-mcp/onboarding/`).
   - Use `persistInProject: true` (ou `DELPHI_ORACLE_MCP_ONBOARDING_IN_PROJECT=1`) apenas se precisar do marcador `.oracle-onboarding.json` no projeto.
3. Rode `get_symbols_overview` para obter um panorama inicial de unidades e simbolos.

Exemplo de encadeamento: selecione `units[i].symbols[j].trace.definition.symbolName` (ou `trace.references.symbolName`) retornado por `get_symbols_overview` e passe diretamente como `symbolName` nas tools `definition` e `references`.

## run_query

Status atual: v2 com busca estrutural tree-sitter Delphi 6 integrada no runtime Go (`third_party/tree-sitter-delphi6`).

O `run_query` varre arquivos Delphi do workspace com a mesma matriz de extensões de `references` (`.pas`, `.pp`, `.dpr`, `.dpk`, `.lpr`, `.inc`), parseia com tree-sitter e retorna matches em JSON estruturado. Com `node_type` (ou `query` no formato de tipo de nó válido), executa query tree-sitter `(node_type) @match`. Sem `node_type`, consultas livres usam fallback textual: filtro por substring no texto dos nós nomeados da AST. Campos legados `file`, `line` e `text` permanecem por compatibilidade.

### Parâmetros

- `query` (string, opcional): filtro textual adicional no conteúdo do nó quando informado junto com `node_type`; sozinho, pode ser consulta livre (fallback textual na AST) ou tipo de nó implícito quando corresponder a um `node_type` tree-sitter válido (ex.: `procedure_declaration`).
- `node_type` (string, opcional): tipo de nó tree-sitter para query estrutural `(node_type) @match`; obrigatório quando `query` não é enviado.
- `filePath` (string, opcional): caminho de arquivo a ser lido primeiro. Quando presente, deve apontar para um arquivo existente.
- `strictFilePath` (boolean, opcional): quando `true`, a busca fica estritamente limitada a `filePath` e não faz fallback para outros arquivos.
- `limit` (number, opcional): quantidade máxima de ocorrências retornadas. Valor padrão: `20`. Deve ser maior que `0`.

Regra de validação mínima:

- É obrigatório informar `query` ou `node_type`.
- `query`, `node_type` e `filePath` precisam ser strings quando fornecidos.
- `strictFilePath` precisa ser boolean quando fornecido.
- `strictFilePath=true` exige `filePath` válido.
- `filePath` inválido ou diretório retorna erro.

### Retorno

O resultado MCP é devolvido como texto contendo JSON com o formato abaixo:

```json
{
  "query": "procedure_declaration",
  "totalMatches": 1,
  "matches": [
    {
      "filePath": "C:\\repo\\query-target.pas",
      "startLine": 1,
      "startColumn": 1,
      "endLine": 1,
      "endColumn": 35,
      "nodeType": "procedure_declaration",
      "captureName": "match",
      "symbolName": "UniqueProcedureDeclarationToken",
      "preview": "procedure UniqueProcedureDeclarationToken;",
      "file": "C:\\repo\\query-target.pas",
      "line": 1,
      "text": "procedure UniqueProcedureDeclarationToken;"
    }
  ]
}
```

Campos atuais:

- `query`: valor efetivamente usado na busca, após trim e fallback para `node_type` quando necessário.
- `totalMatches`: quantidade retornada no payload atual.
- `matches`: lista de objetos com campos estruturados (`filePath`, `startLine`, `startColumn`, `endLine`, `endColumn`, `nodeType`, `preview`, `captureName`, `symbolName`).
- `captureName`: preenchido apenas em matches originados de captura explícita tree-sitter (`@match`).
- `symbolName`: heurística para declarações de rotina, `unit`/`program`/`library` e `package` canônico em `.dpk` (ausente quando não aplicável).
- `file`, `line`, `text`: campos legados mantidos por compatibilidade retroativa.

### Exemplos

Busca mínima por texto:

```json
{
  "name": "run_query",
  "arguments": {
    "query": "procedure_declaration",
    "limit": 5
  }
}
```

Busca priorizando um arquivo específico:

```json
{
  "name": "run_query",
  "arguments": {
    "query": "UniqueProcedureDeclarationToken",
    "filePath": "C:\\repo\\query-target.pas",
    "strictFilePath": true,
    "limit": 5
  }
}
```

Busca estrutural por `node_type` (sem filtro textual adicional):

```json
{
  "name": "run_query",
  "arguments": {
    "node_type": "procedure_declaration",
    "limit": 3
  }
}
```

### Matriz de extensões Delphi (workspace)

Centralizada em `internal/tools/delphi_workspace.go` e compartilhada por `run_query`, `references`/fallback Delphi e `safe_delete_symbol` (scan cross-file):

| Extensão | Uso típico |
|----------|------------|
| `.pas` | Units Delphi |
| `.pp` | Units legadas (Pascal) |
| `.dpr` | Program/project |
| `.dpk` | Package |
| `.lpr` | Program Lazarus/legado |
| `.inc` | Includes/fragmentos (`{$I}`); parseados com wrapper sintético quando necessário |

### Limitações atuais

- Não aceita query tree-sitter arbitrária (apenas padrão `(node_type) @match` ou inferência implícita de tipo a partir de `query`).
- Consultas livres sem `node_type` válido usam fallback textual por substring nos nós nomeados da AST (não varredura linha-a-linha isolada).
- `captureName` só aparece em capturas explícitas; `symbolName` cobre apenas declarações suportadas (rotina, módulo, package `.dpk`).
- `.inc` são fragmentos: o parser pode exigir wrapper `unit … interface …` antes da busca estrutural (comportamento já aplicado em `run_query`).
- Sem score semântico, sem DSL `build_query`/`adapt_query` e sem exportação de AST completa por match.

### Evoluções futuras (fora do escopo v2 atual)

- Queries tree-sitter customizadas e filtros por diretório/escopo sintático.
- Payload enriquecido para auditoria/refactor (ranges semânticos, scores).

## About

This codebase makes use of edited code from [gopls](https://go.googlesource.com/tools/+/refs/heads/master/gopls/internal/protocol) to handle LSP communication. See ATTRIBUTION for details. Everything here is covered by a permissive BSD style license.

[mcp-go](https://github.com/mark3labs/mcp-go) is used for MCP communication. Thank you for your service.

This is beta software. Please let me know by creating an issue if you run into any problems or have suggestions of any kind.

## Contributing

Please keep PRs small and open Issues first for anything substantial. AI slop O.K. as long as it is tested, passes checks, and doesn't smell too bad.

### Setup

Clone the repo:

```bash
git clone https://github.com/isaacphi/mcp-language-server.git
cd mcp-language-server
```

#### Delphi / Oracle LSP

**Fonte semântica autoritativa:** `definition`, `references`, `hover`, `rename_symbol`, `diagnostics`, `workspace_symbols`, `code_actions` e edição simbólica delegam ao **Oracle LSP** (`--lsp` apontando para `oracle-lsp`). O MCP não substitui preprocessor, HIR, SourceMap ou resolução cross-unit do LSP.

**`run_query` + tree-sitter:** consulta estrutural opcional via parser vendored em `third_party/tree-sitter-delphi6/` (auxiliar, não fonte semântica). Sem LSP, respostas são **degradadas** (texto/AST local), nunca equivalentes ao pipeline completo do LSP.

**Modo release (clone só do MCP):** `go build` / `go test` / `go install .` não exigem checkout de `Delphi_Oracle`; o vendor tree-sitter é embutido para `run_query` apenas.

**Modo monorepo (desenvolvimento Oracle):** após alterar a gramática em `Delphi_Oracle/tree-sitter-delphi6`, sincronize o vendor e commite:

```powershell
.\scripts\sync-tree-sitter-delphi6.ps1
go test ./internal/tools/... -run RunQuery -count=1
```

Detalhes: `third_party/tree-sitter-delphi6/README.md`.

A [justfile](https://just.systems/man/en/) is included for convenience:

```bash
just -l
Available recipes:
    build    # Build
    check    # Run code audit checks
    fmt      # Format code
    generate # Generate LSP types and methods
    help     # Help
    install  # Install locally
    snapshot # Update snapshot tests
    test     # Run tests
```

Configure your Claude Desktop (or similar) to use the local binary:

```json
{
  "mcpServers": {
    "language-server": {
      "command": "/full/path/to/your/clone/mcp-language-server/mcp-language-server",
      "args": [
        "--workspace",
        "/path/to/workspace",
        "--lsp",
        "language-server-executable"
      ],
      "env": {
        "LOG_LEVEL": "DEBUG"
      }
    }
  }
}
```

Rebuild after making changes.

### Logging

Setting the `LOG_LEVEL` environment variable to DEBUG enables verbose logging to stderr for all components including messages to and from the language server and the language server's logs.

### LSP interaction

- `internal/lsp/methods.go` contains generated code to make calls to the connected language server.
- `internal/protocol/tsprotocol.go` contains generated code for LSP types. I borrowed this from `gopls`'s source code. Thank you for your service.
- LSP allows language servers to return different types for the same methods. Go doesn't like this so there are some ugly workarounds in `internal/protocol/interfaces.go`.

### Local Development and Snapshot Tests

There is a snapshot test suite that makes it a lot easier to try out changes to tools. These run actual language servers on mock workspaces and capture output and logs.

You will need the language servers installed locally to run them. There are tests for go, rust, python, and typescript.

```
integrationtests/
├── tests/        # Tests are in this folder
├── snapshots/    # Snapshots of tool outputs
├── test-output/  # Gitignored folder showing the final state of each workspace and logs after each test run
└── workspaces/   # Mock workspaces that the tools run on
```

To update snapshots, run `UPDATE_SNAPSHOTS=true go test ./integrationtests/...`
