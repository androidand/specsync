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

function featureIconSvg(name) {
  const base = 'width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"';
  const icons = {
    flow: `<svg ${base}><path d="M4 6h6"/><path d="M4 18h6"/><path d="M10 6l3 6"/><path d="M10 18l3-6"/><path d="M13 12h7"/><path d="M18 9l3 3-3 3"/><circle cx="4" cy="6" r="1.5"/><circle cx="4" cy="18" r="1.5"/></svg>`,
    scan: `<svg ${base}><circle cx="12" cy="12" r="9"/><circle cx="12" cy="12" r="5.5"/><circle cx="12" cy="12" r="1.2"/><path d="M12 12L19.5 8"/></svg>`,
    idempotent: `<svg ${base}><rect x="9" y="9" width="6" height="6" rx="1"/><path d="M6 10a6 6 0 0 1 10-4"/><path d="M16 6h4v4"/><path d="M18 14a6 6 0 0 1-10 4"/><path d="M8 18H4v-4"/></svg>`,
    state: `<svg ${base}><path d="M5 7h5"/><path d="M5 12h9"/><path d="M5 17h13"/><circle cx="4" cy="7" r="1.2"/><circle cx="4" cy="12" r="1.2"/><circle cx="4" cy="17" r="1.2"/><path d="M19 17l1.8 1.8L23 16.6"/></svg>`,
    board: `<svg ${base}><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M9 4v16"/><path d="M15 4v16"/><path d="M5 8h2"/><path d="M11 11h2"/><path d="M17 7h2"/></svg>`,
    followup: `<svg ${base}><path d="M9 3h6"/><path d="M9 1v4"/><path d="M15 1v4"/><rect x="4" y="3" width="16" height="20" rx="2"/><path d="M8 10h8"/><path d="M8 14h8"/><path d="M8 18h5"/><path d="M17 18l2 2 3-3"/></svg>`,
    changelog: `<svg ${base}><path d="M14 2H7a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7z"/><path d="M14 2v5h5"/><path d="M9 12h6"/><path d="M9 16h6"/><path d="M9 8h2"/></svg>`,
  };
  return icons[name] || icons.flow;
}

