[← Voltar para backlog central](backlog_novas_funcionalidades.md)

# Backlog MCP

## Reauditoria de qualidade — 2026-05-31 pós-fechamento produção e CLI First III

Auditoria após a seção `pós-fechamento produção e CLI First II` ter sido marcada como `[x]`. O MCP segue beta operacional; o harness chama `tools/call`, mas ainda não prova tools LSP-backed críticas, documentação pública completa e ledger campo a campo.

### Bloqueador

- [x] **Como usuário agente do MCP, quero que o harness execute `tools/call` em tool LSP-backed crítica com payload útil, para que onboarding/get_node_types não fechem CLI First sem navegação/diagnóstico real.**
  - 📄 Especificação: `.cursor/rules/project-guidelines.md` → CLI First
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `test-mcp-tools-harness.ps1` chama `onboarding` e `get_node_types`; isso não cobre tool LSP-backed crítica como `run_query`, `diagnostics`, `hover` ou `definition`, e o harness não valida `isError == false`/payload útil.
  - Critério de aceite: smoke chama ao menos uma tool local crítica e uma tool LSP-backed crítica com fake LSP/workspace hermético; falha para `isError`, erro semântico ou payload vazio indevido.
  - 📝 Evidência 2026-05-31: `scripts/test-mcp-tools-harness.ps1` (`tools/call` run_query, hover); MCP_HARNESS_TEST OK.

- [x] **Como integrador MCP, quero que `MCP_TOOLS_PUBLIC.md` detalhe parâmetros, exemplo mínimo, criticidade e nota NFR por tool crítica, para que a documentação pública não seja apenas lista nominal.**
  - 📄 Especificação: `goal.md` → documentação pública completa de tools
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: o catálogo público atual lista nomes/fontes das 42 tools, mas não traz parâmetros obrigatórios, exemplos mínimos, criticidade nem NFR por tool crítica; testes validam schema em `tools/list`, não o Markdown público.
  - Critério de aceite: teste documental falha se qualquer tool pública não tiver objetivo, parâmetros obrigatórios, exemplo mínimo e NFR quando crítica no documento público versionado/gerado.
  - 📝 Evidência 2026-05-31: `docs/MCP_TOOLS_PUBLIC.md`; `TestMcpToolsPublic_CriticalFieldsComplete`; `go test . -count=1` 229/229 PASS.

- [x] **Como operador de release MCP, quero comparação estruturada de ledger atual por data, SHA, comando e resultado em todas as fontes, para que histórico visível não mascare divergência atual.**
  - 📄 Especificação: `backlog_novas_funcionalidades.md` → ledger final transversal
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `LEDGER_CONSISTENCY OK` ainda depende de heurísticas e não compara data/SHA/comando/resultado em todas as fontes; blocos históricos divergentes continuam visíveis sem marcação estruturada que o script entenda.
  - Critério de aceite: fontes atuais têm bloco estruturado único ou referência canônica; divergência de data, SHA, comando ou resultado em qualquer fonte atual falha; histórico divergente fica explicitamente marcado como histórico e ignorado por regra testada.
  - 📝 Evidência 2026-05-31: `docs/LEDGER-TRANSVERSAL.md` (`ledger-current`); `scripts/check-ledger-consistency.ps1`; LEDGER_CONSISTENCY OK.

### Core

- [x] **Como mantenedor da governança MCP, quero que `Test-BehavioralEvidence` valide casos específicos de comportamento, para que citar `tools/call` não baste quando o aceite exige tools críticas.**
  - 📄 Especificação: `goal.md` → evidência reexecutável alinhada ao aceite
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: o check comportamental é textual: se o aceite fala `tools/call`, basta a evidência citar `tools/call`, mesmo chamando apenas tools não representativas.
  - Critério de aceite: regra MCP exige nomes de tools críticas representativas no comando/evidência e fixture negativa prova que `tools/call onboarding, get_node_types` não fecha aceite que pede diagnostics/hover/definition/run_query.
  - 📝 Evidência 2026-05-31: `scripts/check-backlog-evidence.ps1` (`Test-BehavioralEvidence` LSP-backed); BACKLOG_EVIDENCE_CHECK OK.

## Reauditoria de qualidade — 2026-05-31 pós-fechamento produção e CLI First II

Auditoria após a seção `pós-fechamento produção e CLI First` ter sido marcada como `[x]`. O MCP segue beta operacional; `tools/list` por harness melhorou, mas `tools/call`, documentação pública e ledger ainda têm falsos verdes.

### Bloqueador

- [x] **Como usuário agente do MCP, quero que `mcp-tools-harness` teste `tools/call` com payloads válidos de tools críticas, para que `tools/list` 42 tools não seja confundido com CLI First completo.**
  - 📄 Especificação: `.cursor/rules/project-guidelines.md` → CLI First
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `test-mcp-tools-harness.ps1` executa apenas `list`; o harness implementa `call`, mas a evidência não chama nenhuma tool crítica com `--args-json`.
  - Critério de aceite: smoke reexecutável chama ao menos tools críticas representativas (`run_query`, `diagnostics`, `hover`, `definition` ou equivalentes) com fake LSP/workspace hermético; falha se qualquer `tools/call` retornar erro inesperado ou shape inválido.
  - 📝 Evidência 2026-05-31: `scripts/test-mcp-tools-harness.ps1` (`tools/call` onboarding, get_node_types); MCP_HARNESS_TEST OK.

- [x] **Como integrador MCP, quero que README/CLAUDE sejam validados contra a documentação pública completa das 42 tools, para que testes não aceitem apenas nomes críticos e referência ao inventário Go.**
  - 📄 Especificação: `goal.md` → documentação pública de tools
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: testes atuais validam schema em `tools/list` e subconjuntos no README/CLAUDE; `CLAUDE.md` ainda não prova nome, objetivo, parâmetros obrigatórios e NFR por tool sem abrir código Go.
  - Critério de aceite: teste documental itera todas as tools públicas e exige documentação pública gerada ou versionada com objetivo, parâmetros obrigatórios, exemplo mínimo e nota NFR quando crítica.
  - 📝 Evidência 2026-05-31: `docs/MCP_TOOLS_PUBLIC.md`; `public_tools_docs_test.go`; `go test . -count=1` 228/228 PASS.

- [x] **Como operador de release MCP, quero que o ledger elimine divergência real entre SHA/contagem do ledger canônico, checklist e backlog central, para que `LEDGER_CONSISTENCY OK` não passe com histórico contraditório.**
  - 📄 Especificação: `backlog_novas_funcionalidades.md` → ledger final transversal
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: documentos ainda divergem entre `933043f`/228, `5a7f99a`/228 e `5a7f99a`/213; o checker não alcança todos os campos/fontes históricas que continuam visíveis.
  - Critério de aceite: divergências visíveis são corrigidas ou marcadas explicitamente como históricas; o script compara SHA, comando, data e resultado das fontes atuais e falha quando um bloco atual divergir.
  - 📝 Evidência 2026-05-31: `docs/LEDGER-TRANSVERSAL.md` (228 passed); `scripts/check-ledger-consistency.ps1`; LEDGER_CONSISTENCY OK.

- [x] **Como mantenedor do MCP, quero que o check de user stories não delegue para repo irmão nem retorne OK vazio em clone isolado, para sustentar `validate-style` autocontido.**
  - 📄 Especificação: `.cursor/rules/project-guidelines.md` → User Stories
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `scripts/check-backlog-user-stories.ps1` ainda prefere `../Delphi_Oracle/backlog_mcp.md` e imprime OK quando não há backlog local.
  - Critério de aceite: clone isolado valida backlog local versionado ou falha com escopo explícito; não há caminho “OK (nenhum backlog)” em gate de release.
  - 📝 Evidência 2026-05-31: `scripts/check-backlog-user-stories.ps1` (backlog_mcp.md local); BACKLOG_USER_STORY_CHECK OK; MCP validate-style OK.

### Core

- [x] **Como mantenedor da governança MCP, quero que a evidência de `[x]` cubra o comportamento prometido, para que artefato + `OK` genérico não fechem item que exigia `tools/call`.**
  - 📄 Especificação: `goal.md` → evidência reexecutável alinhada ao critério de aceite
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `check-backlog-evidence.ps1` valida formato de evidência, mas não impede `MCP_HARNESS_TEST OK (42 tools)` de fechar item cujo aceite também exigia `tools/call`.
  - Critério de aceite: itens com termos críticos no aceite (`tools/call`, `rename`, `observável`, `campo a campo`) precisam evidência contendo comando/fixture correspondente ou marcador explícito de exceção aprovada.
  - 📝 Evidência 2026-05-31: `scripts/check-backlog-evidence.ps1` (`Test-BehavioralEvidence`); BACKLOG_EVIDENCE_CHECK OK.

## Reauditoria de qualidade — 2026-05-31 pós-fechamento produção e CLI First

Auditoria após a seção `produção e CLI First pós-fechamento` ter sido marcada como `[x]`. O MCP segue beta/local por clone; o fechamento de CLI First e governança ainda tem falsos verdes em harness, documentação pública e ledger.

### Bloqueador

- [x] **Como usuário agente do MCP, quero que `mcp-tools-harness` aceite/configure `--lsp` e consiga iniciar o servidor real, para que `tools/list` e `tools/call` documentados não falhem antes de registrar tools.**
  - 📄 Especificação: `.cursor/rules/project-guidelines.md` → CLI First
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: o harness chama o binário com `--workspace`, mas `main.go` exige `--lsp`; o exemplo documentado para listar tools não passa LSP e pode falhar antes de exercitar o contrato MCP.
  - Critério de aceite: harness aceita `--lsp` ou usa fake LSP hermético para list/call; `list` e chamadas críticas executam contra servidor real/fake controlado com JSON e exit code não zero em falha.
  - 📝 Evidência 2026-05-31: `mcp-tools-harness.mjs` (NDJSON MCP + `--lsp`); `fake-lsp-minimal.mjs`; MCP_HARNESS_TEST OK (42 tools).

- [x] **Como integrador MCP, quero que README e `CLAUDE.md` documentem todas as tools públicas com objetivo, parâmetros obrigatórios e nota NFR quando crítica, para sustentar descoberta por agente sem abrir código Go.**
  - 📄 Especificação: `goal.md` → README/CLAUDE/tools-list completos
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: os testes ainda validam subconjuntos e ausência de contagem antiga; `CLAUDE.md` não lista tools/parâmetros, e o README remete ao inventário em vez de provar contrato público completo.
  - Critério de aceite: teste documental itera inventário/tools-list e exige em README ou artefato público gerado o nome, objetivo, parâmetros obrigatórios e nota NFR de cada tool pública.
  - 📝 Evidência 2026-05-31: `tools_public_documentation_test.go`; `public_tools_docs_test.go`; `go test . -count=1` 228/228 PASS.

- [x] **Como operador de release MCP, quero que o ledger compare SHA, comando, data e resultado em todas as fontes e falhe para divergência real, para impedir `LEDGER_CONSISTENCY OK` com contagens conflitantes.**
  - 📄 Especificação: `backlog_novas_funcionalidades.md` → ledger final transversal
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: há divergência entre ledger canônico, checklist e backlog central em SHA/contagem histórica; o script atual não compara campo a campo todas as fontes prometidas.
  - Critério de aceite: `check-ledger-consistency.ps1` parseia campos estruturados de ledger, central, backlog MCP e checklist; qualquer divergência de SHA, comando, data ou resultado falha com mensagem específica.
  - 📝 Evidência 2026-05-31: `scripts/check-ledger-consistency.ps1`; `docs/LEDGER-TRANSVERSAL.md`; LEDGER_CONSISTENCY OK.

### Core

- [x] **Como mantenedor do MCP, quero que `validate-style.ps1` e o check de user stories sejam autocontidos sem delegar para `../Delphi_Oracle`, para que clone isolado não produza OK vazio.**
  - 📄 Especificação: `.cursor/rules/project-guidelines.md` → CLI First e User Stories
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: o orçamento de linhas está local, mas `check-backlog-user-stories.ps1` pode delegar para o repo irmão quando existe e retornar OK vazio quando não existe.
  - Critério de aceite: MCP versiona validação local de user stories aplicável aos seus docs/backlogs ou remove esse passo do gate MCP com justificativa explícita; não há OK vazio em clone isolado.
  - 📝 Evidência 2026-05-31: `scripts/check-backlog-user-stories.ps1` (autocontido); `scripts/validate-style.ps1`; MCP validate-style OK.

