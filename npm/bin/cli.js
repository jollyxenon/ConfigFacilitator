#!/usr/bin/env node

const { spawnSync } = require("node:child_process");
const path = require("node:path");

// Keep this wrapper tiny: all CLI behavior belongs to the Go binary.
const executableName = process.platform === "win32" ? "cfgfc.exe" : "cfgfc";
const executablePath = path.join(__dirname, executableName);

const result = spawnSync(executablePath, process.argv.slice(2), {
  stdio: "inherit",
  windowsHide: true,
});

if (result.error) {
  console.error(`cfgfc: failed to execute ${executablePath}: ${result.error.message}`);
  process.exit(1);
}

if (result.signal) {
  console.error(`cfgfc: terminated by signal ${result.signal}`);
  process.exit(1);
}

process.exit(result.status ?? 0);
