# MCP tools — documentação pública (42 tools)

Fonte de verdade operacional: `tools/list` + este catálogo. Paridade com `tools_inventory.go` validada por `TestPublicTools_ToolsListDocumentsEveryInventoryEntry`.

## Catálogo resumido (todas as tools)

| Tool | Objective | Required params | Example | Critical | NFR |
|------|-----------|-----------------|---------|----------|-----|
| run_query | Query tree-sitter estrutural (fallback quando LSP indisponivel) | query | `{"query":"(class_declaration) @name"}` | yes | degradado sem LSP |
| diagnostics | Diagnosticos LSP do arquivo | filePath | `{"filePath":"unit.pas"}` | yes | LSP-backed |
| hover | Hover LSP na posicao | filePath, line, column | `{"filePath":"unit.pas","line":8,"column":4}` | yes | LSP-backed |
| definition | Definicao de simbolo | symbolName | `{"symbolName":"TSmoke.Consume"}` | yes | LSP-backed |
| onboarding | Onboarding automatico do projeto | projectPath | `{"projectPath":"C:/workspace"}` | yes | local |
| edit_file | Edits multiplos no arquivo | edits, filePath | `{"filePath":"unit.pas","edits":[]}` | yes | mutating |
| references | Referencias LSP | symbolName | `{"symbolName":"TSmoke"}` | yes | LSP-backed |
| rename_symbol | Rename via LSP | filePath, line, column, newName | `{"filePath":"unit.pas","line":8,"column":4,"newName":"Renamed"}` | yes | mutating |
| workspace_symbols | Busca simbolos workspace | query | `{"query":"TSmoke"}` | yes | LSP-backed |
| get_symbols_overview | Overview da unit | uri | `{"uri":"file:///..."}` | yes | LSP-backed |
| ast_summary | Interface da unit | uri | `{"uri":"file:///..."}` | no | LSP-backed |
| dependency_tree | Arvore de dependencias | uri | `{"uri":"file:///..."}` | yes | LSP-backed |
| graph_neighbors | Vizinhos no grafo | symbolName | `{"symbolName":"TSmoke"}` | no | LSP-backed |
| graph_node | No do grafo | symbolName | `{"symbolName":"TSmoke"}` | no | LSP-backed |
| graph_query | Query no grafo | uri, direction (opcional) | `{"uri":"file:///workspace/Unit1.pas","direction":"outgoing"}` | yes | LSP-backed |
| call_graph | Grafo de chamadas | symbolName | `{"symbolName":"TSmoke.Consume"}` | no | LSP-backed |
| semantic_search | Busca semantica | query | `{"query":"TSmoke"}` | yes | LSP-backed |
| code_actions | Code actions LSP | filePath, line, column | `{"filePath":"unit.pas","line":8,"column":4}` | yes | LSP-backed |
| replace_symbol_body | Substitui corpo do simbolo | filePath, symbolName, newBody | `{"filePath":"C:/workspace/Unit1.pas","symbolName":"TSmoke.Consume","newBody":"begin end;"}` | yes | mutating |
| insert_after_symbol | Insere apos simbolo | filePath, symbolName, text | `{"filePath":"C:/workspace/Unit1.pas","symbolName":"TSmoke","text":"procedure X;"}` | yes | mutating |
| insert_before_symbol | Insere antes simbolo | filePath, symbolName, text | `{"filePath":"C:/workspace/Unit1.pas","symbolName":"TSmoke","text":"procedure X;"}` | yes | mutating |
| safe_delete_symbol | Remove simbolo com seguranca | filePath, symbolName | `{"filePath":"C:/workspace/Unit1.pas","symbolName":"TSmoke.Consume"}` | yes | mutating |
| get_diagnostics_for_symbol | Diagnosticos por simbolo | filePath, symbolName | `{"filePath":"C:/workspace/Unit1.pas","symbolName":"TSmoke.Consume"}` | yes | LSP-backed |
| find_implementations | Implementacoes | symbolName | `{"symbolName":"TSmoke"}` | no | LSP-backed |
| get_node_at_position | No AST na posicao | filePath, line, column | `{"filePath":"unit.pas","line":8,"column":4}` | yes | LSP-backed |
| get_node_types | Tipos de nos tree-sitter | (nenhum) | `{}` | no | local |
| check_onboarding_performed | Verifica onboarding | projectPath | `{"projectPath":"."}` | yes | local |
| analyze_complexity | Complexidade ciclomatica | src, symbol_name | `{"src":"procedure TFoo.Bar;\nbegin\n  if x > 0 then DoIt;\nend;","symbol_name":"TFoo.Bar"}` | no | local |
| find_similar_code | Codigo similar | src, query | `{"src":"procedure A;\nbegin\n  DoWork;\nend;","query":"DoWork"}` | no | local |
| activate_project | Ativa diretorio projeto | dir | `{"dir":"C:/workspace/MyProject"}` | no | local |
| build_query | Monta query tree-sitter | node_type | `{"node_type":"class_declaration","symbol":"TSmoke"}` | no | local |
| adapt_query | Adapta query por dialeto | base, dialect | `{"base":"(class_declaration) @name","dialect":"delphi6"}` | no | local |
| memory_write | Escreve memoria agente | title, content | `{"title":"note","content":"text"}` | yes | mutating |
| write_memory | Alias memory_write | title, content | `{"title":"note","content":"text"}` | no | mutating |
| memory_read | Le memoria | id | `{"id":"note"}` | yes | local |
| read_memory | Alias memory_read | id | `{"id":"note"}` | no | local |
| memory_list | Lista memorias | (nenhum) | `{}` | yes | local |
| list_memories | Alias memory_list | (nenhum) | `{}` | no | local |
| memory_edit | Edita memoria | id, content | `{"id":"note","content":"text"}` | no | mutating |
| edit_memory | Alias memory_edit | id, content | `{"id":"note","content":"text"}` | no | mutating |
| memory_delete | Remove memoria | id | `{"id":"note"}` | no | mutating |
| delete_memory | Alias memory_delete | id | `{"id":"note"}` | no | mutating |