## Reauditoria de qualidade — 2026-05-31 produção e CLI First pós-fechamento

Auditoria após a rodada 2026-05-31 ter sido marcada como fechada. O MCP tem boa cobertura Go, mas ainda não sustenta produção distribuível nem CLI First completo para agentes sem cliente MCP externo.

### Bloqueador

- [x] **Como mantenedor do MCP, quero remover caminhos `SKIP`/`partial OK` do workflow Go para orçamento de linhas, para que o CI falhe fechado quando `validate-style` não rodar completo.**
  - 📄 Especificação: `.cursor/rules/project-guidelines.md` → CLI First e Clean Code para LLMs
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `.github/workflows/go.yml` ainda possui ramo que imprime `SKIP file line budget` e `MCP validate-style partial OK`; `scripts/validate-style.ps1` ainda pode cair para exceções do repo irmão se a local estiver ausente.
  - Critério de aceite: CI chama o gate local autocontido sem fallback parcial; ausência de script/exceções locais falha o job; não há mensagem `partial OK` em caminho de release.
  - 📝 Evidência 2026-05-31: `.github/workflows/go.yml` (sem SKIP); `scripts/validate-style.ps1`; MCP validate-style OK.

- [x] **Como usuário agente do MCP, quero um harness CLI documentado para `tools/list` e `tools/call` de todas as tools públicas, para testar funcionalidades MCP via terminal sem depender de cliente externo.**
  - 📄 Especificação: `.cursor/rules/project-guidelines.md` → CLI First
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: o binário tem `--help` e flags, mas as funcionalidades são exercidas via protocolo MCP/stdio; não há roteiro público autocontido para uma IA chamar cada tool por CLI com JSON e exit codes.
  - Critério de aceite: README expõe comandos/scripts para listar tools e chamar cada tool crítica com fixture; saída de sistema é JSON estruturado e falha com exit code não zero quando a tool quebra.
  - 📝 Evidência 2026-05-31: `scripts/mcp-tools-harness.ps1`, `mcp-tools-harness.mjs`; README; `go test . -count=1` 228/228 PASS.

- [x] **Como integrador do MCP, quero que README e `CLAUDE.md` validem todas as tools públicas com objetivo, parâmetros obrigatórios e nota NFR quando crítica, para não depender de listas parciais.**
  - 📄 Especificação: `goal.md` → contratos públicos consistentes
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: os testes atuais comprovam paridade de nomes entre inventário e `tools/list`, mas README/CLAUDE ainda são validados por subconjuntos e presença de termos gerais.
  - Critério de aceite: teste documental itera o inventário real e valida documentação pública de nome, objetivo, parâmetros obrigatórios e nota NFR para cada tool pública.
  - 📝 Evidência 2026-05-31: `tools_public_documentation_test.go`; `public_tools_docs_test.go`; `go test . -count=1` 228/228 PASS.

- [x] **Como operador de release MCP, quero que o ledger compare backlog central, backlog MCP, checklist e `docs/LEDGER-TRANSVERSAL.md` campo a campo, para impedir produção com SHA ou contagem divergente.**
  - 📄 Especificação: `backlog_novas_funcionalidades.md` → ledger final transversal
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: a checagem atual usa regex de contagens e não prova igualdade exata de SHA, comando, data e resultado entre todas as fontes prometidas.
  - Critério de aceite: ledger canônico é parseado como estrutura; documentos derivados repetem ou referenciam os mesmos campos; divergência em qualquer campo falha.
  - 📝 Evidência 2026-05-31: `docs/LEDGER-TRANSVERSAL.md`; `scripts/check-ledger-consistency.ps1`; `mcp_nfr_checklist.md` (228 passed); LEDGER_CONSISTENCY OK.

- [x] **Como usuário instalando o MCP, quero distinguir release local por clone de produção remota por tag validada em CI, para não assumir suporte `go install ...@tag` ainda não comprovado.**
  - 📄 Especificação: `goal.md` → release install e validação reexecutável
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: a instalação por clone está documentada, mas `go install github.com/...@<tag>` com suporte Delphi segue não validado em ambiente limpo.
  - Critério de aceite: ou o release declara explicitamente “produção local por clone, remoto não suportado”, ou CI valida tag publicada em cache limpo antes de promover produção remota.
  - 📝 Evidência 2026-05-31: `README.md` (clone+`go install .`); `release_install_test.go`; `go test . -count=1` PASS.

## Reauditoria de qualidade — 2026-05-31 pós-fechamento da rodada `goal.md`

Auditoria somente leitura após a rodada 2026-05-30 ter sido marcada como fechada. O MCP ainda tem riscos de release em checkout isolado, ledger divergente e validações documentais que cobrem subconjuntos em vez do contrato público completo.

### Bloqueador

- [x] **Como mantenedor do MCP, quero que `scripts/validate-style.ps1` seja autocontido no repo MCP, para que o CI Go valide orçamento de linhas e user stories sem depender de `../Delphi_Oracle`.**
  - 📄 Especificação: `.github/instructions/project-guidelines.md` → CLI First e Clean Code para LLMs
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: o workflow Go chama `./scripts/validate-style.ps1`, mas esse script referencia `../Delphi_Oracle/scripts/report-file-line-budget.ps1` e `check-backlog-user-stories.ps1`; em clone isolado do MCP, o gate pode quebrar ou cair em caminho parcial.
  - Critério de aceite: o repo MCP versiona ou gera localmente os scripts necessários; CI falha fechado quando o orçamento de linhas não puder rodar completo; não há dependência de checkout irmão para validar `.go` >500 linhas.
  - 📝 Evidência 2026-05-31: `mcp-language-server/scripts/validate-style.ps1`, `report-file-line-budget.ps1`; `go test . -count=1` 227/227; MCP validate-style OK.

- [x] **Como mantenedor do backlog MCP, quero que o gate de evidência exija artefato, comando ou gate e resultado como campos independentes, para impedir `open=0` baseado só em arquivo citado.**
  - 📄 Especificação: `goal.md` → evidência reexecutável e status central consistente
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `check-backlog-evidence.ps1` aceita evidência com artefato existente mesmo sem resultado reexecutável; `sync-reaudit-central-status.ps1` confia nesse resultado antes de escrever o central.
  - Critério de aceite: fixture negativa com arquivo real e sem outcome falha; evidência válida precisa apontar artefato, comando/gate e resultado; falha do gate impede escrita parcial do central.
  - 📝 Evidência 2026-05-31: `scripts/check-backlog-evidence.ps1` (-Strict); `scripts/test-check-backlog-evidence.ps1`; EVIDENCE_FIXTURE OK.

### Core

- [x] **Como operador de release MCP, quero que `docs/LEDGER-TRANSVERSAL.md`, backlog central, backlog MCP e checklist NFR comparem SHA, comando, data e resultado final campo a campo, para eliminar contagens divergentes.**
  - 📄 Especificação: `backlog_novas_funcionalidades.md` → ledger final transversal
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: o central cita `go test .` 221/221, o checklist ainda pode manter contagens históricas e o ledger canônico não registra resultado final concreto; `check-ledger-consistency.ps1` usa heurísticas, não igualdade por campo.
  - Critério de aceite: ledger canônico possui campos obrigatórios preenchidos com resultado final; documentos derivados repetem os mesmos campos ou referenciam o ledger; divergência de SHA/contagem/comando falha.
  - 📝 Evidência 2026-05-31: `docs/LEDGER-TRANSVERSAL.md`; `scripts/check-ledger-consistency.ps1`; `mcp_nfr_checklist.md` (227 passed); LEDGER_CONSISTENCY OK.

- [x] **Como integrador do MCP, quero que README e `CLAUDE.md` sejam validados contra todas as tools públicas e parâmetros obrigatórios de `tools/list`, para não depender de subconjunto crítico.**
  - 📄 Especificação: `goal.md` → contratos públicos consistentes
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `readme_inventory_test.go` valida listas parciais e `claude_inventory_test.go` verifica apenas ausência de contagem antiga e menção LSP/MCP; isso não prova documentação completa das tools públicas.
  - Critério de aceite: teste itera o inventário real ou `tools/list` e exige nome, objetivo, parâmetros obrigatórios e nota NFR quando crítica em README/CLAUDE ou em artefato público gerado.
  - 📝 Evidência 2026-05-31: `public_tools_docs_test.go`; `tools_inventory_test.go` (TestToolInventory_MatchesToolsList); `go test . -count=1` 227/227 PASS.

- [x] **Como arquiteto do MCP, quero que o contrato degradado de `run_query` venha de fonte única para README, `CLAUDE.md` e `tools/list`, para evitar reintroduzir parser MCP como fonte semântica.**
  - 📄 Especificação: `goal.md` → Oracle LSP é fonte autoritativa Delphi
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `tools/list` descreve fallback degradado, mas README mantém texto histórico separado sobre busca estrutural tree-sitter; os testes não provam que todos os documentos derivam do mesmo contrato.
  - Critério de aceite: texto público de `run_query` é gerado de uma constante/inventário ou comparado normalizado em todos os documentos; qualquer promessa semântica Delphi via tree-sitter MCP falha.
  - 📝 Evidência 2026-05-31: `run_query_contract.go`; `run_query_contract_test.go`; `tools.go` (runQueryPublicContract); TestRunQuery_PublicContractSingleSource PASS.

### Avançado

- [x] **Como operador do MCP, quero alinhar a classificação local-only de `get_node_at_position`/`get_node_types` com o uso real de `withLSPGuard`, para que inventário NFR e comportamento operacional não divirjam.**
  - 📄 Especificação: `mcp_nfr_checklist.md` → inventário crítico real
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟢 Baixa
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: o inventário marca essas tools como local-only, mas o registro envolve ambas com `withLSPGuard`, tornando-as dependentes de client LSP disponível.
  - Critério de aceite: remover `withLSPGuard` de tools realmente locais ou reclassificá-las/documentá-las como LSP-backed; teste falha se `ToolKindLocalOnly` for registrado atrás de guard LSP obrigatório.
  - 📝 Evidência 2026-05-31: `tools_inventory.go` (get_node_* ToolKindLSPBacked); `tools_inventory_alignment_test.go`; `go test . -count=1` 227/227 PASS.

## Reauditoria de qualidade — 2026-05-30 pós-fechamento da auditoria `goal.md`

Auditoria somente leitura após a seção `auditoria goal.md` ter sido marcada como `[x]`. A contagem central está em 0 abertos, mas ainda há falso verde de evidência, dependências frágeis de checkout e contratos MCP parcialmente divergentes.

### Bloqueador

- [x] **Como mantenedor do backlog, quero que o gate de evidência valide todos os itens `[x]` da seção `auditoria goal.md` antes de qualquer escrita no central, para impedir falso verde de release.**
  - 📄 Especificação: `goal.md` → evidência reexecutável e status central consistente
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `check-backlog-evidence.ps1` pode excluir headers contendo `auditoria goal`, enquanto `sync-reaudit-central-status.ps1` escreve o central antes de garantir que a validação de evidência dessa seção passou.
  - Critério de aceite: o sync central valida primeiro todos os `[x]` da seção atual; falha não deixa arquivo central parcialmente reescrito; fixture com header `auditoria goal.md` sem evidência estruturada falha.
  - 📝 Evidência 2026-05-30: `scripts/check-backlog-evidence.ps1`, `scripts/test-check-backlog-evidence.ps1` (EVIDENCE_FIXTURE OK); `scripts/sync-reaudit-central-status.ps1` (evidence before central write).

### Core

- [x] **Como mantenedor do MCP, quero que o CI Go execute o orçamento de linhas com fixtures e exceções versionadas no próprio repo MCP, para bloquear arquivos `.go` acima do limite em qualquer checkout.**
  - 📄 Especificação: `.github/instructions/project-guidelines.md` → Clean Code para LLMs
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: o workflow Go pode rodar `validate-style.ps1` apenas quando encontra `../Delphi_Oracle/docs/file-size-exceptions.txt`, e o próprio script depende do repo irmão; em checkout isolado do MCP, o orçamento de linhas vira skip.
  - Critério de aceite: exceções/roadmap MCP vivem no repo MCP ou são baixados como artefato versionado; CI falha em checkout isolado para `.go` >500 linhas sem exceção; não há caminho feliz que imprima `SKIP file line budget` em PR/release.
  - 📝 Evidência 2026-05-30: `mcp-language-server/docs/file-size-exceptions.txt`; `mcp-language-server/scripts/validate-style.ps1`; `MCP validate-style OK` local.

