# Diretrizes do Projeto

## Estilo de Código

- Este repositório tem como alvo o Go 1.24.0. Mantenha as alterações compatíveis com a versão declarada no `go.mod`.
- Prefira as receitas do `justfile` em vez de comandos shell ad hoc: use `just fmt`, `just test`, `just check`, `just build` e `just generate`.
- Siga a divisão de pacotes existente em `internal/` e mantenha as mudanças locais à camada responsável pelo comportamento.
- Use o logger de componentes do `internal/logging` em vez de introduzir o uso direto do logger da biblioteca padrão.
- Mantenha a saída da ferramenta MCP estável e intencional. Snapshots de integração confirmam o formato exato da saída, incluindo caminhos de arquivos, intervalos e blocos de texto formatados.

## Arquitetura

- `main.go` analisa as flags da CLI, valida o workspace e o comando LSP, inicia o cliente LSP, registra as ferramentas MCP e controla o comportamento de encerramento (shutdown).
- `internal/lsp/` gerencia o subprocesso do language server, o transporte JSON-RPC/LSP, manipuladores de requisição e detecção de linguagem.
- `internal/protocol/` contém tipos do protocolo LSP gerados e ajudantes (helpers) de compatibilidade. Prefira regerar o código em vez de ediçōes manuais ao alterar a superfície do protocolo.
- `internal/tools/` implementa as ferramentas voltadas para o MCP, como definição, referências, diagnósticos, hover, renomear e edições de arquivos.
- `internal/utilities/` contém ajudantes de nível mais baixo, como a aplicação de edição de texto e preservação de finais de linha.
- `internal/watcher/` rastreia alterações de arquivos no workspace e as sincroniza com o language server conectado.
- `integrationtests/` executa language servers reais em workspaces de teste (fixtures) e valida os resultados com snapshots.

## Build e Testes

- Execute `just fmt` antes de finalizar alterações em código Go.
- Execute `just test` para a verificação normal.
- Execute `just check` ao mexer em infraestrutura compartilhada, código gerado ou em qualquer coisa com probabilidade de afetar os critérios de qualidade (quality gates) de todo o repositório.
- Execute `just generate` após alterações nas entradas de geração de protocolo em `cmd/generate/`, `internal/protocol/` ou métodos LSP gerados.
- Execute `just snapshot` apenas quando uma mudança na saída for intencional. Revise as diferenças de snapshot em vez de atualizá-las mecanicamente.
- Testes de snapshot e de integração requerem que os language servers relevantes estejam instalados localmente: `gopls`, `pyright`, `rust-analyzer`, `typescript-language-server` e `clangd`, dependendo do conjunto (suite) que está sendo executado.

## Convenções

- Não edite manualmente o código de protocolo gerado, a menos que a alteração seja deliberadamente fora do fluxo do gerador; prefira atualizar as entradas do gerador e regerar o código.
- Preserve o tratamento de CRLF/LF ao trabalhar com lógica de edição de arquivos. A camada de utilitários foi projetada para manter intactos os finais de linha originais.
- Mantenha as alterações de log alinhadas com os nomes de componentes existentes e com o comportamento de log (`LOG_LEVEL`), de modo que a depuração (debugging) permaneça consistente em todos os pacotes.
- Testes de integração copiam workspaces de teste para diretórios temporários e normalizam os caminhos nos snapshots. Ao investigar falhas, verifique `integrationtests/test-output/` e quaisquer arquivos `.diff` gerados.
- O servidor espera um caminho real de workspace e um executável LSP disponível no `PATH`. Evite mudanças que enfraqueçam essas validações, a menos que o comportamento esteja sendo intencionalmente redesenhado.