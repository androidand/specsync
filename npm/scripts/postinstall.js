#!/usr/bin/env node
// Downloads the platform-matched specsync binary from GitHub Releases.
// Asset names match .goreleaser.yaml: specsync_<os>_<arch>.tar.gz
const fs = require("fs");
const os = require("os");
const path = require("path");
const { version } = require("../package.json");
const { installBinary } = require("./installer");

const GOOS = { darwin: "darwin", linux: "linux" }[process.platform];
const GOARCH = { x64: "amd64", arm64: "arm64" }[process.arch];

if (!GOOS || !GOARCH) {
  console.error(`specsync: no prebuilt binary for ${process.platform}/${process.arch}. Supported: macOS and Linux on x64/arm64. Use 'go install github.com/androidand/specsync/cmd/specsync@latest' instead.`);
  process.exitCode = 1;
  return;
}

const binDir = path.join(__dirname, "..", "bin");

// Install the specsync skill into every known global agent skill directory.
// Non-fatal: a missing ~/.claude or ~/.codex directory is normal.
function installSkill() {
  const skillSrc = path.join(__dirname, "..", "skills", "specsync", "SKILL.md");
  if (!fs.existsSync(skillSrc)) return; // shouldn't happen in a published package

  const agentDirs = [
    path.join(os.homedir(), ".claude", "skills", "specsync"),
    path.join(os.homedir(), ".codex", "skills", "specsync"),
    path.join(os.homedir(), ".config", "opencode", "skills", "specsync"),
    path.join(os.homedir(), ".copilot", "skills", "specsync"),
    path.join(os.homedir(), ".agents", "skills", "specsync"),
  ];

  const skill = fs.readFileSync(skillSrc, "utf8");
  for (const dir of agentDirs) {
    try {
      fs.mkdirSync(dir, { recursive: true });
      fs.writeFileSync(path.join(dir, "SKILL.md"), skill);
      console.log(`specsync: skill installed → ${dir}`);
    } catch (e) {
      // Silently skip — permission issues or read-only filesystems are non-fatal.
    }
  }
}

installSkill();
installBinary({ version, platform: process.platform, arch: process.arch, binDir, tempDir: os.tmpdir() })
  .then(() => console.log(`specsync ${version} installed (${GOOS}/${GOARCH})`))
  .catch((error) => {
    console.error(`specsync: install failed (${error.message}).`);
    console.error("See https://github.com/androidand/specsync/releases or run 'go install github.com/androidand/specsync/cmd/specsync@latest'.");
    process.exitCode = 1;
  });
