#!/usr/bin/env node
// Thin launcher: exec the platform binary that postinstall downloaded next to this file.
const { spawnSync } = require("child_process");
const { existsSync } = require("fs");
const path = require("path");

const bin = path.join(__dirname, process.platform === "win32" ? "specsync.exe" : "specsync");
if (!existsSync(bin)) {
  console.error(
    "specsync: binary not found. The postinstall step downloads it from GitHub Releases.\n" +
      "Reinstall, or grab a binary from https://github.com/androidand/specsync/releases"
  );
  process.exit(1);
}
const r = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
process.exit(r.status === null ? 1 : r.status);
