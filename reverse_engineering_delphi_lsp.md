# Engenharia Reversa: OmniPascal para LSP Delphi 6

Este documento detalha as descobertas da engenharia reversa da extensão OmniPascal (v0.19.0), com o objetivo de servir de base para a criação de um Language Server Protocol (LSP) para Delphi 6.

## 1. Visão Geral da Arquitetura

O OmniPascal utiliza uma arquitetura de cliente-servidor customizada, muito semelhante ao LSP oficial, mas com um protocolo próprio baseado em JSON via Standard I/O (stdin/stdout).

- **Cliente (VS Code Extension):** Escrito em TypeScript, responsável pela interface com o VS Code e por traduzir as requisições do editor para o protocolo do servidor.
- **Servidor (OmniPascalServer.exe):** Um binário compilado (provavelmente em Delphi ou Free Pascal) que realiza a análise sintática, indexação e lógica de linguagem.

## 2. Protocolo de Comunicação

A comunicação ocorre através de mensagens JSON enviadas pelo `stdin` e recebidas pelo `stdout` do processo do servidor.

### Formato de Requisição (Client -> Server)
```json
{
  "seq": 1,
  "type": "request",
  "command": "nomeDoComando",
  "arguments": { ... }
}
```

### Formato de Resposta (Server -> Client)
```json
{
  "request_seq": 1,
  "success": true,
  "body": { ... }
}
```

### Formato de Evento (Server -> Client)
```json
{
  "type": "event",
  "event": "nomeDoEvento",
  "body": { ... }
}
```

## 3. Catálogo de Comandos Identificados

| Comando | Descrição | Parâmetros Principais |
| :--- | :--- | :--- |
| `setConfig` | Envia configurações (caminhos, etc) | `omnipascal.*`, `workspacePaths` |
| `open` | Notifica que um arquivo foi aberto | `file` (caminho absoluto) |
| `close` | Notifica que um arquivo foi fechado | `file` |
| `change` | Envia mudanças incrementais ou totais | `file`, `line`, `offset`, `insertString` |
| `geterr` | Solicita diagnóstico de erros | `files`, `delay` |
| `completions` | Code Completion | `file`, `line`, `offset` |
| `definition` | Go to Definition | `file`, `line`, `offset` |
| `quickinfo` | Hover / Tooltip de informações | `file`, `line`, `offset` |
| `signatureHelp` | Ajuda de parâmetros de função | `file`, `line`, `offset` |
| `textDocument/documentSymbol` | Símbolos do arquivo (Outline) | `file` |
| `workspace/symbol` | Busca de símbolos no projeto | `query` |
| `getProjectFiles` | Lista arquivos de projeto (.dpr, .lpr) | - |
| `loadProject` | Carrega um projeto específico | `file` |
| `getAllUnits` | Lista todas as units conhecidas | `file` |
| `getPossibleUsesSections` | Identifica seções 'uses' (interface/impl) | `filename` |
| `addUses` | Adiciona uma unit à seção uses | `filename`, `usesSection`, `unitToAdd` |
| `getCodeActions` | Obtém correções rápidas (Quick Fix) | `filename`, `selection` |
| `runCodeAction` | Executa uma ação de código | `filename`, `identifier`, `selection` |

## 4. Sincronização de Buffers (Editor -> Servidor)

O OmniPascal envia o conteúdo do arquivo via comando `change`. 
- **Curiosidade:** O cliente parece enviar o texto completo do documento codificado via `encodeURIComponent` no campo `insertString`.
- **Delphi 6:** Para arquivos grandes de Delphi 6, recomenda-se usar mudanças incrementais para performance.

## 5. Sistema de Diagnósticos

O servidor envia eventos assíncronos de diagnóstico:
- `syntaxDiag`: Erros de sintaxe encontrados durante o parse inicial.
- `semanticDiag`: Erros semânticos (tipos, variáveis não declaradas) após análise mais profunda.

## 6. Configurações Cruciais para Delphi 6

Para que um LSP funcione com Delphi 6, os seguintes caminhos devem ser configurados (como visto no `package.json` do OmniPascal):

1.  **delphiInstallationPath:** Caminho para o binário do compilador (ex: `C:\Program Files\Borland\Delphi6`).
2.  **searchPath:** Lista de diretórios contendo `.pas` e `.dcu`. O Delphi 6 depende fortemente do `Library Path`.
3.  **workspacePaths:** Onde o código fonte do usuário reside.

## 7. Desafios Específicos do Delphi 6

- **Formatos de Arquivo:** Delphi 6 usa `.dpr` para projetos e `.dfm` (binário ou texto) para formulários. O LSP precisa ignorar ou processar `.dfm` se quiser oferecer suporte a componentes.
- **Case Insensitivity:** Pascal não diferencia maiúsculas de minúsculas. O servidor deve lidar com isso na indexação e busca.
- **Units Padrão:** É necessário indexar as units da VCL/RTL do Delphi 6 (System, SysUtils, Classes, Forms, Controls, etc.).

## 8. Estratégia Recomendada para Implementação

1.  **Parser:** Utilizar um parser de Delphi/Object Pascal (ex: baseado em Antlr ou um parser manual em Pascal se o LSP for escrito em Pascal).
2.  **Protocolo:** Embora o OmniPascal use um protocolo customizado, recomenda-se usar o **LSP Padrão (JSON-RPC)** para compatibilidade com outros editores além do VS Code.
3.  **Indexação:** Criar um índice de símbolos (procedures, functions, types, variables) que mapeie o nome para a localização (arquivo, linha, coluna).
4.  **Resolução de Units:** Implementar a lógica de busca de units seguindo a ordem: Pasta do Projeto -> Search Path -> Library Path do Delphi 6.

---
*Documento gerado para auxiliar no desenvolvimento de ferramentas de modernização de legado Delphi 6.*
