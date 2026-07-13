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

## Cloudflare Pages setup

The `Deploy site` GitHub Actions workflow deploys every push to `main` after
the one-time setup below:

1. Create a Cloudflare Pages Direct Upload project named `specsync` with
   production branch `main`.
2. Add repository Actions secrets `CLOUDFLARE_API_TOKEN` and
   `CLOUDFLARE_ACCOUNT_ID`. The token needs permission to edit Cloudflare
   Pages for the account.
3. Attach `specsync.se` as the project's custom domain in Cloudflare Pages.
4. Add the repository Actions variable `CLOUDFLARE_PAGES_ENABLED=true` last.

The enable variable keeps deployments safely disabled until the project,
credentials, and domain are ready. A manual run is available from the Actions
tab after setup.

`tantonet.se/specsync` is maintained by the Tantonet site as a permanent
redirect to the canonical domain. Redirect `www.specsync.se` to the apex in
Cloudflare if the `www` hostname is enabled.
