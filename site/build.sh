#!/usr/bin/env node
// Build script for specsync site.
// Run: node build.sh   (CF Pages build command: cd site && node build.sh)
// Requires: Node 16+, internet access (for GitHub releases).

const fs = require('fs');
const https = require('https');

const VERSION = require('../npm/package.json').version;

function get(url) {
  return new Promise((resolve, reject) => {
    https.get(url, { headers: { 'User-Agent': 'specsync-site-build' } }, res => {
      let data = '';
      res.on('data', c => data += c);
      res.on('end', () => resolve({ status: res.statusCode, body: data }));
    }).on('error', reject);
  });
}

function escapeHtml(s) {
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}

// Convert basic markdown to HTML (just enough for release notes).
function mdToHtml(md) {
  return md
    .replace(/^### (.+)$/gm, '<h5>$1</h5>')
    .replace(/^## (.+)$/gm, '<h4>$1</h4>')
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/^- (.+)$/gm, '<li>$1</li>')
    .replace(/(<li>.*<\/li>\n?)+/g, s => `<ul>${s}</ul>`)
    .replace(/\n{2,}/g, '</p><p>')
    .replace(/^(?!<[hul])/gm, '')
    .trim();
}

async function build() {
  let html = fs.readFileSync('index.html', 'utf8');

  // 1. Stamp version
  html = html.replace(/v0\.2\.1/g, `v${VERSION}`);
  console.log(`  version: ${VERSION}`);

  // 2. Inject features from features.json
  const features = JSON.parse(fs.readFileSync('features.json', 'utf8'));
  const featuresHtml = features.map(f => `
    <div class="feature">
      <span class="feature-icon">${f.icon}</span>
      <h4>${escapeHtml(f.title)}</h4>
      <p>${f.body}</p>
    </div>`).join('\n');
  html = html.replace('<!-- FEATURES -->', featuresHtml);
  console.log(`  features: ${features.length} items`);

  // 3. Fetch latest GitHub releases for changelog
  try {
    const res = await get('https://api.github.com/repos/androidand/specsync/releases?per_page=4');
    if (res.status === 200) {
      const releases = JSON.parse(res.body).filter(r => !r.draft);
      const changelogHtml = releases.slice(0, 3).map(r => {
        const date = new Date(r.published_at).toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' });
        const body = r.body ? mdToHtml(r.body) : '';
        return `
        <div class="release">
          <div class="release-header">
            <a class="release-tag" href="${r.html_url}" target="_blank" rel="noopener">${escapeHtml(r.tag_name)}</a>
            <span class="release-date">${date}</span>
          </div>
          ${body ? `<div class="release-body"><p>${body}</p></div>` : ''}
        </div>`;
      }).join('\n');
      html = html.replace('<!-- CHANGELOG -->', changelogHtml);
      console.log(`  changelog: ${Math.min(releases.length, 3)} releases`);
    } else {
      html = html.replace('<!-- CHANGELOG -->', '<p class="changelog-empty">See <a href="https://github.com/androidand/specsync/releases">GitHub releases</a> for the full changelog.</p>');
      console.warn(`  changelog: GitHub API returned ${res.status}, using fallback`);
    }
  } catch (e) {
    html = html.replace('<!-- CHANGELOG -->', '<p class="changelog-empty">See <a href="https://github.com/androidand/specsync/releases">GitHub releases</a> for the full changelog.</p>');
    console.warn(`  changelog: fetch failed (${e.message}), using fallback`);
  }

  fs.writeFileSync('index.html', html);
  console.log('  site: built → index.html');
}

build().catch(e => { console.error(e); process.exit(1); });