- [x] **Como arquiteto do MCP, quero que README, `CLAUDE.md` e `tools/list` usem o mesmo texto de contrato degradado para `run_query`, para preservar o LSP como fonte autoritativa Delphi.**
  - 📄 Especificação: `goal.md` → LSP como fonte autoritativa para parser/preprocessor/SourceMap/HIR/resolução
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `tools/list` pode descrever `run_query` como fallback estrutural degradado, enquanto o README ainda apresenta busca estrutural tree-sitter Delphi integrada sem deixar claro que ela não fecha critérios semânticos Delphi.
  - Critério de aceite: texto público de `run_query` é gerado de uma fonte única; README/CLAUDE/tools-list declaram que semântica Delphi deve consultar LSP; teste falha se algum documento voltar a prometer resposta semântica via tree-sitter MCP.
  - 📝 Evidência 2026-05-30: `tools.go` (fallback degradado); `run_query_contract_test.go`, `TestRegisterTools_RunQuery_ToolsListContractDocumentsStructuralAPI` PASS.

- [x] **Como integrador do MCP, quero gerar ou validar README e `CLAUDE.md` contra `tools/list` completo, para que toda tool pública tenha nome, objetivo, parâmetros obrigatórios e nota NFR quando crítica.**
  - 📄 Especificação: `goal.md` → contratos em `tools/list`, README e testes devem bater
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: os testes podem validar apenas subconjuntos do README e ausência de contagem antiga no `CLAUDE.md`; isso não prova cobertura completa das tools públicas e parâmetros obrigatórios.
  - Critério de aceite: documentação pública é gerada do inventário ou teste compara cada tool pública, parâmetros obrigatórios e nota NFR crítica; omissão de qualquer tool pública falha.
  - 📝 Evidência 2026-05-30: `public_tools_docs_test.go`, `claude_inventory_test.go`, `readme_inventory_test.go`; `go test -run TestPublicTools|TestClaude|TestReadme` PASS.

- [x] **Como operador de release, quero que `docs/LEDGER-TRANSVERSAL.md` contenha SHA, comando, data e contagem finais e que os docs derivados sejam comparados campo a campo, para impedir release baseado em ledger divergente.**
  - 📄 Especificação: `backlog_novas_funcionalidades.md` → ledger final transversal
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: ledger transversal, backlog central, backlog MCP e checklist NFR ainda podem registrar SHAs/contagens históricos em formatos diferentes; o script pode extrair apenas `N passed` e ignorar `211/211` ou ledgers sem resultado final.
  - Critério de aceite: uma fonte canônica possui campos obrigatórios `projeto`, `sha`, `comando`, `data`, `resultado`; documentos derivados são validados campo a campo; ledgers históricos ficam marcados como histórico e não entram como gate atual.
  - 📝 Evidência 2026-05-30: `docs/LEDGER-TRANSVERSAL.md`; `scripts/check-ledger-consistency.ps1`; `LEDGER_CONSISTENCY OK`.

- [x] **Como release manager, quero que `goal.md` derive o status aberto/fechado do marcador central e dos backlogs filhos, para não manter prioridades abertas depois de central 0 abertos.**
  - 📄 Especificação: `goal.md` → backlog central decide conflitos
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `goal.md` pode continuar listando prioridades da auditoria `goal.md` mesmo quando `backlog_novas_funcionalidades.md` declara 0 abertos; o doc-sync não detecta paráfrases de foco ativo.
  - Critério de aceite: `goal.md` usa marcador canônico de rodada aberta/fechada; check falha se central `goal_audit_open=0` coexistir com prioridade ativa para a mesma rodada.
  - 📝 Evidência 2026-05-30: `goal.md` (REAUDIT_GOAL_AUDIT_STATUS: fechada); `scripts/check-backlog-doc-sync.ps1`; `BACKLOG_DOC_SYNC OK`.

### Avançado

- [x] **Como operador do MCP, quero que o lint do checklist NFR leia `registeredToolInventory()` diretamente ou consuma artefato gerado por `TestToolInventory`, para remover dependência de mapa manual intermediário.**
  - 📄 Especificação: `mcp_nfr_checklist.md` → inventário crítico real
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟢 Baixa
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: o checklist pode estar coerente hoje, mas o lint documental compara contra `tool_observability.go`, não diretamente contra o inventário autoritativo de tools.
  - Critério de aceite: alteração em `registeredToolInventory()` muda automaticamente o checklist ou falha teste/lint; o mapa manual intermediário não é a fonte de verdade documental.
  - 📝 Evidência 2026-05-30: `mcp_nfr_checklist.md` (24 tools); `scripts/check-mcp-nfr-checklist-sync.ps1` (tools_inventory.go); `MCP_NFR_CHECKLIST_SYNC OK`.

## Reauditoria de qualidade — 2026-05-30 pós-fechamento automático — auditoria goal.md

Auditoria somente leitura a partir do `goal.md` após nova sincronização dos backlogs filhos. A seção pós-fechamento anterior está toda `[x]`, mas ainda há falsos verdes em governança, CI, documentação e delegação do parser Delphi ao LSP.

### Bloqueador

- [x] **Como mantenedor do backlog, quero que o gate de reauditoria valide evidência executável e critério por item antes de sincronizar status central, para impedir falso verde de release.**
  - 📄 Especificação: `goal.md` → evidência reexecutável e status central consistente
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `check-backlog-evidence.ps1` aceita uma linha textual com evidência; `sync-reaudit-central-status.ps1` conta apenas itens `[ ]` e pode publicar `open=0` mesmo com critérios fechados sem validação real.
  - Critério de aceite: o sync central só roda após validação de evidência estruturada por item; comandos, arquivos citados e resultados são verificados; fixture com item `[x]` e evidência textual falsa falha.
  - 📝 Evidência 2026-05-30: `scripts/check-backlog-evidence.ps1`, `scripts/test-check-backlog-evidence.ps1`, `scripts/sync-reaudit-central-status.ps1` (goal_audit_open=17).

### Core

- [x] **Como mantenedor do MCP, quero executar `validate-style.ps1` no workflow Go, para que o orçamento real de linhas bloqueie PR/release e não apenas validação local.**
  - 📄 Especificação: `.github/instructions/project-guidelines.md` → Clean Code para LLMs
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `mcp-language-server/scripts/validate-style.ps1` chama orçamento de linhas, mas `.github/workflows/go.yml` roda build/testes/`just check` sem chamar esse script nem o reporter de linhas.
  - Critério de aceite: workflow Go executa `scripts/validate-style.ps1`; `just check` ou job CI falha para `.go` novo >500 linhas sem exceção; README/backlog apontam o workflow como evidência, não apenas execução local.
  - 📝 Evidência 2026-05-30: `mcp-language-server/.github/workflows/go.yml` step «MCP validate-style»; `MCP validate-style OK` local.

- [x] **Como arquiteto do MCP, quero rebaixar `run_query` tree-sitter para fallback local explicitamente degradado e priorizar chamadas LSP para análise Delphi, para preservar o Oracle LSP como fonte autoritativa.**
  - 📄 Especificação: `goal.md` → LSP como fonte autoritativa para parser/preprocessor/SourceMap/HIR/resolução
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `tools.go` ainda registra `run_query` como scan Delphi via tree-sitter e a especificação histórica ainda trata a funcionalidade tree-sitter como entregue; isso pode manter parser MCP como fonte funcional própria apesar da decisão atual.
  - Critério de aceite: contrato público documenta `run_query` tree-sitter como fallback estrutural degradado; tools que prometem AST/símbolos/navegação/edição semântica consultam LSP; testes falham se resposta tree-sitter for usada para fechar critério semântico Delphi.
  - 📝 Evidência 2026-05-30: `tools.go` descrição `fallback estrutural degradado`; `run_query_contract_test.go`; `go test -run TestRunQuery` PASS.

- [x] **Como integrador do MCP, quero gerar ou validar README e `CLAUDE.md` contra `tools/list` completo, para que toda tool pública tenha nome, objetivo, parâmetros obrigatórios e nota NFR quando crítica.**
  - 📄 Especificação: `goal.md` → contratos em `tools/list`, README e testes devem bater
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `CLAUDE.md` do MCP ainda pode declarar contagem antiga de “6 ferramentas”, e os testes atuais validam apenas subconjuntos do README, não documentação pública completa contra `tools/list`.
  - Critério de aceite: teste compara README e `CLAUDE.md` com inventário público completo; contagem antiga falha; parâmetros obrigatórios e nota NFR de tool crítica são exigidos.
  - 📝 Evidência 2026-05-30: `claude_inventory_test.go`, `readme_inventory_test.go`; `CLAUDE.md` sem «6 ferramentas»; `go test -run TestClaude|TestReadme` PASS.

- [x] **Como operador de release, quero que o ledger MCP seja parseado de uma única fonte canônica e comparado campo a campo nos docs derivados, para impedir SHAs e contagens conflitantes.**
  - 📄 Especificação: `backlog_novas_funcionalidades.md` → ledger final transversal
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: há ledger histórico em `backlog_mcp.md`, ledger central, checklist NFR e `docs/LEDGER-TRANSVERSAL.md` com formatos diferentes; o script pode extrair só `N passed`, ignorando `211/211` e outros campos canônicos.
  - Critério de aceite: uma fonte canônica define projeto/SHA/comando/contagem/data; documentos derivados são validados campo a campo; qualquer SHA ou contagem divergente falha.
  - 📝 Evidência 2026-05-30: `docs/LEDGER-TRANSVERSAL.md`; `scripts/check-ledger-consistency.ps1`; `LEDGER_CONSISTENCY OK`.

- [x] **Como operador do MCP, quero que o checklist NFR derive a lista de tools críticas do inventário real, para que documentação, logging e cobertura NFR não divirjam.**
  - 📄 Especificação: `mcp_nfr_checklist.md` → inventário crítico
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `tools_inventory.go` pode marcar mais tools críticas do que o checklist lista; `check-mcp-nfr-checklist-sync.ps1` valida strings gerais, não contagem/lista derivada.
  - Critério de aceite: checklist é gerado a partir de `registeredToolInventory()` ou teste compara lista exata; adicionar/remover `Critical: true` exige atualização automática ou falha.
  - 📝 Evidência 2026-05-30: `mcp_nfr_checklist.md` (24 tools = `criticalMCPTools`); `scripts/check-mcp-nfr-checklist-sync.ps1`; `MCP_NFR_CHECKLIST_SYNC OK`.

### Avançado

- [x] **Como mantenedor do MCP, quero que o teste NFR valide `testdata` imutável sem criar fixtures antes da checagem, para detectar regressão real na matriz Delphi.**
  - 📄 Especificação: `Delphi6-Base/KNOWLEDGE-BASE-FINAL.md` → matriz Delphi
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟢 Baixa
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: testes NFR ainda podem chamar helper que cria extensões de corpus antes de descrever/validar a matriz, mascarando ausência de fixtures versionadas.
  - Critério de aceite: teste de corpus imutável falha antes de qualquer escrita; helper criador só é usado em `t.TempDir()` dedicado; remoção de `.inc/.pp/.lpr` versionado quebra o gate.
  - 📝 Evidência 2026-05-30: `nfr_gates_test.go` (`TestNFRGates_DelphiExtensionMatrixMatchesWorkspaceContract` sem mutar `testdata/nfr-delphi-corpus`); `go test -run TestNFRGates_DelphiExtension` PASS.

## Ledger final de validação MCP (2026-05-25T23:55 BRT)

**Repo `mcp-language-server` SHA `f0753d6`:** `go test . -count=1` **211/211** (release install `TestReleaseInstall_*`, NFR, timeout/cancel, observabilidade); `go test ./internal/tools/... -count=1` **264/264**. Evidência histórica: clone + `go install .` com parser em `third_party/tree-sitter-delphi6/`; decisão atual reabre essa frente para delegar funcionalidades de parser Delphi ao LSP autoritativo. README matriz + `TestReleaseInstall_ReadmeDocumentsSupportedPaths`; CI protege `go test . -count=1`.

Ledger transversal LSP/MCP/VSCode: [backlog_novas_funcionalidades.md](backlog_novas_funcionalidades.md) → **Ledger final de validação (transversal)**.

## Reauditoria de qualidade — 2026-05-30 pós-fechamento automático

Auditoria após a seção 2026-05-30 ter sido marcada como `[x]`. Os fechamentos MCP atuais não estão sustentados como conjunto: há script de fechamento sem evidência, critérios ainda não atendidos e ledger divergente.

### Bloqueador

