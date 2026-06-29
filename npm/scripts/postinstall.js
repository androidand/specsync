#!/usr/bin/env node
// Downloads the platform-matched specsync binary from GitHub Releases.
// Asset names match .goreleaser.yaml: specsync_<os>_<arch>.tar.gz
const fs = require("fs");
const os = require("os");
const path = require("path");
const https = require("https");
const { execFileSync } = require("child_process");
const { version } = require("../package.json");

const GOOS = { darwin: "darwin", linux: "linux" }[process.platform];
const GOARCH = { x64: "amd64", arm64: "arm64" }[process.arch];

// Never hard-fail an install on an unsupported platform.
if (!GOOS || !GOARCH) {
  console.error(`specsync: no prebuilt binary for ${process.platform}/${process.arch}; use 'go install' instead.`);
  process.exit(0);
}

const asset = `specsync_${GOOS}_${GOARCH}.tar.gz`;
const url = `https://github.com/androidand/specsync/releases/download/v${version}/${asset}`;
const binDir = path.join(__dirname, "..", "bin");
const tarball = path.join(os.tmpdir(), asset);

function download(u, dest, cb, redirects = 0) {
  if (redirects > 5) return cb(new Error("too many redirects"));
  https
    .get(u, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        res.resume();
        return download(res.headers.location, dest, cb, redirects + 1);
      }
      if (res.statusCode !== 200) {
        res.resume();
        return cb(new Error(`HTTP ${res.statusCode} for ${u}`));
      }
      const f = fs.createWriteStream(dest);
      res.pipe(f);
      f.on("finish", () => f.close(cb));
    })
    .on("error", cb);
}

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

fs.mkdirSync(binDir, { recursive: true });
download(url, tarball, (err) => {
  if (err) {
    console.error(`specsync: download failed (${err.message}).`);
    console.error("Grab a binary from https://github.com/androidand/specsync/releases or run 'go install github.com/androidand/specsync/cmd/specsync@latest'.");
    process.exit(0); // non-fatal: don't break the consumer's npm install
  }
  try {
    execFileSync("tar", ["-xzf", tarball, "-C", binDir, "specsync"], { stdio: "inherit" });
    fs.chmodSync(path.join(binDir, "specsync"), 0o755);
    fs.unlinkSync(tarball);
    console.log(`specsync ${version} installed (${GOOS}/${GOARCH})`);
  } catch (e) {
    console.error(`specsync: extract failed (${e.message}).`);
    process.exit(0);
  }
});
