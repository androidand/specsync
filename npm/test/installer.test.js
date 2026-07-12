const assert = require("node:assert/strict");
const crypto = require("node:crypto");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");

const { checksumFor, installBinary } = require("../scripts/installer");

test("checksumFor finds the exact release asset", () => {
  const digest = "d".repeat(64);
  assert.equal(checksumFor(`${"a".repeat(64)}  other.tar.gz\n${digest}  specsync_linux_amd64.tar.gz\n`, "specsync_linux_amd64.tar.gz"), digest);
  assert.throws(() => checksumFor("abc  other.tar.gz\n", "missing.tar.gz"), /missing checksum/);
});

test("installBinary rejects unsupported platforms with actionable context", async () => {
  await assert.rejects(installBinary({
    version: "1.0.0", platform: "win32", arch: "x64", binDir: "unused", tempDir: "unused",
  }), /unsupported platform win32\/x64/);
});

test("installBinary rejects HTTP and redirect failures and cleans temporary files", async () => {
  for (const message of ["HTTP 404", "too many redirects"]) {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), "specsync-install-test-"));
    await assert.rejects(installBinary({
      version: "1.0.0", platform: "linux", arch: "x64", binDir: path.join(root, "bin"), tempDir: root,
      fetchBuffer: async () => { throw new Error(message); }, extract: () => {},
    }), new RegExp(message));
    assert.deepEqual(fs.readdirSync(root), []);
    fs.rmSync(root, { recursive: true });
  }
});

test("installBinary rejects checksum mismatch before extraction", async () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "specsync-install-test-"));
  let extracted = false;
  const archive = Buffer.from("archive");
  await assert.rejects(installBinary({
    version: "1.0.0", platform: "linux", arch: "x64", binDir: path.join(root, "bin"), tempDir: root,
    fetchBuffer: async (url) => url.endsWith("checksums.txt") ? Buffer.from(`${"0".repeat(64)}  specsync_linux_amd64.tar.gz\n`) : archive,
    extract: () => { extracted = true; },
  }), /checksum mismatch/);
  assert.equal(extracted, false);
  assert.deepEqual(fs.readdirSync(root), []);
  fs.rmSync(root, { recursive: true });
});

test("installBinary installs a verified executable and cleans its archive", async () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "specsync-install-test-"));
  const archive = Buffer.from("archive");
  const digest = crypto.createHash("sha256").update(archive).digest("hex");
  const binDir = path.join(root, "bin");
  await installBinary({
    version: "1.0.0", platform: "linux", arch: "x64", binDir, tempDir: root,
    fetchBuffer: async (url) => url.endsWith("checksums.txt") ? Buffer.from(`${digest}  specsync_linux_amd64.tar.gz\n`) : archive,
    extract: (_archive, destination) => fs.writeFileSync(path.join(destination, "specsync"), "binary"),
  });
  assert.equal(fs.statSync(path.join(binDir, "specsync")).mode & 0o777, 0o755);
  assert.deepEqual(fs.readdirSync(root).sort(), ["bin"]);
  fs.rmSync(root, { recursive: true });
});

test("installBinary removes temporary files when extraction fails", async () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "specsync-install-test-"));
  const archive = Buffer.from("archive");
  const digest = crypto.createHash("sha256").update(archive).digest("hex");
  await assert.rejects(installBinary({
    version: "1.0.0", platform: "linux", arch: "x64", binDir: path.join(root, "bin"), tempDir: root,
    fetchBuffer: async (url) => url.endsWith("checksums.txt") ? Buffer.from(`${digest}  specsync_linux_amd64.tar.gz\n`) : archive,
    extract: () => { throw new Error("extract failed"); },
  }), /extract failed/);
  assert.deepEqual(fs.readdirSync(root), []);
  fs.rmSync(root, { recursive: true });
});

test("failed upgrade preserves an existing installed binary", async () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "specsync-install-test-"));
  const binDir = path.join(root, "bin");
  fs.mkdirSync(binDir);
  fs.writeFileSync(path.join(binDir, "specsync"), "working-old-binary");

  await assert.rejects(installBinary({
    version: "1.0.0", platform: "linux", arch: "x64", binDir, tempDir: root,
    fetchBuffer: async () => { throw new Error("HTTP 503"); }, extract: () => {},
  }), /HTTP 503/);

  assert.equal(fs.readFileSync(path.join(binDir, "specsync"), "utf8"), "working-old-binary");
  fs.rmSync(root, { recursive: true });
});

test("concurrent installs use distinct staging paths", async () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "specsync-install-test-"));
  const archive = Buffer.from("archive");
  const digest = crypto.createHash("sha256").update(archive).digest("hex");
  const archives = [];
  const options = (name) => ({
    version: "1.0.0", platform: "linux", arch: "x64", binDir: path.join(root, name), tempDir: root,
    fetchBuffer: async (url) => url.endsWith("checksums.txt") ? Buffer.from(`${digest}  specsync_linux_amd64.tar.gz\n`) : archive,
    extract: (archivePath, destination) => {
      archives.push(archivePath);
      fs.writeFileSync(path.join(destination, "specsync"), name);
    },
  });

  await Promise.all([installBinary(options("one")), installBinary(options("two"))]);
  assert.equal(new Set(archives).size, 2);
  assert.equal(fs.readdirSync(root).filter((entry) => entry.startsWith("specsync-install-")).length, 0);
  fs.rmSync(root, { recursive: true });
});
