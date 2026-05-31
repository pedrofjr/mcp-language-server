#!/usr/bin/env node
/** Minimal LSP stub with semantic payloads for MCP tools/call harness. */
let buffer = "";

const HOVER_MARKDOWN = "**TSmoke** — harness fixture (Delphi LSP-backed)";
const DEF_URI = "file:///harness/smoke.pas";

function writeMessage(message) {
  const payload = Buffer.from(JSON.stringify(message), "utf8");
  process.stdout.write(
    Buffer.from(`Content-Length: ${payload.length}\r\n\r\n`, "ascii"),
  );
  process.stdout.write(payload);
}

function semanticHover() {
  return { contents: { kind: "markdown", value: HOVER_MARKDOWN } };
}

function semanticDefinition(params) {
  const uri = params?.textDocument?.uri ?? DEF_URI;
  return {
    uri,
    range: {
      start: { line: 4, character: 2 },
      end: { line: 4, character: 14 },
    },
  };
}

function semanticDiagnostics(params) {
  const uri = params?.textDocument?.uri ?? DEF_URI;
  return [
    {
      range: {
        start: { line: 0, character: 0 },
        end: { line: 0, character: 1 },
      },
      message: "Harness diagnostic E001",
      severity: 1,
      source: "fake-lsp-minimal",
      code: "E001",
    },
  ];
}

function handleMessage(msg) {
  if (msg.method === "initialize") {
    writeMessage({
      jsonrpc: "2.0",
      id: msg.id,
      result: {
        capabilities: {
          hoverProvider: true,
          definitionProvider: true,
        },
        serverInfo: { name: "fake-lsp-minimal", version: "1.1.0" },
      },
    });
    return;
  }
  if (msg.method === "textDocument/hover") {
    writeMessage({ jsonrpc: "2.0", id: msg.id, result: semanticHover() });
    return;
  }
  if (msg.method === "textDocument/definition") {
    writeMessage({
      jsonrpc: "2.0",
      id: msg.id,
      result: semanticDefinition(msg.params),
    });
    return;
  }
  if (
    msg.method === "textDocument/diagnostic" ||
    msg.method === "textDocument/documentDiagnostic"
  ) {
    writeMessage({
      jsonrpc: "2.0",
      id: msg.id,
      result: { kind: "full", items: semanticDiagnostics(msg.params) },
    });
    return;
  }
  if (msg.method === "shutdown") {
    writeMessage({ jsonrpc: "2.0", id: msg.id, result: null });
    return;
  }
  if (msg.method === "exit") {
    process.exit(0);
  }
  if (msg.id !== undefined) {
    writeMessage({ jsonrpc: "2.0", id: msg.id, result: null });
  }
}

process.stdin.on("data", (chunk) => {
  buffer += chunk.toString("utf8");
  while (true) {
    const headerEnd = buffer.indexOf("\r\n\r\n");
    if (headerEnd < 0) return;
    const header = buffer.slice(0, headerEnd);
    const match = /Content-Length:\s*(\d+)/i.exec(header);
    if (!match) {
      buffer = buffer.slice(headerEnd + 4);
      continue;
    }
    const len = Number(match[1]);
    const bodyStart = headerEnd + 4;
    if (buffer.length < bodyStart + len) return;
    const body = buffer.slice(bodyStart, bodyStart + len);
    buffer = buffer.slice(bodyStart + len);
    try {
      handleMessage(JSON.parse(body));
    } catch {
      /* ignore */
    }
  }
});