*(Demais tools seguem o mesmo padrao de colunas; nomes alinhados a `tools_inventory.go`.)*

## Validação por agente/CI (CLI First)

| Comando | Quando usar | Exige oracle-lsp real |
|---------|-------------|----------------------|
| `go test . -count=1` | Inventário completo de tools + regressões Go | Não (fake LSP no harness) |
| `powershell -NoProfile -File scripts/test-mcp-tools-harness.ps1` | Gate de release: matriz crítica com `tools/call` e asserts | Não (fake LSP) |
| `powershell -NoProfile -File scripts/test-mcp-tools-harness-negative.ps1` | Fixtures negativas (edição sem mutação, safe_delete) | Não |

Saída esperada: `MCP_HARNESS_TEST OK (workspace hermetico + mutacoes + matriz critica completa)`.

## Detalhe — tools criticas LSP-backed (harness)

### run_query
- **Objective:** executar query tree-sitter no workspace
- **Required params:** `query`
- **Example:** `{"query":"(class_declaration) @name"}`
- **NFR:** fallback estrutural; nao substitui semantica LSP plena

### diagnostics
- **Objective:** publicar diagnosticos do arquivo via LSP
- **Required params:** `filePath`
- **Example:** `{"filePath":"C:/path/unit.pas"}`
- **NFR:** LSP-backed; timeout documentado em `tools_nfr_inventory.go`

### hover
- **Objective:** hover LSP (tipo/doc) na posicao
- **Required params:** `filePath`, `line`, `column` (1-indexed)
- **Example:** `{"filePath":"unit.pas","line":8,"column":4}`
- **NFR:** LSP-backed

### definition
- **Objective:** obter definicao de simbolo
- **Required params:** `symbolName`
- **Example:** `{"symbolName":"TSmoke.Consume"}`
- **NFR:** LSP-backed
