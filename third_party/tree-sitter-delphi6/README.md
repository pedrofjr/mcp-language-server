# tree-sitter-delphi6 (vendored)

Cópia mínima do parser Delphi 6 usada por `run_query` e tools estruturais do MCP.

- `src/parser.c` — gramática gerada (tree-sitter)
- `bindings/go` — binding CGO consumido via `go.mod` (`replace` local)

## Modo release / clone limpo

`go build` e `go test` do repositório **não** exigem `../Delphi_Oracle`; o módulo resolve para `./third_party/tree-sitter-delphi6/bindings/go`.

## Modo monorepo (desenvolvimento)

Após alterar a gramática em `Delphi_Oracle/tree-sitter-delphi6`, atualize o vendor:

```powershell
.\scripts\sync-tree-sitter-delphi6.ps1
# ou, com caminho explícito:
.\scripts\sync-tree-sitter-delphi6.ps1 -SourceRoot C:\caminho\Delphi_Oracle\tree-sitter-delphi6
```

Depois rode `go test ./internal/tools/... -run RunQuery` e commite `third_party/` junto com a mudança de gramática.
