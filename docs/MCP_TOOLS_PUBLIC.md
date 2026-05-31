# MCP tools — documentação pública (42 tools)

Fonte de verdade operacional: `tools/list` + este catálogo. Paridade com `tools_inventory.go` validada por `TestPublicTools_ToolsListDocumentsEveryInventoryEntry`.

## Catálogo resumido (todas as tools)

| Tool | Objective | Required params | Example | Critical | NFR |
|------|-----------|-----------------|---------|----------|-----|
| run_query | Query tree-sitter estrutural (fallback quando LSP indisponivel) | query | `{"query":"(class_declaration) @name"}` | yes | degradado sem LSP |
| diagnostics | Diagnosticos LSP do arquivo | filePath | `{"filePath":"unit.pas"}` | yes | LSP-backed |
| hover | Hover LSP na posicao | filePath, line, column | `{"filePath":"unit.pas","line":8,"column":4}` | yes | LSP-backed |
| definition | Definicao de simbolo | symbolName | `{"symbolName":"TSmoke.Consume"}` | yes | LSP-backed |
| onboarding | Onboarding automatico do projeto | (nenhum) | `{}` | yes | local |
| edit_file | Edits multiplos no arquivo | edits, filePath | ver tools/list | yes | mutating |
| references | Referencias LSP | symbolName | `{"symbolName":"TSmoke"}` | yes | LSP-backed |
| rename_symbol | Rename via LSP | filePath, line, column, newName | ver tools/list | yes | mutating |
| workspace_symbols | Busca simbolos workspace | query | `{"query":"TSmoke"}` | yes | LSP-backed |
| get_symbols_overview | Overview da unit | uri | `{"uri":"file:///..."}` | yes | LSP-backed |
| ast_summary | Interface da unit | uri | `{"uri":"file:///..."}` | yes | LSP-backed |
| dependency_tree | Arvore de dependencias | uri | `{"uri":"file:///..."}` | yes | LSP-backed |
| graph_neighbors | Vizinhos no grafo | symbolName | ver tools/list | yes | LSP-backed |
| graph_node | No do grafo | symbolName | ver tools/list | yes | LSP-backed |
| graph_query | Query no grafo | symbolName | ver tools/list | yes | LSP-backed |
| call_graph | Grafo de chamadas | symbolName | ver tools/list | yes | LSP-backed |
| semantic_search | Busca semantica | query | ver tools/list | yes | LSP-backed |
| code_actions | Code actions LSP | filePath, line, column | ver tools/list | yes | LSP-backed |
| replace_symbol_body | Substitui corpo do simbolo | symbolName, newBody | ver tools/list | yes | mutating |
| insert_after_symbol | Insere apos simbolo | symbolName, newCode | ver tools/list | yes | mutating |
| insert_before_symbol | Insere antes simbolo | symbolName, newCode | ver tools/list | yes | mutating |
| safe_delete_symbol | Remove simbolo com seguranca | symbolName | ver tools/list | yes | mutating |
| get_diagnostics_for_symbol | Diagnosticos por simbolo | symbolName | ver tools/list | yes | LSP-backed |
| find_implementations | Implementacoes | symbolName | ver tools/list | yes | LSP-backed |
| get_node_at_position | No AST na posicao | filePath, line, column | ver tools/list | yes | LSP-backed |
| get_node_types | Tipos de nos tree-sitter | (nenhum) | `{}` | no | local |
| check_onboarding_performed | Verifica onboarding | projectPath | ver tools/list | yes | local |
| analyze_complexity | Complexidade ciclomatica | src, symbol_name | ver tools/list | no | local |
| find_similar_code | Codigo similar | src, pattern | ver tools/list | no | local |
| activate_project | Ativa diretorio projeto | dir | ver tools/list | no | local |
| build_query | Monta query tree-sitter | node_type | ver tools/list | no | local |
| adapt_query | Adapta query por dialeto | base, dialect | ver tools/list | no | local |
| memory_write | Escreve memoria agente | id, content | ver tools/list | yes | mutating |
| write_memory | Alias memory_write | id, content | ver tools/list | yes | mutating |
| memory_read | Le memoria | id | ver tools/list | yes | local |
| read_memory | Alias memory_read | id | ver tools/list | yes | local |
| memory_list | Lista memorias | (nenhum) | `{}` | yes | local |
| list_memories | Alias memory_list | (nenhum) | `{}` | yes | local |
| memory_edit | Edita memoria | id, content | ver tools/list | yes | mutating |
| edit_memory | Alias memory_edit | id, content | ver tools/list | yes | mutating |
| memory_delete | Remove memoria | id | ver tools/list | yes | mutating |
| delete_memory | Alias memory_delete | id | ver tools/list | yes | mutating |

*(Demais tools seguem o mesmo padrao de colunas; nomes alinhados a `tools_inventory.go`.)*

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
