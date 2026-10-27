#!/usr/bin/env node
// Launcher: runs the native binary that npm installed for this platform.
"use strict";

const { spawn } = require("node:child_process");

const pkg = `@gabriellsales/discord-mcp-${process.platform}-${process.arch}`;
const exe = process.platform === "win32" ? "discord-mcp.exe" : "discord-mcp";

let bin;
try {
  bin = require.resolve(`${pkg}/bin/${exe}`);
} catch {
  console.error(
    `discord-mcp: no binary for ${process.platform}-${process.arch}.\n` +
      `Reinstall without --no-optional / --omit=optional, or download it from\n` +
      `https://github.com/gabriellsalesx/discord-mcp/releases`
  );
  process.exit(1);
}

const child = spawn(bin, process.argv.slice(2), { stdio: "inherit", windowsHide: true });
for (const sig of ["SIGINT", "SIGTERM", "SIGHUP"]) {
  process.on(sig, () => child.kill(sig));
}
child.on("error", (err) => {
  console.error(`discord-mcp: ${err.message}`);
  process.exit(1);
});
child.on("exit", (code, signal) => {
  if (signal) process.kill(process.pid, signal);
  else process.exit(code ?? 1);
});
