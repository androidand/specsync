const crypto = require("crypto");
const fs = require("fs");
const https = require("https");
const path = require("path");
const { execFileSync } = require("child_process");

const PLATFORM = { darwin: "darwin", linux: "linux" };
const ARCH = { x64: "amd64", arm64: "arm64" };

function fetchBuffer(url, redirects = 0) {
  return new Promise((resolve, reject) => {
    if (redirects > 5) return reject(new Error("too many redirects"));
    https.get(url, (response) => {
      if (response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
        response.resume();
        return fetchBuffer(new URL(response.headers.location, url).toString(), redirects + 1).then(resolve, reject);
      }
      if (response.statusCode !== 200) {
        response.resume();
        return reject(new Error(`HTTP ${response.statusCode} for ${url}`));
      }
      const chunks = [];
      response.on("data", (chunk) => chunks.push(chunk));
      response.on("end", () => resolve(Buffer.concat(chunks)));
      response.on("error", reject);
    }).on("error", reject);
  });
}

function checksumFor(contents, asset) {
  for (const line of contents.split(/\r?\n/)) {
    const match = line.trim().match(/^([a-fA-F0-9]{64})\s+\*?(.+)$/);
    if (match && match[2] === asset) return match[1].toLowerCase();
  }
  throw new Error(`missing checksum for ${asset}`);
}

function extractTar(archive, destination) {
  execFileSync("tar", ["-xzf", archive, "-C", destination, "specsync"], { stdio: "inherit" });
}

async function installBinary(options) {
  const goos = PLATFORM[options.platform];
  const goarch = ARCH[options.arch];
  if (!goos || !goarch) throw new Error(`unsupported platform ${options.platform}/${options.arch}`);

  const asset = `specsync_${goos}_${goarch}.tar.gz`;
  const base = `https://github.com/androidand/specsync/releases/download/v${options.version}`;
  const stagingDir = fs.mkdtempSync(path.join(options.tempDir, "specsync-install-"));
  const archivePath = path.join(stagingDir, asset);
  const get = options.fetchBuffer || fetchBuffer;
  const extract = options.extract || extractTar;

  try {
    const [archive, checksums] = await Promise.all([
      get(`${base}/${asset}`),
      get(`${base}/checksums.txt`),
    ]);
    fs.writeFileSync(archivePath, archive);
    const expected = checksumFor(checksums.toString("utf8"), asset);
    const actual = crypto.createHash("sha256").update(archive).digest("hex");
    if (actual !== expected) throw new Error(`checksum mismatch for ${asset}`);

    extract(archivePath, stagingDir);
    const stagedBinary = path.join(stagingDir, "specsync");
    if (!fs.existsSync(stagedBinary)) throw new Error(`archive did not contain specsync`);
    fs.chmodSync(stagedBinary, 0o755);

    fs.mkdirSync(options.binDir, { recursive: true });
    const binary = path.join(options.binDir, "specsync");
    fs.renameSync(stagedBinary, binary);
    return { asset, binary };
  } finally {
    fs.rmSync(stagingDir, { recursive: true, force: true });
  }
}

module.exports = { checksumFor, fetchBuffer, installBinary };
