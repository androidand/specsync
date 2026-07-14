#!/usr/bin/env node
// Build script for the specsync site. Idempotent: it replaces the content
// between <!-- X:start --> and <!-- X:end --> markers, so it can run on the
// committed (already-built) index.html and re-inject cleanly.
// Run: node build.sh   (CF Pages build command: cd site && node build.sh)
// Requires: Node 16+. Network + git are best-effort; missing ones degrade.

const fs = require("fs");
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

// inlineMd renders the handful of inline markers a changelog entry uses:
// **bold** and `code`. Runs after escapeHtml, so raw < > & are already safe.
function inlineMd(s) {
  return escapeHtml(s)
    .replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>")
    .replace(/`([^`]+)`/g, "<code>$1</code>");
}

// renderChangelogBody renders one version section's body: "### Category"
// headings each followed by a "- " bullet list. Hand-edited bullets are
// routinely soft-wrapped across lines (see CHANGELOG.md itself) — a
// continuation line is anything that doesn't start a new heading or bullet,
// and is folded back onto the item it continues before rendering.
function renderChangelogBody(body) {
  const groups = [];
  let current = null;
  let item = null;
  const flush = () => {
    if (item !== null) {
      if (!current) { current = { heading: null, items: [] }; groups.push(current); }
      current.items.push(item);
      item = null;
    }
  };
  for (const line of body.split("\n")) {
    const h = line.match(/^#{1,6}\s+(.+)$/);
    const bullet = line.match(/^[-*]\s+(.+)$/);
    if (h) {
      flush();
      current = { heading: h[1].trim(), items: [] };
      groups.push(current);
    } else if (bullet) {
      flush();
      item = bullet[1].trim();
    } else if (line.trim() === "") {
      continue; // blank lines never separate a Keep a Changelog bullet from its continuation
    } else if (item !== null) {
      item += " " + line.trim();
    }
  }
  flush();

  return groups.map((g) => {
    const heading = g.heading ? `<h5>${escapeHtml(g.heading)}</h5>` : "";
    const lis = g.items.map((it) => `<li>${inlineMd(it)}</li>`).join("\n");
    return `${heading}<ul>${lis}</ul>`;
  }).join("\n");
}

// parseChangelogSections splits a Keep a Changelog file into its "## [x.y.z]
// - date" sections, in file order (newest first, per specsync's convention).
function parseChangelogSections(md) {
  const heading = /^## \[([^\]]+)\](?:\s*-\s*(.+))?\s*$/gm;
  const matches = [...md.matchAll(heading)];
  return matches.map((m, i) => {
    const start = m.index + m[0].length;
    const end = i + 1 < matches.length ? matches[i + 1].index : md.length;
    return { version: m[1], date: m[2] || "", body: md.slice(start, end).trim() };
  });
}

// shippedIssueNumbers is every "#N" issue reference appearing anywhere in
// CHANGELOG.md (any past release, not just the ones the page displays).
// features.json's "soon" cards are cross-checked against this set so a badge
// clears itself the moment its feature actually ships — nobody has to
// remember to hand-edit features.json in sync with the changelog.
function shippedIssueNumbers(md) {
  const nums = new Set();
  for (const m of md.matchAll(/#(\d+)/g)) nums.add(m[1]);
  return nums;
}

async function build() {
  let html = fs.readFileSync("index.html", "utf8");

  // Read CHANGELOG.md once — the features section's "soon" check and the
  // changelog section's rendering both key off the same content.
  let changelogMd = "";
  try {
    changelogMd = fs.readFileSync("../CHANGELOG.md", "utf8");
  } catch (_) {}

  // 1. Version (released tag).
  const version = releasedVersion();
  if (version) {
    html = replaceRegion(html, "VERSION", `v${version}`);
    console.log(`  version: v${version} (released)`);
  }

  // 2. Features from features.json. status:"soon" cards are clearly badged as
  //    planned-not-yet-shipped, so the page stays true to what is installable.
  //    A "soon" card tagged with the issue it ships with auto-clears once that
  //    issue appears in CHANGELOG.md — see shippedIssueNumbers above.
  const features = JSON.parse(fs.readFileSync("features.json", "utf8"));
  const shipped = shippedIssueNumbers(changelogMd);
  let autoCleared = 0;
  const featuresHtml = features.map((f) => {
    let soon = f.status === "soon";
    if (soon && f.issue && shipped.has(String(f.issue))) {
      soon = false;
      autoCleared++;
    }
    const badge = soon ? ` <span class="soon">soon</span>` : "";
    const cls = soon ? "feature is-soon" : "feature";
    return `      <div class="${cls}">
        <span class="feature-icon">${f.icon}</span>
        <h4>${escapeHtml(f.title)}${badge}</h4>
        <p>${f.body}</p>
      </div>`;
  }).join("\n");
  html = replaceRegion(html, "FEATURES", featuresHtml);
  const soonCount = features.filter((f) => f.status === "soon").length - autoCleared;
  console.log(`  features: ${features.length} (${soonCount} marked soon${autoCleared ? `, ${autoCleared} auto-cleared as shipped` : ""})`);

  // 3. Changelog from this repo's own CHANGELOG.md — specsync's feature-level
  //    output, never a raw commit dump. No network call: the file is already
  //    in the checkout.
  let changelog;
  try {
    const md = changelogMd;
    if (!md) throw new Error("CHANGELOG.md not found");
    const sections = parseChangelogSections(md)
      .filter((s) => s.version.toLowerCase() !== "unreleased")
      .slice(0, 3);
    if (sections.length === 0) throw new Error("no released sections found");
    changelog = sections.map((s) => {
      const date = s.date
        ? new Date(s.date).toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric" })
        : "";
      return `        <div class="release">
          <div class="release-header">
            <a class="release-tag" href="https://github.com/androidand/specsync/releases/tag/v${s.version}" target="_blank" rel="noopener">v${escapeHtml(s.version)}</a>
            ${date ? `<span class="release-date">${date}</span>` : ""}
          </div>
          <div class="release-body">${renderChangelogBody(s.body)}</div>
        </div>`;
    }).join("\n");
    console.log(`  changelog: ${sections.length} releases (from CHANGELOG.md)`);
  } catch (e) {
    changelog = `        <p class="changelog-empty">See <a href="https://github.com/androidand/specsync/releases">GitHub releases</a> for the full changelog.</p>`;
    console.warn(`  changelog: ${e.message} — using fallback`);
  }
  html = replaceRegion(html, "CHANGELOG", changelog);

  fs.writeFileSync("index.html", html);
  console.log("  site: built → index.html");
}

build().catch((e) => { console.error(e); process.exit(1); });
