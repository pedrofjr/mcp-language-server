#!/usr/bin/env node
/**
 * Harness CLI MCP: tools/list e tools/call via stdio (CLI First).
 */
import { spawn } from "node:child_process";
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const MCP_ROOT = path.resolve(__dirname, "..");
const DEFAULT_FAKE_LSP = path.join(MCP_ROOT, "scripts", "fake-lsp-minimal.mjs");

function usage() {
  console.log(`MCP tools harness (stdio JSON-RPC)

Subcomandos:
  list    Lista tools via tools/list (JSON stdout, exit 0)
  call    Chama tools/call com --tool e --args-json

Flags comuns:
  --workspace <dir>   Workspace (obrigatorio)
  --lsp <cmd>         Comando LSP (default: node scripts/fake-lsp-minimal.mjs)
  --lsp-args <json>   Args JSON array para o LSP apos --
  --timeout-ms <n>    Timeout (default 120000)
  --help

Exemplo:
  node scripts/mcp-tools-harness.mjs list --workspace . --lsp node --lsp-args '["scripts/fake-lsp-minimal.mjs"]'
`);
}

function parseCommon(argv) {
  const out = { timeoutMs: 120_000, lspCommand: "", lspArgs: [] };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === "--help" || a === "-h") out.help = true;
    else if (a === "--workspace") out.workspace = path.resolve(argv[++i]);
    else if (a === "--lsp") out.lspCommand = argv[++i];
    else if (a === "--lsp-args") out.lspArgs = JSON.parse(argv[++i]);
    else if (a === "--timeout-ms") out.timeoutMs = Number(argv[++i]);
    else if (!out.sub && (a === "list" || a === "call")) out.sub = a;
    else if (a === "--tool") out.tool = argv[++i];
    else if (a === "--args-json") out.argsJson = argv[++i];
    else if (a === "--args-file") out.argsJson = readFileSync(argv[++i], "utf8");
    else throw new Error(`argumento desconhecido: ${a}`);
  }
  return out;
}

/** MCP stdio uses newline-delimited JSON (mcp-go ServeStdio), not LSP Content-Length. */
class NdjsonRpcClient {
  constructor(proc, requestTimeoutMs) {
    this.proc = proc;
    this.requestTimeoutMs = requestTimeoutMs;
    this.nextId = 1;
    this.pending = new Map();
    this.buffer = "";
    proc.stdout.on("data", (c) => this.onData(c.toString("utf8")));
    proc.stderr.on("data", (c) => process.stderr.write(c));
  }

  rejectAllPending(reason) {
    for (const { reject, timer } of this.pending.values()) {
      if (timer) clearTimeout(timer);
      reject(new Error(reason));
    }
    this.pending.clear();
  }

  onData(chunk) {
    this.buffer += chunk;
    while (true) {
      const lineEnd = this.buffer.indexOf("\n");
      if (lineEnd < 0) return;
      const line = this.buffer.slice(0, lineEnd).trim();
      this.buffer = this.buffer.slice(lineEnd + 1);
      if (!line) continue;
      try {
        const msg = JSON.parse(line);
        if (msg.id !== undefined && this.pending.has(msg.id)) {
          const { resolve, reject, timer } = this.pending.get(msg.id);
          this.pending.delete(msg.id);
          if (timer) clearTimeout(timer);
          if (msg.error) reject(new Error(JSON.stringify(msg.error)));
          else resolve(msg.result);
        }
      } catch {
        /* skip non-json log lines */
      }
    }
  }

  request(method, params) {
    const id = this.nextId++;
    const payload = JSON.stringify({ jsonrpc: "2.0", id, method, params });
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        if (this.pending.has(id)) {
          this.pending.delete(id);
          reject(new Error(`request timeout: ${method}`));
        }
      }, this.requestTimeoutMs);
      this.pending.set(id, { resolve, reject, timer });
      this.proc.stdin.write(`${payload}\n`, "utf8", (err) => err && reject(err));
    });
  }

  notify(method, params) {
    const payload = JSON.stringify({ jsonrpc: "2.0", method, params });
    this.proc.stdin.write(`${payload}\n`, "utf8");
  }

  close() {
    this.rejectAllPending("client closed");
    this.proc.stdin.end();
    this.proc.kill();
  }
}

function resolveDefaultLsp() {
  return {
    command: process.execPath,
    args: [DEFAULT_FAKE_LSP],
  };
}

function spawnMcp(workspace, lspCommand, lspArgs) {
  const bin = path.join(
    MCP_ROOT,
    process.platform === "win32" ? "mcp-language-server.exe" : "mcp-language-server",
  );
  const mcpCmd = existsSync(bin) ? bin : "go";
  const mcpRunArgs = existsSync(bin)
    ? ["--workspace", workspace, "--lsp", lspCommand, "--", ...lspArgs]
    : ["run", ".", "--workspace", workspace, "--lsp", lspCommand, "--", ...lspArgs];
  return spawn(mcpCmd, mcpRunArgs, { stdio: ["pipe", "pipe", "pipe"], cwd: MCP_ROOT });
}