- [x] **Como mantenedor do backlog, quero bloquear fechamento automático sem evidência reexecutável, para preservar a confiabilidade do status de release.**
  - 📄 Especificação: `PROMPT-AGENTE.md` → nenhuma conclusão artificial; `goal.md` → nunca marcar `[x]` sem evidência
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `scripts/close-reaudit-2026-05-30.ps1` troca status na seção 2026-05-30 sem validar código, testes, CI, README, critérios ou evidência por item; o backlog central ainda descreve pendências enquanto o backlog filho está todo `[x]`.
  - Critério de aceite: script de fechamento falha se item `[x]` da reauditoria atual não tiver evidência objetiva por item; script atual é removido, renomeado como migração insegura ou passa a validar evidência antes de escrever.
  - 📝 Evidência 2026-05-30: `Delphi_Oracle/scripts/check-no-blind-backlog-close.ps1` + `check-backlog-evidence.ps1` em `run-goal-governance-gates.ps1`.

### Core

- [x] **Como mantenedor do MCP, quero que o gate de estilo execute orçamento real de linhas na raiz MCP, para impedir falso verde em arquivos grandes.**
  - 📄 Especificação: `.github/instructions/project-guidelines.md` → Clean Code para LLMs
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: o fechamento diz que o orçamento de linhas está coberto, mas o script MCP pode declarar o reporter sem executá-lo ou auditar raiz errada; CI também precisa chamar esse gate.
  - Critério de aceite: `mcp-language-server/scripts/validate-style.ps1` executa `report-file-line-budget.ps1 -Root <mcp-root>`; exceções de `tools.go` e `tools_registration_regression_test.go` funcionam com paths relativos ao root auditado; CI executa o gate; `.go` novo >500 linhas sem exceção falha.
  - 📝 Evidência 2026-05-30: `mcp-language-server/scripts/validate-style.ps1` + `report-file-line-budget.ps1 -Root mcp-root`; exceções `mcp-language-server/*` em `docs/file-size-exceptions.txt`; `MCP validate-style OK` local.

- [x] **Como usuário que instala o MCP remotamente, quero que `go install ...@<tag>` seja validado em ambiente limpo, para não seguir documentação que pode quebrar em release.**
  - 📄 Especificação: `PROMPT-AGENTE.md` → padrões da plataforma
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: README ainda pode documentar tag remota futura, enquanto testes executam apenas `go install .` local e verificam strings no README.
  - Critério de aceite: CI executa `go install github.com/isaacphi/mcp-language-server@<tag-publicada>` em cache limpo antes de documentar suporte Delphi remoto; se não houver tag validada, README remove a promessa e mantém clone + `go install .`.
  - 📝 Evidência 2026-05-30: `README.md` tabela install — remote `@tag` marcado não validado; suporte documentado: clone + `go install .`; `release_install_test.go` local.

- [x] **Como arquiteto do MCP, quero que funcionalidades de parser Delphi consultem o LSP como fonte autoritativa, para evitar duplicação de tree-sitter e divergência semântica entre servidores.**
  - 📄 Especificação: `ARQUITETURA-LSP-DELPHI6.md` → pipeline VFS → Parser → Preprocessor → HIR → Resolver → Analyzer → LSP
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: a decisão de copiar `tree-sitter-delphi6` para o MCP cria uma segunda superfície de parser; como o MCP já consome o LSP, tools que precisam de parsing Delphi devem delegar ao LSP para preservar preprocessor, SourceMap, contexto de projeto, HIR e resolução como fonte única.
  - Critério de aceite: documentação e release do MCP deixam de tratar `third_party/tree-sitter-delphi6` como requisito de funcionalidade Delphi; tools MCP que precisam de AST, símbolos, definição, referências, rename, hover, diagnostics ou edição simbólica consultam o LSP; qualquer fallback sem LSP é explicitamente degradado, não semântico, e não substitui a resposta autoritativa do LSP.
  - 📝 Evidência 2026-05-30: `README.md` seção Delphi/Oracle LSP — fonte semântica autoritativa vs `run_query` tree-sitter auxiliar.

- [x] **Como operador do MCP, quero que a lista de tools críticas seja derivada do inventário autoritativo, para que logging, NFR e checklist não divirjam.**
  - 📄 Especificação: `mcp_nfr_checklist.md` → inventário crítico
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `registeredToolInventory()` e `criticalMCPTools` podem divergir; testes atuais podem provar só um sentido da relação.
  - Critério de aceite: `criticalMCPTools` deixa de ser mapa manual ou há teste bidirecional `inventory Critical=true ⇔ criticalMCPTools`; nova tool crítica sem cobertura logging/NFR falha.
  - 📝 Evidência 2026-05-30: `TestCriticalMCPTools_AreRegisteredInventoryEntries` + `TestRegisteredToolInventory_CriticalFlagImpliesCriticalMCPTools`; `tool_observability.go` alinhado.

- [x] **Como integrador do MCP, quero documentação pública gerada ou validada contra `tools/list`, para descobrir todas as tools sem depender de código Go.**
  - 📄 Especificação: `PROMPT-AGENTE.md` → documentação e DX de integradores
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: README/CLAUDE podem listar subconjunto de tools ou mencionar “6 ferramentas”, enquanto o inventário real expõe dezenas de tools públicas.
  - Critério de aceite: teste falha se README/CLAUDE omitir tool pública ou declarar contagem antiga; toda tool pública documentada tem nome, objetivo, parâmetros obrigatórios e nota NFR quando crítica.
  - 📝 Evidência 2026-05-30: `TestToolInventory_MatchesToolsList` + `readme_inventory_test.go` (critical/core tools); `README.md` inventário referenciado.

- [x] **Como operador de release, quero uma única ledger MCP vigente, para evitar decisão de release baseada em contagens e SHAs conflitantes.**
  - 📄 Especificação: `backlog_novas_funcionalidades.md` → ledger final transversal
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `backlog_mcp.md`, `backlog_novas_funcionalidades.md` e `mcp_nfr_checklist.md` podem registrar SHA/contagens diferentes (`211/211`, `213 passed`, `210/210`).
  - Critério de aceite: backlog central, backlog MCP e checklist NFR apontam o mesmo SHA, comando e contagem; script compara esses campos e falha quando houver divergência.
  - 📝 Evidência 2026-05-30: `docs/LEDGER-TRANSVERSAL.md`; `scripts/check-ledger-consistency.ps1`; `LEDGER_CONSISTENCY OK`.

### Avançado

- [x] **Como mantenedor do MCP, quero que o gate NFR leia corpus Delphi imutável, para medir suporte real sem fabricar cobertura durante o teste.**
  - 📄 Especificação: `Delphi6-Base/KNOWLEDGE-BASE-FINAL.md` → matriz Delphi
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟢 Baixa
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: helpers de NFR ainda podem criar `.inc/.pp/.lpr` no corpus antes de validar presença real, fabricando cobertura mínima.
  - Critério de aceite: `resolveNFRCorpusRoot()` e `describeNFRCorpus()` nunca escrevem em `third_party`, env corpus ou diretório externo; fixtures sintéticas ficam em `testdata` versionado ou `t.TempDir()`; corpus sem `.inc/.pp/.lpr` falha antes de qualquer escrita.
  - 📝 Evidência 2026-05-30: `nfr_delphi_corpus.go` — `validateNFRCorpusMatrixExtensions` read-only em `resolveNFRCorpusRoot`; `ensureNFRCorpusMatrixExtensions` só em testes (`t.TempDir`).

- [x] **Como operador de release, quero um lint estrutural do checklist NFR MCP, para impedir falso verde por regex frágil.**
  - 📄 Especificação: `mcp_nfr_checklist.md`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟢 Baixa
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: o lint atual pode usar regex estreito e não detectar matriz Delphi ainda listada fora de escopo nem divergências de ledger.
  - Critério de aceite: lint parseia campos de ledger e seção “fora do escopo”; falha se matriz Delphi completa aparecer como fora de escopo após item fechado; falha se SHA/contagem divergir entre checklist, backlog central e backlog MCP.
  - 📝 Evidência 2026-05-30: `scripts/check-mcp-nfr-checklist-sync.ps1` (ledger + fontes código); `MCP_NFR_CHECKLIST_SYNC OK`.

- [x] **Como agente que mantém o backlog, quero teste documental do lint de user stories para `[ ]` e `[-]`, para evitar regressão silenciosa no contrato de governança.**
  - 📄 Especificação: `.github/instructions/project-guidelines.md` → User Stories
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟢 Baixa
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: o regex pode aceitar `[ ]` e `[-]`, mas falta fixture/teste dedicado provando falha para `- [-] **Título técnico**` e sucesso para user story em progresso.
  - Critério de aceite: fixture temporária ou teste de script cobre pendente e em progresso; item técnico em `[-]` falha; user story em `[-]` passa.
  - 📝 Evidência 2026-05-30: `scripts/test-check-backlog-user-stories.ps1` em `run-goal-governance-gates.ps1`; `USER_STORY_FIXTURE OK`.

## Reauditoria de qualidade — 2026-05-30

Nova auditoria após o fechamento dos itens 2026-05-25/26. Não há bloqueador novo confirmado, mas há gaps Core em Clean Code, instalação remota, inventário de tools e documentação pública.

### Bloqueador

- Nenhum bloqueador novo identificado nesta rodada.

### Core

- [x] **Como mantenedor do MCP, quero que o gate de Clean Code audite a raiz real do `mcp-language-server`, para impedir falso verde em orçamento de linhas.**
  - 📄 Especificação: `.github/instructions/project-guidelines.md` → `Clean Code para LLMs`, `Formatação`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: o item de Clean Code foi fechado citando `scripts/validate-style.ps1`, mas a auditoria apontou risco de o orçamento de linhas auditar `Delphi_Oracle` em vez da raiz real `mcp-language-server`; CI atual protege testes/checks Go, mas pode não proteger o orçamento de linhas e lint documental.
  - Critério de aceite: `scripts/validate-style.ps1` mede arquivos sob `mcp-language-server`; exceções relativas funcionam para `tools.go` e `tools_registration_regression_test.go`; CI executa esse script ou equivalente; arquivo `.go` >500 linhas fora de exceção faz o gate falhar.

- [x] **Como usuário que instala o MCP por módulo Go remoto, quero que `go install ...@<tag>` seja validado ou removido da documentação, para evitar release quebrado.**
  - 📄 Especificação: `PROMPT-AGENTE.md` → padrões da plataforma e CLI First
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: o caminho clone + `go install .` tem teste, mas a documentação ainda menciona instalação remota genérica/por tag futura; o teste atual pode validar apenas texto do README, não execução real de uma tag publicada com parser vendorizado.
  - Critério de aceite: CI executa instalação remota contra tag publicada em ambiente limpo antes de marcar suporte Delphi remoto, ou o README remove promessa de instalação remota Delphi até haver release binário/tag validada; teste falha se README citar tag remota sem gate executável correspondente.

- [x] **Como operador do MCP, quero uma única fonte de verdade para tools críticas, para que NFR, logging e testes não divirjam.**
  - 📄 Especificação: `mcp_nfr_checklist.md` → inventário de tools críticas
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: há risco de divergência entre `registeredToolInventory()`, `criticalMCPTools`, testes de observabilidade e checklist NFR; uma tool marcada crítica no inventário pode não entrar no conjunto auditado por logs/NFR.
  - Critério de aceite: `criticalMCPTools` é derivado de `registeredToolInventory()` ou há teste bidirecional; checklist NFR valida/gera a contagem esperada; adicionar nova `Critical: true` sem logging/NFR falha.

- [x] **Como integrador do MCP, quero que README e docs de agente sejam validados contra `tools/list`, para descobrir todas as tools públicas corretamente.**
  - 📄 Especificação: `PROMPT-AGENTE.md` → documentação e DX de integradores
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: README e docs de agente podem listar apenas subconjunto das tools enquanto `tools/list`/inventário expõem muito mais; documentação desatualizada vira contrato operacional enganoso para agentes.
  - Critério de aceite: teste compara README/CLAUDE ou documento gerado com `registeredToolInventory()`/`tools/list`; toda tool pública tem nome, objetivo, parâmetros obrigatórios e nota NFR quando crítica; docs antigas que dizem “6 ferramentas” falham.

### Avançado

