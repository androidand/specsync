#!/usr/bin/env node
// Build script for the specsync site. Idempotent: it replaces the content
// between <!-- X:start --> and <!-- X:end --> markers, so it can run on the
// committed (already-built) index.html and re-inject cleanly.
// Run: node build.sh   (CF Pages build command: cd site && node build.sh)
// Requires: Node 16+. No git dependency (some build environments have no
// tags). The GitHub Releases fetch is best-effort: on failure the version
// badge and changelog are left exactly as committed, never degraded.

const fs = require("fs");
const https = require("https");

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

  // 1 & 3. Version + changelog both come from one GitHub Releases fetch — no
  // local git-tag dependency (some build environments, e.g. a shallow-clone
  // CI checkout, have no tags at all) and no stale checked-in fallback value.
  // On any failure (network blocked, rate-limited, no releases yet) neither
  // region is touched, so a build never regresses the last known-good,
  // already-committed content — better a stale-but-correct badge than a
  // wrong one or an empty placeholder.
  let releases = null;
  try {
    const res = await get("https://api.github.com/repos/androidand/specsync/releases?per_page=4");
    if (res.status !== 200) throw new Error(`HTTP ${res.status}`);
    releases = JSON.parse(res.body).filter((r) => !r.draft);
  } catch (e) {
    console.warn(`  releases: ${e.message} — version and changelog left as committed`);
  }

  if (releases && releases.length > 0) {
    html = replaceRegion(html, "VERSION", releases[0].tag_name);
    console.log(`  version: ${releases[0].tag_name} (released)`);

    const changelog = releases.slice(0, 3).map((r) => {
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
    html = replaceRegion(html, "CHANGELOG", changelog);
    console.log(`  changelog: ${Math.min(releases.length, 3)} releases`);
  }

  // shippedIssueNumbers: every "#N" reference in the fetched release bodies —
  // cross-checked against each "soon" feature's `issue` field so its badge
  // clears itself the moment that issue actually ships, instead of relying on
  // someone remembering to hand-edit features.json. Reuses the same fetch as
  // the changelog above (no extra request, no extra failure mode): if it's
  // null, "soon" badges are simply left exactly as authored.
  const shipped = new Set();
  if (releases) {
    for (const r of releases) for (const m of (r.body || "").matchAll(/#(\d+)/g)) shipped.add(m[1]);
  }

  // 2. Features from features.json. status:"soon" cards are clearly badged as
  //    planned-not-yet-shipped, so the page stays true to what is installable.
  const features = JSON.parse(fs.readFileSync("features.json", "utf8"));
  const featuresHtml = features.map((f) => {
    const soon = f.status === "soon" && !(f.issue && shipped.has(String(f.issue)));
    const badge = soon ? ` <span class="soon">soon</span>` : "";
    const cls = soon ? "feature is-soon" : "feature";
    return `      <div class="${cls}">
        <span class="feature-icon">${f.icon}</span>
        <h4>${escapeHtml(f.title)}${badge}</h4>
        <p>${f.body}</p>
      </div>`;
  }).join("\n");
  html = replaceRegion(html, "FEATURES", featuresHtml);
  const soonCount = features.filter((f) => f.status === "soon" && !(f.issue && shipped.has(String(f.issue)))).length;
  console.log(`  features: ${features.length} (${soonCount} marked soon)`);

  fs.writeFileSync("index.html", html);
  console.log("  site: built → index.html");
}

build().catch((e) => { console.error(e); process.exit(1); });