function assertSymbolAbsentOnDisk(filePath, symbolName) {
  const member = symbolName.includes(".")
    ? symbolName.split(".").pop()
    : symbolName;
  const disk = readFileSync(filePath, "utf8");
  const qualifiedPattern = new RegExp(
    `\\b${member.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}\\b`,
    "i",
  );
  if (qualifiedPattern.test(disk)) {
    return {
      ok: false,
      error: `simbolo solicitado "${symbolName}" ainda presente no arquivo`,
    };
  }
  return { ok: true };
}

function testSafeDeleteDiskNegativeFixture() {
  const dir = mkdtempSync(path.join(os.tmpdir(), "mcp-safe-del-neg-"));
  const filePath = path.join(dir, "symbol_mutate.pas");
  writeFileSync(
    filePath,
    ["unit U;", "implementation", "procedure DeleteMe;", "begin", "end.", "end."].join(
      "\n",
    ),
    "utf8",
  );
  const check = assertSymbolAbsentOnDisk(filePath, "DeleteMe");
  rmSync(dir, { recursive: true, force: true });
  if (check.ok) {
    console.error(
      JSON.stringify({
        ok: false,
        error: "safe-delete-disk-negative: deveria falhar com DeleteMe ainda no disco",
      }),
    );
    return 1;
  }
  console.log(JSON.stringify({ ok: true, test: "safe-delete-disk-negative" }, null, 2));
  return 0;
}

async function withMcpSession(opts, fn) {
  const lsp = opts.lspCommand
    ? { command: opts.lspCommand, args: opts.lspArgs ?? [] }
    : resolveDefaultLsp();
  const proc = spawnMcp(opts.workspace, lsp.command, lsp.args);
  const client = new NdjsonRpcClient(proc, opts.timeoutMs);
  const sessionTimer = setTimeout(() => {
    client.rejectAllPending("session timeout");
    client.close();
  }, opts.timeoutMs * 2);

  try {
    await client.request("initialize", {
      protocolVersion: "2024-11-05",
      capabilities: {},
      clientInfo: { name: "mcp-tools-harness", version: "1.0.0" },
    });
    client.notify("notifications/initialized", {});
    return await fn(client);
  } finally {
    clearTimeout(sessionTimer);
    client.close();
  }
}