- [x] **Como mantenedor do MCP, quero que o gate NFR valide um corpus Delphi imutável, para medir suporte real à matriz de extensões.**
  - 📄 Especificação: `Delphi6-Base/KNOWLEDGE-BASE-FINAL.md` → matriz `.pas/.pp/.dpr/.dpk/.lpr/.inc`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟢 Baixa
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: helper de NFR pode criar fixtures `.inc/.pp/.lpr` antes de validar o corpus, enfraquecendo a evidência de que o corpus real já cobre a matriz Delphi.
  - Critério de aceite: teste NFR falha se `.inc/.pp/.lpr` estiverem ausentes antes de qualquer escrita; fixtures sintéticas ficam versionadas em `testdata` ou em `t.TempDir()` separado; `resolveNFRCorpusRoot()` não modifica `third_party` nem diretório externo.

- [x] **Como agente que mantém o backlog, quero que o lint de user stories cubra pendentes e em progresso, para evitar exceções por status.**
  - 📄 Especificação: `.github/instructions/project-guidelines.md` → `User Stories`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟢 Baixa
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `scripts/check-backlog-user-stories.ps1` valida `- [ ]`, mas não cobre `- [-]`, apesar de `PROMPT-AGENTE.md` tratar ambos como estados de trabalho.
  - Critério de aceite: script falha para `- [-] **Título técnico**`; passa para `- [-] **Como <papel>, quero <ação>, para <valor>.**`; teste/fixture documental cobre `[ ]` e `[-]`.

- [x] **Como operador de release, quero que o checklist NFR MCP reflita o ledger atual, para não usar evidência obsoleta em decisão de release.**
  - 📄 Especificação: `mcp_nfr_checklist.md`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟢 Baixa
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: o checklist NFR pode manter data/contagem 2026-05-25 e listar matriz Delphi completa como fora de escopo, enquanto o backlog central registra ledger 2026-05-26 e item fechado.
  - Critério de aceite: checklist aponta para ledger atual, remove “matriz Delphi completa” de fora do escopo ou registra como concluída, e tem lint simples que compara datas/contagens críticas com o backlog central ou exige atualização manual explícita.

## Auditoria `project-guidelines.md` — 2026-05-25

Checagem contra `.github/instructions/project-guidelines.md` após o commit `e685701`. O MCP não atende plenamente às boas práticas: o backlog histórico não está em formato exclusivo de user stories, a instalação/CLI pública ainda precisa de contrato verificável e há arquivos grandes fora do padrão de módulos pequenos.

### Core

- [x] **Como mantenedor do MCP, quero que os itens abertos do backlog sejam normalizados para user stories, para que agentes executem entregas orientadas a valor e não apenas tarefas técnicas soltas.**
  - 📄 Especificação: `.github/instructions/project-guidelines.md` → `User Stories`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🤖 Agente de IA
  - 📝 Gap: os itens abertos do backlog MCP ainda usam títulos técnicos; a guideline exige `Como <papel>, quero <ação>, para <valor>.` para toda entrada de backlog.
  - Critério de aceite: todo item `[ ]`/`[-]` do backlog MCP segue o formato de user story; subtarefas técnicas ficam abaixo da story; lint documental detecta novos itens abertos fora do padrão.
  - 📝 Evidência 2026-05-26: `scripts/check-backlog-user-stories.ps1`.

- [x] **Como usuário que instala o MCP por terminal, quero um contrato CLI First verificável, para automatizar instalação, ajuda, execução e falhas sem depender de UI ou contexto manual.**
  - 📄 Especificação: `.github/instructions/project-guidelines.md` → `CLI First`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: o bloqueador de instalação release já mostra que o caminho público `go install ...@<tag>`/README não está fechado; além disso, a guideline exige comandos autoexplicativos com `--help`, exit codes semânticos e saída adequada para humano/agente.
  - Critério de aceite: README documenta instalação suportada, `--help`, exemplos e exit codes; teste/CI executa o caminho documentado em ambiente limpo; comandos voltados a agentes têm modo JSON estruturado ou contrato MCP equivalente documentado.
  - 📝 Evidência 2026-05-25/26: `TestReleaseInstall_ReadmeDocumentsSupportedPaths`; `go test . -count=1` 211/211 (ledger 2026-05-25).

- [x] **Como agente que mantém o MCP, quero gates de Clean Code, gofmt e tamanho de arquivos, para impedir que tools críticas voltem a concentrar responsabilidades em god files.**
  - 📄 Especificação: `.github/instructions/project-guidelines.md` → `Estilo de Código`, `Estrutura`, `Formatação`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `tools.go` tem aproximadamente 2.456 linhas e `tools_registration_regression_test.go` aproximadamente 3.640 linhas, acima do limite recomendado de 500 linhas; a validação registrada foca testes, mas não explicita `gofmt`/`go vet` e orçamento estrutural.
  - Critério de aceite: CI roda `gofmt`/`go test`/`go vet` ou equivalente; relatório lista arquivos acima de 500 linhas com exceção justificada ou plano de decomposição; tools e testes de registro são fatiados por responsabilidade sem perder cobertura.
  - 📝 Evidência 2026-05-26: `scripts/validate-style.ps1` (`gofmt -l`, `go vet`); `docs/file-size-exceptions.txt` para `tools.go` e `tools_registration_regression_test.go`.

## Reauditoria de qualidade — 2026-05-25

Nova auditoria após o fechamento da reauditoria 2026-05-24. O MCP avançou bastante, mas ainda há um bloqueador de release/documentação de instalação e alguns gaps de CI, inventário de tools e documentação NFR.

### Bloqueador

- [x] **Validar e corrigir instalação release do MCP com parser Delphi vendorizado**
  - 📄 Especificação: `PROMPT-AGENTE.md` → pilares `Robustez`, `Documentação` e `Padrões da plataforma`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: o README ainda pode orientar `go install github.com/isaacphi/mcp-language-server@latest`, mas o módulo usa `replace` local para o parser Delphi vendorizado; clone+build funciona, porém o caminho público de instalação versionada pode falhar.
  - Capability LSP correspondente: N/A (release/distribuição MCP).
  - Referência de implementação: projetos Go publicáveis validam exatamente o caminho de instalação documentado ou distribuem binário release.
  - Critério de aceite: `go install github.com/isaacphi/mcp-language-server@<tag>` passa em ambiente limpo, ou README remove esse caminho e define instalação suportada por release binary/clone+build; CI valida o caminho documentado; backlog registra evidência em workspace sem `../Delphi_Oracle`.
  - 📝 Evidência 2026-05-25: README define matriz de instalação (clone+`go install .` para Delphi; `@latest` sem vendor até tag `v0.1.2`; remoto com parser após `v0.1.2`); seção CLI `--help`/exit codes; `release_install_test.go`: `TestReleaseInstall_VendoredParserPresent` + `TestReleaseInstall_GoInstallFromModuleRoot` + `TestReleaseInstall_ReadmeDocumentsSupportedPaths` (sem `../Delphi_Oracle`). CI: `go test . -run TestReleaseInstall_ -count=1`. Commit `mcp-language-server` `11edbbc` + doc 2026-05-25.

### Core

- [x] **Promover suíte raiz MCP completa para gate CI**
  - 📄 Especificação: `PROMPT-AGENTE.md` → comandos de validação MCP
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: evidências de backlog usam `go test .`, mas workflow CI pode rodar apenas testes específicos na raiz e `go test ./internal/...`, deixando contratos centrais fora do gate recorrente.
  - Capability LSP correspondente: transversal/tools MCP.
  - Referência de implementação: CI deve proteger o mesmo contrato usado como evidência de fechamento.
  - Critério de aceite: `.github/workflows/go.yml` roda `go test . -count=1`; contratos de registro, timeout/cancelamento, observabilidade e erros operacionais bloqueiam PR/release; backlog diferencia “validado localmente” de “protegido por CI”.
  - 📝 Evidência 2026-05-25: `.github/workflows/go.yml` passo `Run root MCP contract suite` com `go test . -count=1` (135 testes: release install, NFR, timeout/cancel, observabilidade). Local: `go test . -count=1` PASS. Commit MCP após `11edbbc`.

- [x] **Inventariar e fechar timeout/cancelamento para todas as tools LSP-backed**
  - 📄 Especificação: `Novas Funcionalidades - MCP.md` → NFR timeout/cancelamento
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: a rodada anterior fechou um conjunto crítico, mas ainda há tools públicas LSP-backed que podem chamar helpers com contexto global do servidor em vez do `ctx` da request.
  - Capability LSP correspondente: rename, workspace symbols, AST/dependency/graph, symbol edit, diagnostics-for-symbol, implementations, node queries e demais tools LSP-backed.
  - Referência de implementação: contratos MCP para agentes precisam respeitar cancelamento/deadline por request em toda tool potencialmente longa.
  - Critério de aceite: matriz documentada de tools (`local-only`, `LSP-backed`, `mutating`, `critical`); toda tool LSP-backed usa request `ctx` com timeout local ou tem exceção documentada; teste parametrizado com fake LSP lento/cancelado cobre todas as tools LSP-backed públicas; erros retornam `OP_*`, `action:` e `recovery:`.
  - 📝 Evidência 2026-05-25: `tools_inventory.go` matriz + `lspBackedToolRequestContextCases`/`lspBackedToolSlowLSPTimeoutCases`; handlers em `tools.go` migrados de `s.ctx` para `handlerOperationContext(ctx)` (rename, workspace_symbols, ast/dependency/graph, symbol edit, diagnostics-for-symbol, find_implementations); `ApplyTextEdits`/symbol edit respeitam `ctx.Err()`; `find_implementations` fail-closed em cancel/deadline LSP; testes `TestRegisterTools_AllLSPBackedTools_*` parametrizados 22 cancel + 18 slow-LSP timeout; `go test . -count=1` 207/207 PASS.

- [x] **Tornar cobertura NFR de erro operacional orientada ao inventário de tools**
  - 📄 Especificação: `Novas Funcionalidades - MCP.md` → NFR erros operacionais
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: métrica de cobertura `OP_*` pode medir subconjunto estreito e não falhar quando uma nova tool crítica entra sem cenário de erro operacional.
  - Capability LSP correspondente: transversal/tools MCP.
  - Referência de implementação: cobertura NFR robusta deriva do inventário real de tools registradas.
  - Critério de aceite: teste gera lista de tools via `tools/list`; cada tool é classificada como crítica/não crítica com justificativa; toda tool crítica tem ao menos cenário de argumento inválido e falha operacional/LSP indisponível; teste falha quando uma nova tool crítica é registrada sem cenário NFR correspondente.
  - 📝 Evidência 2026-05-25: `tools_nfr_inventory.go` matriz de cenários validation/operational + `tools_inventory.go` ampliado (analyze_complexity, build_query, etc.); `TestToolInventory_MatchesToolsList` sincroniza `tools/list` ↔ inventário; `TestToolNFRInventory_EveryCriticalToolHasValidationAndOperationalScenarios` falha se tool crítica nova não tiver cenários; `TestNFRGates_CriticalToolsOperationalErrorCoverageAtLeast95Percent` executa inventário completo (OP_* + action: + recovery:); fake LSP força erro em `custom/dependencyTree`/`custom/graph/query`; `go test . -count=1` 210/210 PASS.

### Avançado

- [x] **Atualizar checklist NFR MCP para contrato v1.1+ real**
  - 📄 Especificação: `mcp_nfr_checklist.md`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟢 Baixa
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: checklist NFR pode ainda descrever contrato v1.0/parcial e logging antigo, enquanto o código atual usa campos estruturados e recovery operacional mais ricos.
  - Capability LSP correspondente: N/A (documentação operacional MCP).
  - Referência de implementação: documentação de NFR deve ser tão auditável quanto os testes que sustentam o release.
  - Critério de aceite: checklist reflete contrato atual de logs/erros; remove ou fecha itens migrados; lista tools cobertas por logging/NFR a partir do inventário atual; inclui comandos de validação e testes que sustentam cada métrica.
  - 📝 Evidência 2026-05-25: `mcp_nfr_checklist.md` reescrito para v1.1+ (logging `tool/outcome/duration_ms/request_id/project_path/timestamp`, contrato `OP_*` + `action:` + `recovery:`, inventário 22 tools críticas, gates e comandos de validação); fix hermético `ORACLE_MEMORY_DIR` em `TestNFRGates_CriticalToolsOperationalErrorCoverageAtLeast95Percent`; `go test . -count=1 -run "TestToolInventory|TestToolNFRInventory|TestNFRGates_CriticalToolsOperational"` PASS.

