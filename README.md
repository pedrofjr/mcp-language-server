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
2. **Install or update this server**: `go install github.com/isaacphi/mcp-language-server@latest`
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
- `get_symbols_overview`: Aggregates `workspace/symbol` results by file/unit and returns a compact JSON summary with totals and grouped symbols.
- `run_query`: Executa o motor de consulta v2 com varredura Delphi no workspace, shape estruturado de matches e compatibilidade legada.

## run_query

Status atual: v2 entregue (baseline estável), ainda abaixo de SOTA estrutural.

O `run_query` está disponível em uma versão v2 que amplia o contrato anterior. Ele realiza varredura textual determinística em arquivos Delphi do workspace (`.pas`, `.dpr`, `.dpk`), permite recorte estrito por arquivo e retorna matches em shape estruturado. O objetivo desta versão é garantir integração previsível para agentes e clientes sem quebrar compatibilidade com consumidores legados.

### Parâmetros

- `query` (string, opcional): texto procurado nas linhas do arquivo. Se informado, é o valor principal usado na busca.
- `node_type` (string, opcional): fallback usado quando `query` não for informado.
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
- `matches`: lista de objetos com campos estruturados (`filePath`, `startLine`, `startColumn`, `endLine`, `endColumn`, `nodeType`, `preview`).
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

Busca usando `node_type` como fallback quando `query` não é enviado:

```json
{
  "name": "run_query",
  "arguments": {
    "node_type": "procedure_declaration",
    "limit": 3
  }
}
```

### Limitações atuais

- Ainda não executa query tree-sitter real.
- `nodeType` atual é inferido de forma textual (heurística), sem parser estrutural completo.
- Ainda não retorna capturas tree-sitter, score semântico e contexto de AST completo.
- O resultado v2 é textual-estruturado e determinístico, útil como baseline de contrato, não como busca estrutural SOTA.

### Próximos passos para a versão SOTA

- Executar queries tree-sitter reais sobre Delphi.
- Retornar matches estruturados com `range`, tipo de nó e capturas.
- Permitir filtro estrito por arquivo, diretório e escopo sintático.
- Evoluir o payload para cenários de auditoria, refactor assistido e navegação estrutural.

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
