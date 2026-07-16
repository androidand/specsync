# Specsync site

This directory is the source and deployable output for the standalone
Specsync site at <https://specsync.se>. `index.html` is intentionally a static
page; `build.sh` refreshes its released version, feature cards, and changelog.

## Local build

```sh
cd site
node build.sh
```

The build is idempotent and updates `index.html` in place.

## Feature cards

Each `features.json` entry carries a `"group"`: `"plan"`, `"collaborate"`, or
`"ship"` — `build.sh` renders these as three themed groups instead of one flat
grid of fourteen-plus equal cards, so the section tells a story rather than
reading as an inventory.

## Marking a feature card "soon"

A `features.json` entry for work that hasn't shipped yet should carry both
`"status": "soon"` and `"issue": <number>` — the issue number the feature
ships under. `build.sh` cross-checks that number against every `#N` reference
in the fetched GitHub Release bodies (the same fetch used for the changelog
section) and clears the "soon" badge automatically the moment the feature
actually ships, so nobody has to remember to hand-edit `features.json` in
sync with the changelog. Without an `issue` field, a `"soon"` badge never
clears itself and needs a manual edit once shipped.

## Cloudflare Pages setup

Deployment is Cloudflare Pages' own **git integration** ("Connect to Git"),
configured once in the Cloudflare dashboard — there is no GitHub Actions
workflow involved. On every merge to `main`, Cloudflare clones the repo itself
and runs the build below:

1. Create a Cloudflare Pages project named `specsync`, connected to this repo,
   production branch `main`.
2. Build command: `cd site && node build.sh`. Build output directory: `site`.
3. Attach `specsync.se` as the project's custom domain.

## Changelog rendering

The hero promises release notes "never a commit dump," but a release
shouldn't look emptier than it was either — so `build.sh` renders two kinds
of bullet differently rather than picking one extreme:

- a bullet ending in a resolved `(#N[, #M...])` issue reference (the shape
  `specsync changelog -release-notes` produces for a commit linked to an
  OpenSpec change) gets a prominent accent "#N" badge — a real spec backs it.
- a bullet ending in only a bare commit hash (shipped work with no linked
  change) still shows, but with a quiet, muted commit link instead — visibly
  secondary, never confused with a spec-backed entry.
- chore/docs/ci commits are already rolled into a "N internal commits
  omitted" comment by specsync itself, and merge commits never appear in the
  generated body at all, so neither ever reaches the landing page.

A bullet with no reference at all, or a release body that doesn't match this
shape (older goreleaser-raw releases, pre v0.7.0), falls back to a plain "no
spec-derived entries" line — never a raw dump. Every release still gets a
"View complete release details on GitHub" link for the full history.

Cloudflare's own checkout is shallow and has no tags, and its build fleet's
shared IPs can hit GitHub's unauthenticated API rate limit — `build.sh` is
written to tolerate both: version and changelog come from one GitHub Releases
API call with no git-tag dependency, and on any fetch failure the last
committed content is left untouched rather than degraded. Run `node build.sh`
locally before merging a release-relevant change so the committed baseline is
always current, since that's what a failed remote build falls back to.

`tantonet.se/specsync` is maintained by the Tantonet site as a permanent
redirect to the canonical domain. Redirect `www.specsync.se` to the apex in
Cloudflare if the `www` hostname is enabled.