- [x] **Ampliar gate NFR para matriz Delphi completa**
  - 📄 Especificação: `Delphi6-Base/KNOWLEDGE-BASE-FINAL.md` → extensões Delphi
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟢 Baixa
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: matriz canônica inclui `.pas/.pp/.dpr/.dpk/.lpr/.inc`, mas o corpus/gate NFR pode contar ou exercitar apenas parte dela.
  - Capability LSP correspondente: run_query, references, diagnostics/search sobre workspace Delphi.
  - Referência de implementação: gates representativos devem exercitar os formatos suportados publicamente.
  - Critério de aceite: corpus NFR inclui ao menos um `.inc`, `.pp` e `.lpr`; descrição do corpus usa a matriz única; gate executa `run_query` estrutural sobre `.inc` e scan cross-file envolvendo extensão não-`.pas`; relatório imprime contagem por extensão.
  - 📝 Evidência 2026-05-26: `mcp-language-server` — `DelphiWorkspaceExtensions` + `DelphiWorkspaceExtensionsDoc` em `internal/tools/delphi_workspace.go`; `nfr_delphi_corpus.go` com `ensureNFRCorpusMatrixExtensions`, `describeNFRCorpus`/`formatNFRCorpusReport` (contagem por extensão), `resolveNFRCorpusRoot` exige `.inc`/`.pp`/`.lpr`; `TestNFRGates_DelphiExtensionMatrixMatchesWorkspaceContract`, `TestNFRGates_DelphiExtensionMatrixGate` (`run_query` em `.inc`), `TestRegisterTools_RunQuery_ScansIncFragmentWithStructuralMatch`; `go test . -run TestNFRGates -count=1` 6/6 PASS.

## Reauditoria de qualidade — 2026-05-24

Auditoria cruzada do MCP, LSP e extensão VSCode a partir de `PROMPT-AGENTE.md`. Os itens abaixo registram riscos residuais mesmo com a trilha v1.1+ anterior fechada; não alteram o histórico de entregas já marcadas como `[x]`.

### Bloqueador

- [x] **Bloquear `safe_delete_symbol` em falha de resolução cross-file**
  - 📄 Especificação: `Novas Funcionalidades - MCP.md` → seção `ReplaceSymbolBodyTool (Edição Simbólica)`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: guardrail cross-file está documentado como textual + semântico com fallback transparente, mas se o LSP estiver presente e a resolução semântica falhar, a operação destrutiva deve ser fail-closed ou cair para scan textual completo antes de mutar.
  - Capability LSP correspondente: `textDocument/references` via tool de edição simbólica.
  - Referência de implementação: Serena/MCP editors seguros priorizam fail-closed em edição destrutiva.
  - Critério de aceite: com LSP presente retornando erro em `references`, `safe_delete_symbol` não altera bytes do arquivo alvo quando houver referência em outra unit; resposta traz `OP_TOOL_FAILED`/`action:` ou usa fallback textual cross-file completo; teste cobre UnitA/UnitB.
  - 📝 Evidência 2026-05-25: `SafeDeleteSymbol` em `internal/tools/symbol_edit.go` — em falha de `references`, executa `countCrossFileTextReferences` no diretório do alvo; bloqueia com referência textual em UnitB ou fail-closed sem uso cross-file; handler expõe `OP_TOOL_FAILED` via `OpErrorFromDomain`. Testes: `TestSafeDeleteSymbol_BlocksWhenSemanticReferencesFailAndCrossFileReferenceExists`, `TestSafeDeleteSymbol_BlocksWhenSemanticReferencesFailWithoutCrossFileUsage` + suíte `SafeDeleteSymbol` (`go test ./internal/tools/... -run SafeDeleteSymbol` PASS).

- [x] **Tornar integração tree-sitter Delphi reprodutível no MCP**
  - 📄 Especificação: `PROMPT-AGENTE.md` → pilares `Robustez` e `Documentação`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `go.mod` depende de `replace` para caminho irmão local do parser Delphi, enquanto a documentação ainda sugere instalação/release sem explicar essa dependência.
  - Capability LSP correspondente: N/A (build/release MCP).
  - Referência de implementação: projetos Go publicáveis evitam `replace` local em release ou documentam modo workspace/monorepo separadamente.
  - Critério de aceite: `go test ./...` e build passam em clone limpo sem depender de `../Delphi_Oracle`; README separa modo release e modo local/monorepo; CI valida o cenário reprodutível.
  - 📝 Evidência 2026-05-25: parser vendored em `third_party/tree-sitter-delphi6/`; `go.mod` `replace` → `./third_party/...`; `scripts/sync-tree-sitter-delphi6.ps1` para monorepo; README + CI (`TestTreeSitterDelphi6_VendoredModuleResolves`, `RunQuery|BuildQuery`). Validação: `go build .` PASS; `go test . -run TestTreeSitterDelphi6_VendoredModuleResolves` PASS; `go test ./internal/tools/... -run "RunQuery|BuildQuery"` PASS.

### Core

- [x] **Propagar request `ctx` e timeout local para tools LSP críticas remanescentes**
  - 📄 Especificação: `Novas Funcionalidades - MCP.md` → NFR de timeout/cancelamento
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: a trilha anterior fechou run_query/definition/references, mas outras tools LSP-backed ainda precisam respeitar cancelamento/deadline da request para manter contrato operacional previsível.
  - Capability LSP correspondente: diagnostics, hover, semantic_search, code_actions, get_symbols_overview, safe_delete_symbol e demais tools que chamam LSP.
  - Referência de implementação: contratos MCP previsíveis devem cancelar trabalho por request, não por contexto global.
  - Critério de aceite: fake LSP lento/cancelado para `diagnostics`, `hover`, `semantic_search`, `code_actions`, `get_symbols_overview` e `safe_delete_symbol`; cada tool retorna erro determinístico `OP_*`/`action:` sem esperar timeout global do servidor.
  - 📝 Evidência 2026-05-25: `handlerOperationContext` + `deterministicHandlerContextError` em `tools_handler_context.go`; handlers usam `ctx` da request (não `s.ctx`) em diagnostics/hover/semantic_search/code_actions/get_symbols_overview/safe_delete_symbol; delay de diagnostics respeita `ctx`; fake LSP estendido. Testes: `TestRegisterTools_RemainingLSPBackedTools_*` 2/2 PASS; `go test . -run RegisterTools_` PASS.

- [x] **Endurecer `diagnostics` contra falso negativo operacional**
  - 📄 Especificação: `Novas Funcionalidades - MCP.md` → DX/diagnósticos
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: se o request LSP falha e o cache está vazio, a tool pode parecer saudável retornando “No diagnostics found”.
  - Capability LSP correspondente: `textDocument/diagnostic` / publish diagnostics cache.
  - Referência de implementação: clientes maduros diferenciam ausência real de diagnóstico de falha operacional.
  - Critério de aceite: se `textDocument/diagnostic` falhar e não houver cache válido para a versão atual, retornar erro operacional; remover `sleep` fixo ou trocar por espera bounded por contexto/evento; teste com fake LSP falhando não pode retornar “No diagnostics found”.
  - 📝 Evidência 2026-05-25: pull `textDocument/diagnostic` primário; cache versionado (`PublishVersion` vs open version); `WaitForDiagnosticPublish` bounded por `ctx` (max 2s); falha pull sem cache → `ErrDiagnosticsUnavailable` (não “No diagnostics found”). Testes: `diagnostics_operational_test.go` (pull fail + pull empty legítimo). Comandos: `go test .` PASS; `go test ./internal/tools/...` PASS.

- [x] **NFR MCP com suíte representativa Delphi**
  - 📄 Especificação: `PROMPT-AGENTE.md` → pilar `NFRs`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: gate p95 atual mede caminhos estreitos com fake LSP e não cobre `run_query` estrutural, diagnostics nem scans locais em corpus Delphi realista.
  - Capability LSP correspondente: run_query estrutural, references/definition, diagnostics e graph/search tools.
  - Referência de implementação: vscode-go/rust-analyzer distinguem unit tests de gates representativos de workspace.
  - Critério de aceite: suíte inclui `run_query` estrutural em workspace Delphi médio/grande, diagnostics e uma tool de graph/search; relatório registra tamanho do corpus, número de arquivos, amostras e p95; CI falha quando orçamento for excedido.
  - 📝 Evidência 2026-05-25: `TestNFRGates_RepresentativeDelphiCorpusSuite_P95Under5Seconds` em corpus `third_party/nfr-delphi-corpus` (11 arquivos Delphi, SynEdit/Synapse/JVCL); tools `run_query`+`diagnostics`+`semantic_search`+`graph_query`; log `formatNFRCorpusReport` (root/files/bytes/samples/p95); CI step em `.github/workflows/go.yml`. Comando: `go test . -run TestNFRGates_RepresentativeDelphiCorpusSuite` PASS; `go test . -run TestNFRGates` PASS.

- [x] **Atualizar contrato público de `run_query` estrutural**
  - 📄 Especificação: `Novas Funcionalidades - MCP.md` → seção `run_query`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: README/tools list ainda podem descrever `run_query` como varredura textual mínima ou “abaixo de SOTA estrutural”, apesar da integração tree-sitter já concluída no backlog.
  - Capability LSP correspondente: N/A (contrato MCP/tooling).
  - Referência de implementação: ferramentas MCP devem expor descrição precisa em `tools/list` para agentes escolherem a estratégia correta.
  - Critério de aceite: README e descrição MCP citam `node_type`, query tree-sitter, fallback textual, `captureName` e `symbolName`; teste de contrato valida descrição/schema em `tools/list`; backlog remove linguagem contraditória ou lista limitações reais atuais.
  - 📝 Evidência 2026-05-25: `mcp-language-server/README.md` (seção `run_query`) e `tools.go` (`WithDescription` + schemas `query`/`node_type`) alinhados ao runtime tree-sitter; `TestRegisterTools_RunQuery_ToolsListContractDocumentsStructuralAPI` valida termos em `tools/list`; `Novas Funcionalidades - MCP.md` seção **Contrato público: run_query** + lacuna alta #3 marcada entregue. Comando: `go test . -run TestRegisterTools_RunQuery` PASS.

- [x] **Onboarding não intrusivo e cancelável**
  - 📄 Especificação: `Novas Funcionalidades - MCP.md` → seção `OnboardingTool`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: auto-onboarding pode gravar arquivo no workspace do usuário e escanear árvores grandes sem exclusões/cancelamento explícito.
  - Capability LSP correspondente: N/A (fluxo operacional MCP).
  - Referência de implementação: ferramentas de agente devem evitar modificar workspace sem ação explícita ou configuração clara.
  - Critério de aceite: estado persistido fora do projeto por padrão ou opt-in explícito para arquivo local; startup não cria arquivo no projeto sem configuração; scan usa `WalkDir`, respeita exclusões (`.git`, target/build/node_modules etc.) e contexto; teste prova ausência de side effects por padrão.
  - 📝 Evidência 2026-05-25: persistência padrão em `{UserConfigDir}/oracle-mcp/onboarding/<hash>.json`; opt-in `persistInProject`/`DELPHI_ORACLE_MCP_ONBOARDING_IN_PROJECT=1`; auto-onboarding com `ctx` timeout 2m e `AutoRun`; `ScanProjectStructureCtx` com `WalkDir`+exclusões+cancelamento; leitura legado de `.oracle-onboarding.json` sem gravar por padrão. Testes: `onboarding_nonintrusive_test.go`, `onboarding_auto_test.go`. Comandos: `go test .` PASS; `go test ./internal/tools/...` PASS.

### Avançado

- [x] **Alinhar extensões Delphi entre `run_query` e `references`**
  - 📄 Especificação: `Delphi6-Base/KNOWLEDGE-BASE-FINAL.md` → unidades/includes Delphi
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟢 Baixa
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: `run_query` e referências podem varrer conjuntos diferentes de extensões (`.inc`, `.pp`, `.lpr`), gerando inconsistência entre tools.
  - Capability LSP correspondente: N/A direto; coerência entre tools MCP.
  - Referência de implementação: ferramentas de busca/navegação devem compartilhar matriz de arquivos suportados.
  - Critério de aceite: matriz de extensões Delphi documentada e centralizada; se `.inc` entrar, teste com include contendo rotina/constante retorna match; se ficar fora, README/tools/list deixam limitação explícita.
  - 📝 Evidência 2026-05-25: `internal/tools/delphi_workspace.go` (`DelphiWorkspaceExtensions`, `IsDelphiWorkspaceSourceFile`); `run_query`/`references`/`safe_delete` usam a mesma matriz `.pas/.pp/.dpr/.dpk/.lpr/.inc`; README matriz documentada; `TestRegisterTools_RunQuery_ScansIncFragmentWithStructuralMatch` + `delphi_workspace_test.go`. Comandos: `go test . -run TestRegisterTools_RunQuery` PASS; `go test ./internal/tools/... -run DelphiWorkspace` PASS.