async function main() {
  const argv = process.argv.slice(2);
  if (argv.includes("--test-safe-delete-negative")) {
    process.exit(testSafeDeleteDiskNegativeFixture());
  }
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
    const tools = await withMcpSession(opts, (c) => c.request("tools/list", {}));
    const count = tools?.tools?.length ?? 0;
    if (count === 0) {
      console.error(JSON.stringify({ ok: false, error: "tools/list vazio" }));
      process.exit(1);
    }
    console.log(JSON.stringify({ ok: true, toolCount: count, tools }, null, 2));
    process.exit(0);
  }

  if (opts.sub === "call") {
    if (!opts.tool) {
      console.error(JSON.stringify({ ok: false, error: "--tool obrigatorio" }));
      process.exit(1);
    }
    const args = opts.argsJson ? JSON.parse(opts.argsJson) : {};
    const result = await withMcpSession(opts, (c) =>
      c.request("tools/call", { name: opts.tool, arguments: args }),
    );
    if (result?.isError) {
      console.error(JSON.stringify({ ok: false, tool: opts.tool, isError: true, result }));
      process.exit(1);
    }
    const payload = result?.content ?? result;
    if (
      payload === undefined ||
      payload === null ||
      (Array.isArray(payload) && payload.length === 0)
    ) {
      console.error(
        JSON.stringify({ ok: false, tool: opts.tool, error: "tools/call sem payload util" }),
      );
      process.exit(1);
    }
    const textBlob = JSON.stringify(payload).toLowerCase();
    if (opts.tool === "hover") {
      if (
        textBlob.includes("no hover information") ||
        !textBlob.includes("harness") && !textBlob.includes("tsmoke")
      ) {
        console.error(
          JSON.stringify({
            ok: false,
            tool: opts.tool,
            error: "hover sem payload semantico LSP (fallback generico)",
          }),
        );
        process.exit(1);
      }
    }
    if (opts.tool === "definition") {
      const hasSource =
        textBlob.includes("procedure") &&
        (textBlob.includes("consume") || textBlob.includes("tsmoke"));
      if (!hasSource) {
        console.error(
          JSON.stringify({
            ok: false,
            tool: opts.tool,
            error: "definition sem corpo/fonte Delphi semantico (procedure/consume)",
          }),
        );
        process.exit(1);
      }
    }
    if (opts.tool === "diagnostics") {
      if (!textBlob.includes("e001") && !textBlob.includes("diagnostic")) {
        console.error(
          JSON.stringify({
            ok: false,
            tool: opts.tool,
            error: "diagnostics sem codigo/mensagem semantica",
          }),
        );
        process.exit(1);
      }
    }
    if (opts.tool === "references") {
      if (
        !textBlob.includes("range") &&
        !textBlob.includes("uri") &&
        !textBlob.includes("reference") &&
        !textBlob.includes("tsmoke")
      ) {
        console.error(
          JSON.stringify({
            ok: false,
            tool: opts.tool,
            error: "references sem conteudo semantico",
          }),
        );
        process.exit(1);
      }
    }
    if (opts.tool === "workspace_symbols") {
      if (!textBlob.includes("tsmoke")) {
        console.error(
          JSON.stringify({
            ok: false,
            tool: opts.tool,
            error: "workspace_symbols sem simbolo esperado",
          }),
        );
        process.exit(1);
      }
    }
    if (opts.tool === "code_actions") {
      if (!textBlob.includes("quickfix") && !textBlob.includes("trim")) {
        console.error(
          JSON.stringify({
            ok: false,
            tool: opts.tool,
            error: "code_actions sem quickfix/trim semantico",
          }),
        );
        process.exit(1);
      }
    }
    let callArgs = {};
    try {
      callArgs = JSON.parse(opts.argsJson ?? "{}");
    } catch {
      callArgs = {};
    }
    if (opts.tool === "edit_file") {
      const targetPath = callArgs.filePath;
      if (!targetPath || typeof targetPath !== "string") {
        console.error(
          JSON.stringify({
            ok: false,
            tool: opts.tool,
            error: "edit_file sem filePath nos args para verificacao em disco",
          }),
        );
        process.exit(1);
      }
      if (!existsSync(targetPath)) {
        console.error(
          JSON.stringify({
            ok: false,
            tool: opts.tool,
            error: `edit_file alvo inexistente: ${targetPath}`,
          }),
        );
        process.exit(1);
      }
      const disk = readFileSync(targetPath, "utf8");
      if (!disk.includes("Marker = 42")) {
        console.error(
          JSON.stringify({
            ok: false,
            tool: opts.tool,
            error: "edit_file: arquivo em disco sem mutacao Marker = 42",
          }),
        );
        process.exit(1);
      }
    }
    if (opts.tool === "rename_symbol") {
      const renamed =
        textBlob.includes("successfully renamed") &&
        textBlob.includes("tsmokerenamed") &&
        textBlob.includes("updated");
      if (!renamed) {
        console.error(
          JSON.stringify({
            ok: false,
            tool: opts.tool,
            error: "rename_symbol sem rename LSP aplicado (TSmokeRenamed/Updated)",
          }),
        );
        process.exit(1);
      }
      const targetPath = callArgs.filePath;
      if (targetPath && existsSync(targetPath)) {
        const disk = readFileSync(targetPath, "utf8");
        if (!disk.includes("TSmokeRenamed")) {
          console.error(
            JSON.stringify({
              ok: false,
              tool: opts.tool,
              error: "rename_symbol: arquivo em disco sem TSmokeRenamed",
            }),
          );
          process.exit(1);
        }
      }
    }
    const diskMarkers = {
      replace_symbol_body: "HARNESS_BODY_REPLACED",
      insert_after_symbol: "HARNESS_AFTER",
      insert_before_symbol: "HARNESS_BEFORE",
    };
    if (diskMarkers[opts.tool]) {
      const targetPath = callArgs.filePath;
      const marker = diskMarkers[opts.tool];
      if (!targetPath || !existsSync(targetPath)) {
        console.error(
          JSON.stringify({
            ok: false,
            tool: opts.tool,
            error: `${opts.tool} sem filePath para verificacao em disco`,
          }),
        );
        process.exit(1);
      }
      const disk = readFileSync(targetPath, "utf8");
      if (!disk.includes(marker)) {
        console.error(
          JSON.stringify({
            ok: false,
            tool: opts.tool,
            error: `${opts.tool}: arquivo em disco sem token esperado ${marker}`,
          }),
        );
        process.exit(1);
      }
    }
    if (opts.tool === "safe_delete_symbol") {
      const targetPath = callArgs.filePath;
      const symbolName = String(callArgs.symbolName ?? "").trim();
      if (!targetPath || !existsSync(targetPath)) {
        console.error(
          JSON.stringify({
            ok: false,
            tool: opts.tool,
            error: "safe_delete_symbol sem filePath para verificacao em disco",
          }),
        );
        process.exit(1);
      }
      if (!symbolName) {
        console.error(
          JSON.stringify({
            ok: false,
            tool: opts.tool,
            error: "safe_delete_symbol sem symbolName para verificacao em disco",
          }),
        );
        process.exit(1);
      }
      const absent = assertSymbolAbsentOnDisk(targetPath, symbolName);
      if (!absent.ok) {
        console.error(
          JSON.stringify({
            ok: false,
            tool: opts.tool,
            error: `safe_delete_symbol: ${absent.error}`,
          }),
        );
        process.exit(1);
      }
    }
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
