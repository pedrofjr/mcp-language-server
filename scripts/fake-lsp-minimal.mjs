#!/usr/bin/env node
/** Minimal LSP stub for mcp-tools-harness (initialize + shutdown only). */
let buffer = "";

function writeMessage(message) {
  const payload = Buffer.from(JSON.stringify(message), "utf8");
  process.stdout.write(
    Buffer.from(`Content-Length: ${payload.length}\r\n\r\n`, "ascii"),
  );
  process.stdout.write(payload);
}

function handleMessage(msg) {
  if (msg.method === "initialize") {
    writeMessage({
      jsonrpc: "2.0",
      id: msg.id,
      result: {
        capabilities: {},
        serverInfo: { name: "fake-lsp-minimal", version: "1.0.0" },
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
    writeMessage({
      jsonrpc: "2.0",
      id: msg.id,
      result: null,
    });
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
