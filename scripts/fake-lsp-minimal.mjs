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
          renameProvider: true,
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
  if (msg.method === "textDocument/references") {
    const uri = msg.params?.textDocument?.uri ?? DEF_URI;
    writeMessage({
      jsonrpc: "2.0",
      id: msg.id,
      result: [
        {
          uri,
          range: {
            start: { line: 8, character: 4 },
            end: { line: 8, character: 10 },
          },
        },
      ],
    });
    return;
  }
  if (msg.method === "workspace/symbol") {
    writeMessage({
      jsonrpc: "2.0",
      id: msg.id,
      result: [
        {
          name: "TSmoke",
          kind: 5,
          location: {
            uri: DEF_URI,
            range: {
              start: { line: 4, character: 2 },
              end: { line: 4, character: 14 },
            },
          },
        },
      ],
    });
    return;
  }
  if (msg.method === "textDocument/rename") {
    const uri = msg.params?.textDocument?.uri ?? DEF_URI;
    const newName = msg.params?.newName ?? "Renamed";
    writeMessage({
      jsonrpc: "2.0",
      id: msg.id,
      result: {
        changes: {
          [uri]: [
            {
              range: {
                start: { line: 5, character: 2 },
                end: { line: 5, character: 8 },
              },
              newText: newName,
            },
            {
              range: {
                start: { line: 12, character: 10 },
                end: { line: 12, character: 16 },
              },
              newText: newName,
            },
          ],
        },
      },
    });
    return;
  }
  if (msg.method === "textDocument/codeAction") {
    writeMessage({
      jsonrpc: "2.0",
      id: msg.id,
      result: [
        {
          title: "Add unit Trim to uses",
          kind: "quickfix",
          edit: {
            changes: {
              [DEF_URI]: [
                {
                  range: {
                    start: { line: 0, character: 0 },
                    end: { line: 0, character: 0 },
                  },
                  newText: "",
                },
              ],
            },
          },
        },
      ],
    });
    return;
  }
  if (msg.method === "custom/dependencyTree") {
    const uri = msg.params?.uri ?? DEF_URI;
    const unitKey = uri.split("/").pop()?.replace(/\.pas$/i, "") ?? "smoke";
    const key = unitKey.toLowerCase();
    writeMessage({
      jsonrpc: "2.0",
      id: msg.id,
      result: {
        root: key,
        tree: {
          [key]: ["SysUtils", "Classes"],
        },
        treeBySection: {
          [key]: {
            interface: ["SysUtils"],
            implementation: ["Classes"],
          },
        },
        cycles: [],
      },
    });
    return;
  }
  if (msg.method === "custom/graph/query") {
    const uri = msg.params?.uri ?? DEF_URI;
    const root = uri.split("/").pop()?.replace(/\.pas$/i, "") ?? "smoke";
    writeMessage({
      jsonrpc: "2.0",
      id: msg.id,
      result: {
        root,
        relationType: msg.params?.relationType ?? "uses_unit",
        direction: msg.params?.direction ?? "both",
        depth: msg.params?.depth ?? 1,
        nodes: [
          { id: root, kind: "unit", resolved: true },
          { id: "SysUtils", kind: "unit", resolved: true },
        ],
        edges: [
          {
            source: root,
            target: "SysUtils",
            type: "uses_unit",
            direction: "imports",
          },
        ],
        stats: {
          nodeCount: 2,
          edgeCount: 1,
          truncated: false,
          rootResolved: true,
        },
      },
    });
    return;
  }
  if (msg.method === "custom/semanticSearch") {
    writeMessage({
      jsonrpc: "2.0",
      id: msg.id,
      result: {
        results: [
          {
            symbol: "TSmoke.Consume",
            unit: "smoke",
            score: 0.9,
            matchReason: "name + harness fixture",
            location: {
              uri: DEF_URI,
              range: {
                start: { line: 5, character: 2 },
                end: { line: 5, character: 14 },
              },
            },
          },
        ],
      },
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