// inlineMd renders the handful of inline markers a changelog bullet uses:
// **bold** and `code`. Runs after escapeHtml, so raw < > & are already safe.
function inlineMd(s) {
  return escapeHtml(s)
    .replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>")
    .replace(/`([^`]+)`/g, "<code>$1</code>");
}

function parseChangelogEntries(md) {
  const lines = md.split("\n");
  const entries = [];
  let currentVersion = "";
  let currentDate = "";
  let buf = [];
  const flush = () => {
    if (!currentVersion) return;
    entries.push({
      version: currentVersion,
      date: currentDate,
      body: buf.join("\n").trim(),
    });
  };
  for (const line of lines) {
    const m = line.match(/^##\s+\[([^\]]+)\](?:\s*-\s*([^\n]+))?\s*$/);
    if (m) {
      flush();
      currentVersion = m[1].trim().replace(/^v/i, "");
      currentDate = (m[2] || "").trim();
      buf = [];
      continue;
    }
    if (currentVersion) buf.push(line);
  }
  flush();
  return entries;
}

function parseChangelogSections(md) {
  const sections = {};
  for (const e of parseChangelogEntries(md)) sections[e.version] = e.body;
  return sections;
}

// Both a local CHANGELOG.md section and a GitHub release body (from v0.7.0
// on — specsync's own `changelog -release-notes`) share this shape:
// "### Added"-style headings with bullets, each ending in either
// "(#N[, #M...])" — a commit resolved to an OpenSpec change's issue — or a
// bare short hash, for a commit that links to no change (chore/docs/ci
// commits are already rolled into a "N internal commits omitted" comment by
// specsync itself, and merge commits never appear at all — so what's left
// unlinked here is real, shipped feat/fix work that just isn't spec-backed
// yet). Both render: an issue-linked entry gets a prominent "#N" badge — a
// spec actually stands behind it. An unlinked one still shows — the release
// shouldn't read as emptier than it was — but with a quiet commit link
// instead, so the two are never visually confused. A body that doesn't look
// like this shape at all (older goreleaser-raw releases, pre v0.7.0) falls
// back to a plain "view full release" link — never a raw dump.
const ISSUE_SUFFIX = /\s*\(((?:#\d+)(?:,\s*#\d+)*)\)\s*$/;
const HASH_SUFFIX = /\s*\(([0-9a-f]{7,40})\)\s*$/i;

function renderChangelogSection(section, repoUrl) {
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
  for (const line of section.split("\n")) {
    const h = line.match(/^#{1,6}\s+(.+)$/);
    const bullet = line.match(/^[-*]\s+(.+)$/);
    if (h) {
      flush();
      current = { heading: h[1].trim(), items: [] };
      groups.push(current);
    } else if (bullet) {
      flush();
      item = bullet[1].trim();
    } else if (line.trim() === "" || /^<!--.*-->$/.test(line.trim())) {
      continue; // blank lines and the "N internal commits omitted" marker
    } else if (item !== null) {
      item += " " + line.trim();
    }
  }
  flush();

  return groups.map((g) => {
    const items = g.items
      .map((it) => {
        const issue = it.match(ISSUE_SUFFIX);
        if (issue) {
          const text = it.slice(0, issue.index).trim();
          const refs = issue[1].split(",").map((r) => r.trim()).map((ref) =>
            `<a href="${repoUrl}/issues/${ref.slice(1)}" target="_blank" rel="noopener">${ref}</a>`
          ).join(", ");
          return `<li>${inlineMd(text)} <span class="release-ref">${refs}</span></li>`;
        }
        const hash = it.match(HASH_SUFFIX);
        if (hash) {
          const text = it.slice(0, hash.index).trim();
          const sha = hash[1];
          return `<li>${inlineMd(text)} <a class="release-ref-commit" href="${repoUrl}/commit/${sha}" target="_blank" rel="noopener">${sha.slice(0, 7)}</a></li>`;
        }
        return null; // no reference at all — not shown on the landing page
      })
      .filter(Boolean);
    if (items.length === 0) return "";
    const heading = g.heading ? `<h5>${escapeHtml(g.heading)}</h5>` : "";
    return `${heading}<ul>${items.join("\n")}</ul>`;
  }).filter(Boolean).join("\n");
}

async function build() {
  let html = fs.readFileSync("index.html", "utf8");
  let changelogEntries = [];
  let changelogSections = {};
  try {
    const localChangelog = fs.readFileSync("../CHANGELOG.md", "utf8");
    changelogEntries = parseChangelogEntries(localChangelog);
    changelogSections = parseChangelogSections(localChangelog);
  } catch (e) {
    console.warn(`  changelog: ${e.message} — local CHANGELOG.md unavailable, using release body fallback`);
  }

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
      const version = String(r.tag_name || "").replace(/^v/i, "");
      const changelogBody = changelogSections[version]
        ? renderChangelogSection(changelogSections[version], "https://github.com/androidand/specsync")
        : "";
      const releaseBody = r.body ? renderChangelogSection(r.body, "https://github.com/androidand/specsync") : "";
      const body = changelogBody || releaseBody;
      const empty = `<p class="release-empty">No spec-derived entries for this release.</p>`;
      return `        <div class="release">
          <div class="release-header">
            <a class="release-tag" href="${r.html_url}" target="_blank" rel="noopener">${escapeHtml(r.tag_name)}</a>
            <span class="release-date">${date}</span>
          </div>
          <div class="release-body">${body || empty}</div>
          <a class="release-full-link" href="${r.html_url}" target="_blank" rel="noopener">View complete release details on GitHub →</a>
        </div>`;
    }).join("\n");
    html = replaceRegion(html, "CHANGELOG", changelog);
    console.log(`  changelog: ${Math.min(releases.length, 3)} releases`);
  } else if (changelogEntries.length > 0) {
    const repoUrl = "https://github.com/androidand/specsync";
    const released = changelogEntries.filter((e) => e.version.toLowerCase() !== "unreleased").slice(0, 3);
    if (released.length > 0) {
      html = replaceRegion(html, "VERSION", `v${released[0].version}`);
      const changelog = released.map((e) => {
        const body = renderChangelogSection(e.body, repoUrl);
        const empty = `<p class="release-empty">No spec-derived entries for this release.</p>`;
        const date = /^\d{4}-\d{2}-\d{2}$/.test(e.date)
          ? new Date(`${e.date}T00:00:00Z`).toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric" })
          : (e.date || "");
        return `        <div class="release">
          <div class="release-header">
            <a class="release-tag" href="${repoUrl}/releases/tag/v${escapeHtml(e.version)}" target="_blank" rel="noopener">v${escapeHtml(e.version)}</a>
            <span class="release-date">${escapeHtml(date)}</span>
          </div>
          <div class="release-body">${body || empty}</div>
          <a class="release-full-link" href="${repoUrl}/releases/tag/v${escapeHtml(e.version)}" target="_blank" rel="noopener">View complete release details on GitHub →</a>
        </div>`;
      }).join("\n");
      html = replaceRegion(html, "CHANGELOG", changelog);
      console.log(`  changelog: ${released.length} releases (local CHANGELOG fallback)`);
    }
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

  // 2. Features from features.json, grouped into three themes (Plan /
  //    Collaborate / Ship) so fourteen equal cards read as a story instead of
  //    an inventory. status:"soon" cards are clearly badged as
  //    planned-not-yet-shipped, so the page stays true to what is installable.
  const features = JSON.parse(fs.readFileSync("features.json", "utf8"));
  const GROUP_LABELS = { plan: "Plan", collaborate: "Collaborate", ship: "Ship" };
  const featureCard = (f) => {
    const soon = f.status === "soon" && !(f.issue && shipped.has(String(f.issue)));
    const badge = soon ? ` <span class="soon">soon</span>` : "";
    const cls = soon ? "feature is-soon" : "feature";
      const icon = `<span class="feature-icon" aria-hidden="true">${featureIconSvg(f.icon)}</span>`;
    return `        <div class="${cls}">
        <h4>${icon}<span class="feature-title">${escapeHtml(f.title)}</span>${badge}</h4>
          <p>${f.body}</p>
        </div>`;
  };
  const featuresHtml = Object.keys(GROUP_LABELS).map((key) => {
    const items = features.filter((f) => f.group === key);
    if (items.length === 0) return "";
    return `      <div class="feature-theme">
        <h3 class="feature-theme-label">${GROUP_LABELS[key]}</h3>
        <div class="features">
${items.map(featureCard).join("\n")}
        </div>
      </div>`;
  }).filter(Boolean).join("\n");
  html = replaceRegion(html, "FEATURES", featuresHtml);
  const soonCount = features.filter((f) => f.status === "soon" && !(f.issue && shipped.has(String(f.issue)))).length;
  console.log(`  features: ${features.length} (${soonCount} marked soon)`);

  fs.writeFileSync("index.html", html);
  console.log("  site: built → index.html");
}

build().catch((e) => { console.error(e); process.exit(1); });
