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
