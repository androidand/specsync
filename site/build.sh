#!/usr/bin/env node
// Build script for the specsync site. Idempotent: it replaces the content
// between <!-- X:start --> and <!-- X:end --> markers, so it can run on the
// committed (already-built) index.html and re-inject cleanly.
// Run: node build.sh   (CF Pages build command: cd site && node build.sh)
// Requires: Node 16+. Network + git are best-effort; missing ones degrade.

const fs = require("fs");
const https = require("https");
const { execSync } = require("child_process");

// Version reflects what is RELEASED: the latest git tag (truthful to npm),
// falling back to package.json only when no tag is reachable.
function releasedVersion() {
  try {
    const tag = execSync("git describe --tags --abbrev=0", { stdio: ["ignore", "pipe", "ignore"] })
      .toString().trim();
    if (tag) return tag.replace(/^v/, "");
  } catch (_) {}
  try {
    return require("../npm/package.json").version;
  } catch (_) {}
  return "";
}

function get(url) {
  return new Promise((resolve, reject) => {
    https.get(url, { headers: { "User-Agent": "specsync-site-build" } }, (res) => {
      let data = "";
      res.on("data", (c) => (data += c));
      res.on("end", () => resolve({ status: res.statusCode, body: data }));
    }).on("error", reject);
  });
}

function escapeHtml(s) {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

// replaceRegion swaps whatever is between the start/end markers for name.
function replaceRegion(html, name, inner) {
  const start = `<!-- ${name}:start -->`;
  const end = `<!-- ${name}:end -->`;
  const re = new RegExp(escapeRe(start) + "[\\s\\S]*?" + escapeRe(end));
  if (!re.test(html)) {
    console.warn(`  ${name}: markers not found, skipping`);
    return html;
  }
  return html.replace(re, start + "\n" + inner + "\n" + end);
}
function escapeRe(s) { return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"); }

function mdToHtml(md) {
  return escapeHtml(md)
    .replace(/^### (.+)$/gm, "<h5>$1</h5>")
    .replace(/^## (.+)$/gm, "<h4>$1</h4>")
    .replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>")
    .replace(/`([^`]+)`/g, "<code>$1</code>")
    .replace(/^[-*] (.+)$/gm, "<li>$1</li>")
    .replace(/(<li>.*<\/li>\n?)+/g, (s) => `<ul>${s}</ul>`)
    .replace(/\n{2,}/g, "</p><p>")
    .trim();
}

// goreleaser (changelog.use: github) writes release bodies as
//   * <40-char sha>: <commit subject> (@author)
// Render those as grouped, human-readable entries: conventional-commit types
// bucket into Features / Fixes / Other, the sha shortens into a commit link.
// Bodies that don't match (hand-written notes) fall back to mdToHtml.
const COMMIT_LINE = /^\* ([0-9a-f]{40}): (.+?) \(@[^)]+\)\s*$/;
const CONVENTIONAL = /^(\w+)(?:\([^)]*\))?!?:\s*(.+)$/;
const TYPE_GROUP = { feat: "Features", fix: "Fixes" };

function renderReleaseBody(body, repoUrl) {
  const commits = [];
  let matched = true;
  for (const line of body.split("\n")) {
    const t = line.trim();
    if (!t || /^#+\s*Changelog$/i.test(t)) continue;
    const m = t.match(COMMIT_LINE);
    if (!m) { matched = false; break; }
    const conv = m[2].match(CONVENTIONAL);
    commits.push({
      sha: m[1],
      group: conv ? (TYPE_GROUP[conv[1].toLowerCase()] || "Other") : "Other",
      text: conv ? conv[2] : m[2],
    });
  }
  if (!matched || commits.length === 0) return `<p>${mdToHtml(body)}</p>`;

  const groups = ["Features", "Fixes", "Other"];
  return groups.map((g) => {
    const items = commits.filter((c) => c.group === g);
    if (items.length === 0) return "";
    const lis = items.map((c) =>
      `<li>${escapeHtml(c.text)} <a class="commit-sha" href="${repoUrl}/commit/${c.sha}" target="_blank" rel="noopener">${c.sha.slice(0, 7)}</a></li>`
    ).join("\n");
    return `<h5>${g}</h5><ul>${lis}</ul>`;
  }).filter(Boolean).join("\n");
}

async function build() {
  let html = fs.readFileSync("index.html", "utf8");

  // 1. Version (released tag).
  const version = releasedVersion();
  if (version) {
    html = replaceRegion(html, "VERSION", `v${version}`);
    console.log(`  version: v${version} (released)`);
  }

  // 2. Features from features.json. status:"soon" cards are clearly badged as
  //    planned-not-yet-shipped, so the page stays true to what is installable.
  const features = JSON.parse(fs.readFileSync("features.json", "utf8"));
  const featuresHtml = features.map((f) => {
    const soon = f.status === "soon";
    const badge = soon ? ` <span class="soon">soon</span>` : "";
    const cls = soon ? "feature is-soon" : "feature";
    return `      <div class="${cls}">
        <span class="feature-icon">${f.icon}</span>
        <h4>${escapeHtml(f.title)}${badge}</h4>
        <p>${f.body}</p>
      </div>`;
  }).join("\n");
  html = replaceRegion(html, "FEATURES", featuresHtml);
  const soonCount = features.filter((f) => f.status === "soon").length;
  console.log(`  features: ${features.length} (${soonCount} marked soon)`);

  // 3. Changelog from GitHub releases (best-effort).
  let changelog;
  try {
    const res = await get("https://api.github.com/repos/androidand/specsync/releases?per_page=4");
    if (res.status === 200) {
      const releases = JSON.parse(res.body).filter((r) => !r.draft).slice(0, 3);
      changelog = releases.map((r) => {
        const date = new Date(r.published_at).toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric" });
        const body = r.body ? renderReleaseBody(r.body, "https://github.com/androidand/specsync") : "";
        return `        <div class="release">
          <div class="release-header">
            <a class="release-tag" href="${r.html_url}" target="_blank" rel="noopener">${escapeHtml(r.tag_name)}</a>
            <span class="release-date">${date}</span>
          </div>
          ${body ? `<div class="release-body">${body}</div>` : ""}
        </div>`;
      }).join("\n");
      console.log(`  changelog: ${releases.length} releases`);
    } else {
      throw new Error(`HTTP ${res.status}`);
    }
  } catch (e) {
    changelog = `        <p class="changelog-empty">See <a href="https://github.com/androidand/specsync/releases">GitHub releases</a> for the full changelog.</p>`;
    console.warn(`  changelog: ${e.message} — using fallback`);
  }
  html = replaceRegion(html, "CHANGELOG", changelog);

  fs.writeFileSync("index.html", html);
  console.log("  site: built → index.html");
}

build().catch((e) => { console.error(e); process.exit(1); });