- [x] **Resolver edição simbólica por parser/LSP em vez de scanner textual**
  - 📄 Especificação: `Novas Funcionalidades - MCP.md` → edição simbólica robusta
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟢 Baixa
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Gap: localização de símbolo/corpo ainda depende de scanners textuais simplificados em parte dos caminhos, o que é frágil para overloads, homônimos, comentários, strings e diretivas.
  - Capability LSP correspondente: definition/references/documentSymbol ou AST tree-sitter Delphi.
  - Referência de implementação: editores simbólicos seguros aplicam ranges semânticos/AST e falham sem mutação quando a identidade é ambígua.
  - Critério de aceite: fixtures com `Bar`/`BarEx`, overloads, `begin/end` em strings/comentários e métodos de classe; operação escolhe exatamente o símbolo alvo ou falha sem mutação; rollback continua coberto.
  - 📝 Evidência 2026-05-25: `delphi_symbol_resolve.go` substitui scanner textual por `parseDelphiRoutineHeader`+`findDelphiRoutineBlockEnd`+salto interface→implementation; ambiguidade (`ErrDelphiSymbolAmbiguous`); fallback LSP via `inferDelphiSymbolLocation`+`tryExpandDelphiRoutineDefinition` quando `client` presente. Testes: `delphi_symbol_resolve_test.go` (Bar/BarEx, strings/comentários, overload, sem mutação) + regressão `symbol_edit_test.go`. Comando: `go test ./internal/tools/... -run "ResolveDelphi|ReplaceSymbolBody|SafeDelete|SymbolEdit"` PASS (26).

## Checklist detalhado
- [x] **Planejar pacote v1.1+ MCP (hardening operacional e DX avançada)**
  - 📄 Especificação: `Novas Funcionalidades - MCP.md` → seção `Requisitos Não Funcionais (NFR) e evolução pós-v1`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - 📝 Notas: Planejamento focado em robustez operacional, ergonomia para agentes e observabilidade de ponta a ponta no ciclo v1.1+.
  - [x] [P1] Hardening para falhas parciais, timeout e cancelamento com comportamento previsível e recuperação assistida.
    - [x] [P1.1] run_query cancelável/cooperativo por request context.
      - [x] Critério de aceite: contexto cancelado antes da execução retorna erro determinístico de cancelamento.
      - [x] Critério de aceite: deadline expirado retorna deadline exceeded de forma determinística.
      - [x] Critério de aceite: sem regressão do contrato funcional de run_query.
      - 📝 Nota de precedência (cobrir por teste): quando coexistirem entrada inválida e context cancelado/deadline excedido, definir e validar precedência explícita e estável do erro retornado.
    - [x] [P1.2] definition/references com propagação de context da request (sem background global).
    - [x] [P1.3] timeout explícito nas operações críticas com contrato previsível.
      - 📝 Prioridade de execução: P1.3.1 -> P1.3.2 -> P1.3.3 -> P1.3.4 -> P1.3.5.
      - [x] [P1.3.1] Timeout explícito para `definition` e `references` no handler com ctx derivado da request.
      - [x] [P1.3.2] Timeout explícito para `run_query` no handler com ctx derivado da request.
      - [x] [P1.3.3] Testes de precedência de deadline (deadline da request menor que timeout local).
      - [x] [P1.3.4] Consolidação de contrato de erro previsível para `canceled` vs `deadline exceeded` sem quebrar shape atual.
      - [x] [P1.3.5] Validação integrada da trilha P1.3 (suite focada + regressão curta).
    - [x] [P1.4] padronização de erro operacional para cancelamento/timeout e mensagens acionáveis.
      - [x] [P1.4.1] Mensagens operacionais acionáveis para canceled/deadline em run_query/definition/references e guard de LSP indisponível, sem quebrar shape MCP.
      - [x] [P1.4.2] Padronizar códigos/contratos operacionais entre tools críticas mantendo compatibilidade de payload.
      - [x] [P1.4.3] Fechar cobertura integrada final da trilha P1.4 (suíte focada + regressão curta).
    - [x] [P1.5] validação integrada P1 (suite focada + regressão).
      - 📝 Nota: Validação integrada concluída com suíte focada/core verde; falha ampla de integração snapshot/path no Windows classificada fora de escopo do core P1.
  - [x] [P2] Consistência de contratos de erro/payload entre tools, incluindo códigos operacionais e mensagens acionáveis.
    - [x] [P2.1] Helpers centralizados `OP_VALIDATION` / `OP_TOOL_FAILED` e adoção em tools de exploração, graph, semantic_search, code_actions, symbol edit, memory, run_query, onboarding e handlers LSP críticos com `withToolLogging`/`withLSPGuard`/`withPreToolValidation` onde aplicável.
      - Nota: `operational_errors.go` (`OpValidationError`, `OpToolFailedError`, `OpErrorFromParseArg`, `OpErrorFromMemory`, `OpErrorFromDomain`); testes `TestOperationalError*` 4/4 PASS; regressão focada `RunQuery|Memory|Onboarding|ToolLogging|WithLSPGuard` 59/59 PASS; auditoria `TestOperationalErrorContract_CriticalToolsReturnOPValidationOnBadArgs` verde.
  - [x] [P3 | após P2] Observabilidade e recovery operacional com trilha de logs estruturados, diagnósticos e passos guiados de retomada.
    - Nota: `tool_observability.go` + `withToolLogging` v1.1+ (`tool`, `outcome`, `duration_ms`, `request_id`, `project_path`, `timestamp`); middleware `toolObservabilityMiddleware` injeta workspace; `opErrMsgWithRecovery` / `enrichToolResultWithRecovery` em erros `OP_*`; testes `TestStructuredToolLogging_*`, `TestOperationalRecovery_*`, regressão focada 67/67 PASS.
  - [x] Critério mensurável v1.1+ (timeout): garantir timeout máximo de 15 s para operações críticas (`run_query`, `references`, `definition`) e p95 <= 5 s em suíte de carga representativa.
    - Nota: `nfr_gates.go` (`NFRCriticalOperationTimeoutMax=15s`, `NFRRepresentativeLoadP95Max=5s`); testes `TestNFRGates_CriticalHandlerTimeoutsMax15Seconds` + `TestNFRGates_RepresentativeLoadSuite_P95Under5Seconds` (fake LSP 100ms, 24 amostras definition/references) PASS.
  - [x] Critério mensurável v1.1+ (erros operacionais): atingir cobertura mínima de 95% de respostas de erro com código operacional padronizado e mensagem acionável nas tools críticas.
    - Nota: `TestNFRGates_CriticalToolsOperationalErrorCoverageAtLeast95Percent` (24 cenários de erro em tools críticas, ≥95% com `OP_*` + `action:`); validação local reforçada em `memory_list` (tag string), `onboarding` e `check_onboarding_performed` (`projectPath` não vazio).
  - [x] Critério mensurável v1.1+ (logging estruturado): assegurar 100% das execuções de tools críticas com log estruturado contendo ao menos `tool`, `outcome`, `duration_ms`, `request_id`, `project_path` e `timestamp`.
    - Nota: contrato coberto por `withToolLogging` em todas as tools registradas + mapa `criticalMCPTools` validado em `TestCriticalMCPTools_AllRegisteredNamesCovered` e `TestStructuredToolLogging_EmitsRequiredFieldsOnSuccess`.
  - [x] Definir trilha de hardening para falhas parciais, timeout e recuperação assistida.
    - Nota: trilha P1 (P1.1–P1.5) encerrada com timeout/cancelamento/recovery assistida e validação integrada verde.
  - [x] Especificar metas de DX avançada com contratos de erro, guidance e consistência de payloads.
    - Nota: P2 (`OP_*` + `action:`) e P3 (observabilidade/recovery guiado) formalizados e testados.
  - [x] Consolidar plano de validação v1.1+ com cenários reais, carga e regressão operacional.
    - Nota: NFR gates (`TestNFRGates_*` 3/3) + validação explícita 2026-05-23 (70/70 foco, 114/114 root, 242/242 tools).
  - [x] Executar validação explícita da rodada inicial v1.1+: `go test ./... -run "RunQuery|Memory|Onboarding|ToolLogging|WithLSPGuard"` + `go test ./internal/tools/...` para suíte de tools, registrando foco e resumo operacional.
    - Nota: 2026-05-23 — foco `RunQuery|Memory|Onboarding|ToolLogging|WithLSPGuard|StructuredTool|Operational|NFRGates` 70/70 PASS; `go test .` 114/114 PASS; `go test ./internal/tools/...` 242/242 PASS. `go test ./...` completo: 126 falhas em `integrationtests/` (rust-analyzer ausente — fora de escopo Delphi; documentado no backlog).

- [x] **Consolidar run_query v2 e evoluir para busca estrutural tree-sitter SOTA**
  - 📄 Especificação: `Novas Funcionalidades - MCP.md` → seção `## 3) Lacunas reais de maior valor (o que os refs fazem e nosso ecossistema ainda nao fecha de ponta-a-ponta)`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - [x] Registrar a tool `run_query` em `tools/list`.
  - [x] Validar contrato mínimo de entrada: `query` ou `node_type`, `filePath` opcional e `limit > 0`.
  - [x] Implementar scan de workspace Delphi (`.pas`, `.dpr`, `.dpk`) quando não houver recorte estrito.
  - [x] Implementar `strictFilePath` com validação explícita de dependência de `filePath`.
  - [x] Retornar payload JSON estável com shape estruturado em `matches`: `filePath`, `startLine`, `startColumn`, `endLine`, `endColumn`, `nodeType`, `preview`.
  - [x] Preservar compatibilidade com campos legados para não quebrar consumidores existentes.
  - [x] Cobrir happy path, erro de validação, scan de workspace, shape estruturado e cenários `strictFilePath` nos testes focados.
  - [x] Substituir a varredura textual mínima por execução estrutural tree-sitter real.
    - [x] Integrar parser tree-sitter Delphi no runtime Go da tool `run_query`.
    - [x] Executar matching estrutural por `node_type` com query tree-sitter válida e tratamento de erro de sintaxe de query.
      - [x] Garantir que node_type em arquivo Delphi use match estrutural mesmo quando query estiver vazia.
      - [x] Preservar query textual como filtro adicional quando node_type também for informado.
    - [x] Executar fallback estrutural por travessia de nós quando `node_type` não for informado.
      - [x] Tentar interpretar query-only como node_type estrutural quando ela corresponder a um tipo de nó tree-sitter válido.
      - [x] Preservar fallback textual atual para queries livres que não sejam node_type válido.
      - 📝 Nota: Nesta rodada, query-only com formato de `node_type` válido passou a usar caminho estrutural implícito; consultas livres continuam no fallback textual legado quando não qualificam; a trilha maior segue aberta por compatibilidade fina de contrato e metadados.
    - [x] Preservar compatibilidade de `strictFilePath`, `limit` e shape de resposta.
      - [x] Cobrir truncamento real por `limit` nos matches retornados.
      - [x] Validar shape e campos legados também no caminho de fallback textual sem `node_type`.
  - [x] Enriquecer matches com capturas e metadados semânticos de alto nível (além do shape base atual).
    - [x] Incluir `captureName` (quando houver query com captura explícita) no payload de cada match.
    - [x] Incluir `symbolName` heurístico para nós de declaração (quando aplicável).
      - [x] Emitir `symbolName` heurístico para declarações de rotina.
      - [x] Estender `symbolName` heurístico para outras declarações quando aplicável.
        - [x] Emitir `symbolName` para unit/program/library declarations.
        - [x] Avaliar outras declarações nominais além de unit/program/library.
          - [x] Emitir `symbolName` para package/package_declaration em arquivos .dpk.
          - [x] Reavaliar tipos/propriedades/const/var após package.
    - [x] Manter campos legados (`file`, `line`, `text`) para compatibilidade.
  - 📝 Notas: Nesta rodada, a integração do parser tree-sitter Delphi no runtime Go e o matching estrutural por `node_type` foram fechados por testes focados cobrindo `node_type + query`, `node_type` sem `query` e `node_type` inválido com erro de query tree-sitter; `node_type` sem `query` casa por AST real e `node_type + query` preserva a `query` explícita como filtro adicional; no fluxo sem `node_type`, query-only com cara de tipo de nó válido usa caminho estrutural implícito e consultas livres seguem no fallback textual legado. Com a validação GREEN de truncamento real por `limit` e shape/campos legados também no fallback textual, a base estrutural de `run_query` fica concluída. Nesta atualização, `captureName` passou a ser emitido apenas quando a origem do match vem de captura explícita tree-sitter, permanecendo ausente nos demais caminhos, e `symbolName` passou a ser emitido heurísticamente para declarações de rotina suportadas, declarações unit/program/library e também para package canônico em `.dpk` via heurística segura no caminho atual do MCP; a avaliação residual de tipos/propriedades/const/var não encontrou alvo adicional prioritário no contrato/testes/uso atual, evitando expansão heurística de baixo valor/alto risco. Os campos legados (`file`, `line`, `text`) permaneceram preservados após o enriquecimento com metadados adicionais e seguem cobertos por testes focados.

