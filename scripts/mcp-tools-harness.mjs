#!/usr/bin/env node
/**
 * Harness CLI MCP: tools/list e tools/call via stdio (CLI First).
 *
 *   node scripts/mcp-tools-harness.mjs --help
 *   node scripts/mcp-tools-harness.mjs list --workspace <dir>
 *   node scripts/mcp-tools-harness.mjs call --workspace <dir> --tool <name> --args-json '{}'
 */
import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const MCP_ROOT = path.resolve(__dirname, "..");

function usage() {
  console.log(`MCP tools harness (stdio JSON-RPC)

Subcomandos:
  list    Lista tools via tools/list (JSON stdout, exit 0)
  call    Chama tools/call com --tool e --args-json

Flags comuns:
  --workspace <dir>   Workspace (obrigatorio)
  --timeout-ms <n>    Timeout (default 120000)
  --help

Exemplo:
  node scripts/mcp-tools-harness.mjs list --workspace .
  node scripts/mcp-tools-harness.mjs call --workspace . --tool onboarding --args-json "{}"
`);
}

function parseCommon(argv) {
  const out = { timeoutMs: 120_000 };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === "--help" || a === "-h") out.help = true;
    else if (a === "--workspace") out.workspace = path.resolve(argv[++i]);
    else if (a === "--timeout-ms") out.timeoutMs = Number(argv[++i]);
    else if (!out.sub && (a === "list" || a === "call")) out.sub = a;
    else if (a === "--tool") out.tool = argv[++i];
    else if (a === "--args-json") out.argsJson = argv[++i];
    else throw new Error(`argumento desconhecido: ${a}`);
  }
  return out;
}

class JsonRpcClient {
  constructor(proc) {
    this.proc = proc;
    this.nextId = 1;
    this.pending = new Map();
    this.buffer = "";
    proc.stdout.on("data", (c) => this.onData(c.toString("utf8")));
    proc.stderr.on("data", (c) => process.stderr.write(c));
  }

  onData(chunk) {
    this.buffer += chunk;
    while (true) {
      const headerEnd = this.buffer.indexOf("\r\n\r\n");
      if (headerEnd < 0) return;
      const header = this.buffer.slice(0, headerEnd);
      const match = /Content-Length:\s*(\d+)/i.exec(header);
      if (!match) {
        this.buffer = this.buffer.slice(headerEnd + 4);
        continue;
      }
      const len = Number(match[1]);
      const bodyStart = headerEnd + 4;
      if (this.buffer.length < bodyStart + len) return;
      const body = this.buffer.slice(bodyStart, bodyStart + len);
      this.buffer = this.buffer.slice(bodyStart + len);
      try {
        const msg = JSON.parse(body);
        if (msg.id !== undefined && this.pending.has(msg.id)) {
          const { resolve, reject } = this.pending.get(msg.id);
          this.pending.delete(msg.id);
          if (msg.error) reject(new Error(JSON.stringify(msg.error)));
          else resolve(msg.result);
        }
      } catch {
        /* skip */
      }
    }
  }

  request(method, params) {
    const id = this.nextId++;
    const payload = JSON.stringify({ jsonrpc: "2.0", id, method, params });
    const frame = `Content-Length: ${Buffer.byteLength(payload, "utf8")}\r\n\r\n${payload}`;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.proc.stdin.write(frame, "utf8", (err) => err && reject(err));
    });
  }

  notify(method, params) {
    const payload = JSON.stringify({ jsonrpc: "2.0", method, params });
    const frame = `Content-Length: ${Buffer.byteLength(payload, "utf8")}\r\n\r\n${payload}`;
    this.proc.stdin.write(frame, "utf8");
  }

  close() {
    this.proc.stdin.end();
    this.proc.kill();
  }
}

function spawnMcp(workspace) {
  const bin = path.join(
    MCP_ROOT,
    process.platform === "win32" ? "mcp-language-server.exe" : "mcp-language-server",
  );
  const cmd = existsSync(bin) ? bin : "go";
  const args = existsSync(bin)
    ? ["--workspace", workspace]
    : ["run", ".", "--workspace", workspace];
  return spawn(cmd, args, { stdio: ["pipe", "pipe", "pipe"], cwd: MCP_ROOT });
}

async function withMcpSession(workspace, timeoutMs, fn) {
  const proc = spawnMcp(workspace);
  const client = new JsonRpcClient(proc);
  const timer = setTimeout(() => client.close(), timeoutMs);
  try {
    await client.request("initialize", {
      protocolVersion: "2024-11-05",
      capabilities: {},
      clientInfo: { name: "mcp-tools-harness", version: "1.0.0" },
    });
    client.notify("notifications/initialized", {});
    return await fn(client);
  } finally {
    clearTimeout(timer);
    client.close();
  }
}

async function main() {
  const argv = process.argv.slice(2);
  const opts = parseCommon(argv);
  if (opts.help || !opts.sub) {
    usage();
    process.exit(opts.help ? 0 : 1);
  }
  if (!opts.workspace) {
    console.error(JSON.stringify({ ok: false, error: "--workspace obrigatorio" }));
    process.exit(1);
  }

  if (opts.sub === "list") {
    const tools = await withMcpSession(opts.workspace, opts.timeoutMs, (c) =>
      c.request("tools/list", {}),
    );
    console.log(JSON.stringify({ ok: true, tools }, null, 2));
    process.exit(0);
  }

  if (opts.sub === "call") {
    if (!opts.tool) {
      console.error(JSON.stringify({ ok: false, error: "--tool obrigatorio" }));
      process.exit(1);
    }
    const args = opts.argsJson ? JSON.parse(opts.argsJson) : {};
    const result = await withMcpSession(opts.workspace, opts.timeoutMs, (c) =>
      c.request("tools/call", { name: opts.tool, arguments: args }),
    );
    console.log(JSON.stringify({ ok: true, tool: opts.tool, result }, null, 2));
    process.exit(0);
  }

  usage();
  process.exit(1);
}

main().catch((err) => {
  console.error(JSON.stringify({ ok: false, error: String(err.message ?? err) }));
  process.exit(1);
});