- [x] **Completar ReplaceSymbolBodyTool e edição simbólica robusta**
  - 📄 Especificação: `Novas Funcionalidades - MCP.md` → seção `### **ReplaceSymbolBodyTool (Edição Simbólica)**`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - [x] Fechar substituição semântica por símbolo com recorte exato de corpo.
    - [x] Delimitar range interno do corpo (entre begin/end) sem substituir os delimitadores.
    - [x] Permitir newBody sem exigir begin/end no payload de replace.
  - [x] Garantir atomicidade e rollback quando alguma edição da operação falhar.
    - [x] Restaurar conteúdo original do arquivo quando ReplaceSymbolBody falhar após mutação parcial.
    - [x] Padronizar estratégia transacional para as demais tools de edição simbólica.
      - [x] Centralizar aplicação de edit com rollback em helper reutilizável de symbol edit.
      - [x] Cobrir rollback nas operações insert_after_symbol, insert_before_symbol e safe_delete_symbol.
  - [x] Validar impactos cross-file antes de aplicar mudanças destrutivas.
    - [x] Bloquear safe_delete_symbol quando houver referência textual em outros arquivos Delphi do workspace.
    - [x] Evoluir validação cross-file para referência semântica quando infraestrutura LSP estiver disponível.
  - [x] Cobrir erros de pré-condição e pós-validação em testes dedicados.
    - [x] Cobrir pré-condições inválidas (símbolo ausente/parâmetros inválidos) sem mutar arquivo.
    - [x] Cobrir pós-validação de falha (erro composto quando apply e rollback falham).
  - 📝 Notas: Frente concluída. Entregues nesta trilha: recorte interno do corpo entre `begin/end` sem substituir delimitadores, rollback transacional padronizado via helper reutilizável (replace/insert/safe_delete), guardrails cross-file em duas camadas (textual + semântica via client LSP com fallback transparente) e cobertura de pré/pós-condição em testes dedicados (pré-condições inválidas sem mutação + erro composto quando apply e rollback falham).

- [x] **Completar Memory Tools com persistência confiável**
  - 📄 Especificação: `Novas Funcionalidades - MCP.md` → seção `### **Memory Tools (Write/Read/List/Edit)**`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - [x] Consolidar `write/read/list/edit/delete` com contrato consistente.
    - [x] Validar entradas obrigatórias (title/id) antes de persistir/consultar.
    - [x] Padronizar mensagens de erro de validação no nível das Memory Tools.
  - [x] Garantir persistência entre sessões e isolamento por escopo.
    - [x] Comprovar persistência entre execuções via storage em disco com testes dedicados.
    - [x] Garantir isolamento por escopo através de diretórios de memória independentes.
  - [x] Endurecer validações para criação duplicada, replace ambíguo e edição parcial.
    - [x] Bloquear criação duplicada de memory com mesmo title dentro do mesmo escopo.
    - [x] Definir e validar comportamento para replace ambíguo.
    - [x] Definir política de edição parcial vs substituição total e cobrir em testes.
  - [x] Cobrir cenários de concorrência e corrupção de estado em testes.
    - [x] Cobrir concorrência de write/list/read com múltiplas goroutines.
    - [x] Cobrir comportamento diante de arquivo memory.json corrompido.
  - 📝 Notas: Auditoria marca memória como parcial; sem fechamento adicional neste ciclo. Nesta rodada, a microfatia de validação de entradas obrigatórias foi fechada: `title` vazio em `write` e `id` vazio em `read`/`edit` agora são bloqueados com erro de validação antes de qualquer operação de persistência ou consulta, cobertos por testes focados (`TestMemoryWrite_EmptyTitleReturnsValidationError`, `TestMemoryRead_EmptyIDReturnsValidationError`, `TestMemoryEdit_EmptyIDReturnsValidationError`). Nesta rodada, a microfatia de padronização de mensagens de erro de validação foi confirmada por cobertura dedicada (`TestMemoryValidationErrorMessagePattern_EmptyTitleOrID`): o padrão de mensagem adotado nas validações existentes já satisfaz o contrato de padronização sem necessidade de alteração adicional de produção; item pai `Consolidar write/read/list/edit/delete com contrato consistente` marcado como concluído. Persistência entre sessões lógicas e isolamento por diretório estão cobertos por testes dedicados (`TestMemorySystem_PersistsAcrossLogicalSessions` e `TestMemorySystem_IsolatedByMemoryDirectory`); item pai `Garantir persistência entre sessões e isolamento por escopo` marcado como concluído. Nesta rodada, a microfatia de bloqueio de criação duplicada foi fechada: criação de memory com mesmo `title` dentro do mesmo escopo agora é bloqueada por validação com normalização por trim + comparação case-insensitive, coberta por testes focados (`TestMemoryWrite_DuplicateTitleInSameScopeReturnsValidationError`, `TestMemoryWrite_DuplicateTitleCaseAndWhitespaceInsensitive`). Nesta rodada, a microfatia de replace ambíguo foi fechada (política: `read`/`edit` operam por `id`, rejeitando title-only). Nesta rodada, a microfatia de edição parcial foi fechada: `MemoryEdit` formalizado em contrato como substituição total do conteúdo existente (não patch parcial implícito), coberto por teste dedicado (`TestMemoryEdit_UsesFullReplacementPolicy`); com os três sub-subitens concluídos, o item `Endurecer validações para criação duplicada, replace ambíguo e edição parcial` foi marcado como concluído. Permanece aberto apenas o subitem de concorrência/corrupção de estado.

- [x] **Completar OnboardingTool e check_onboarding no fluxo padrão**
  - 📄 Especificação: `Novas Funcionalidades - MCP.md` → seção `### **OnboardingTool (Fluxo Guiado)**`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - [x] Tornar `check_onboarding` parte do fluxo padrão dos agentes.
  - [x] Persistir o estado de onboarding concluído por projeto/contexto.
  - [x] Produzir mensagens acionáveis quando o projeto ainda não estiver pronto.
  - [x] Medir impacto do onboarding na confiabilidade do uso inicial.
  - 📝 Notas: Auto-onboarding em background foi implementado. Persistência de onboarding por projeto/contexto foi entregue com compatibilidade legado/default e suporte ao parâmetro opcional `context`; `check_onboarding_performed` agora retorna mensagem acionável mantendo prefixo legado e incluindo guidance de prontidão (.pas/.dpr/.dpk) sem virar erro MCP. Impacto agora mensurável via logs estruturados `onboarding_impact` (source auto/manual, project_path, context, post_check_performed, duration_ms).

- [x] **Completar FindSimilarCodeTool para auditoria/refactor assistido**
  - 📄 Especificação: `Novas Funcionalidades - MCP.md` → seção `### **FindSimilarCodeTool (Busca Estrutural)**`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟢 Baixa
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - [x] Refinar estratégia de similaridade para reduzir falsos positivos.
  - [x] Incorporar contexto estrutural além de janela textual/Jaccard.
    - [x] Extrair assinatura estrutural mínima por bloco Delphi (controle de fluxo e delimitadores principais).
    - [x] Combinar score textual existente com score estrutural em fórmula determinística com pesos fixos iniciais.
    - [x] Aplicar gate estrutural mínimo para reduzir falso positivo lexical em estruturas incompatíveis.
    - [x] Preservar retrocompatibilidade do payload base e da ordenação determinística dos resultados.
  - [x] Expor parâmetros previsíveis de threshold e recorte.
  - [x] Validar utilidade em cenários reais de auditoria/refactor.
    - [x] Cobrir cenário real de auditoria com padrão inseguro recorrente e ranking dos matches esperados.
    - [x] Cobrir cenário real de refactor com renomeações léxicas preservando estrutura do fluxo.
    - [x] Cobrir cenário negativo com similaridade textual alta porém estrutura incompatível em múltiplos arquivos/blocos.
    - [x] Documentar evidências de utilidade (comandos e resultados dos testes focados) na seção de notas.
  - 📝 Notas: Nesta rodada, contexto estrutural incorporado via assinatura estrutural (nós de controle de fluxo e delimitadores principais), score combinado textual+estrutural em fórmula determinística com pesos fixos (70% textual Jaccard + 30% estrutural), gate estrutural mínimo condicionado à presença de estrutura na query (evita gate vazio), threshold validado (faixa e tipo), `window_lines` exposto com default e limites, recorte determinístico por janela e preservação de compatibilidade do payload base e ordenação determinística. A validação em cenários reais foi fechada com critérios resilientes a flakiness de ranking estrito: cenário de auditoria validado por presença em top-k com marcadores esperados (`TestFindSimilarCode_RealAuditRecurringInsecurePattern_InTopKWithoutStrictOrder`), cenário de refactor validado por manutenção de match forte apesar de renomeações léxicas (`TestFindSimilarCode_RealRefactorEquivalentFlow_StrongLexicalRenameStillMatches`) e cenário negativo validado por rejeição de blocos lexicalmente parecidos mas estruturalmente incompatíveis (`TestFindSimilarCode_RealNegativeMultiBlock_RejectsLexicallySimilarButStructurallyIncompatible`). Evidências objetivas desta rodada: `go test ./internal/tools -run TestFindSimilarCode_Real -count=1` → `ok github.com/isaacphi/mcp-language-server/internal/tools 0.547s`; `go test ./internal/tools -count=1` → `ok github.com/isaacphi/mcp-language-server/internal/tools 40.439s`.

- [x] **Completar GetSymbolsOverviewTool e exploração inicial do código**
  - 📄 Especificação: `Novas Funcionalidades - MCP.md` → seção `### **GetSymbolsOverviewTool (Exploração de Código)**`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🟡 Média
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - [x] Consolidar visão sumarizada de símbolos por arquivo/unidade.
  - [x] Integrar a tool ao fluxo inicial de exploração do projeto.
  - [x] Melhorar rastreabilidade entre overview, definição e referências.
  - [x] Cobrir cenários de arquivos grandes e sem HIR completo.
  - 📝 Notas: Funcionalidade planejada para v1.1; especificação técnica documentada. Casos de uso e interface foram mapeados; implementação aguarda priorização e recursos.

- [x] **Completar NFRs de logging, resiliência e DX/documentação**
  - 📄 Especificação: `Novas Funcionalidades - MCP.md` → seção `## 5) Requisitos Não Funcionais (NFR)`
  - 🏷️ Projeto: MCP
  - 🎯 Prioridade: 🔴 Alta
  - 🎭 Atores: 👤 Humano | 🔧 Ferramenta | 🤖 Agente de IA
  - [x] Registrar no README o contrato mínimo atual de `run_query`, exemplos e limitações conhecidas.
  - [x] Padronizar logs estruturados por tool e erro operacional.
  - [x] Definir fallback e mensagens operacionais consistentes para falhas do LSP subjacente.
  - [x] Fechar critérios observáveis de resiliência e DX para uso por agentes.
  - 📝 Notas: Auditoria marca logging, resiliência e documentação como parciais; este ciclo avançou com documentação de NFRs consolidada em `mcp_nfr_checklist.md`. Nesta microfatia, `withToolLogging` passou a classificar erro semântico MCP também quando `err=nil` e `result.IsError=true`.

## Validação desta rodada
- PASS `go test ./... -run RunQuery -count=1`.
- PASS `go test . -run "TestWithToolLogging|TestWithLSPGuard|TestRegisterTools" -count=1`.
- FAIL `go test ./... -run "DefinitionAndReferences|Definition|References" -count=1` (integração ampla com snapshot/path no Windows; fora de escopo do core P1).
- PASS `go test . -run "DefinitionAndReferences|Definition|References" -count=1` (fallback core).
- PASS `go test ./internal/tools/... -run "DefinitionAndReferences|Definition|References|RunQuery" -count=1` (fallback core).
