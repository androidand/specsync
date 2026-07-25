## ADDED Requirements

### Requirement: The product site has one canonical source

The repository SHALL keep the source and deployable output for the Specsync
product site under `site/`, and the page SHALL identify `https://specsync.se/`
as its canonical URL.

#### Scenario: A product claim changes

- **WHEN** a maintainer updates the Specsync site's product copy
- **THEN** the change is reviewed and released from the Specsync repository
- **AND** Tantonet does not maintain a second copy of that page

### Requirement: Main deploys to Cloudflare Pages

The repository SHALL provide a GitHub Actions workflow that builds the site and
deploys it to the `specsync` Cloudflare Pages project on pushes to `main`.

#### Scenario: Cloudflare setup is incomplete

- **WHEN** the required enable variable is absent or false
- **THEN** the production deployment job is skipped without exposing secrets or failing the build

#### Scenario: Cloudflare setup is enabled

- **WHEN** a commit reaches `main`
- **AND** the Cloudflare project, credentials, and enable variable are configured
- **THEN** GitHub Actions builds `site/index.html`
- **AND** deploys the `site/` directory as the production branch

### Requirement: The legacy route redirects permanently

The Tantonet deployment SHALL redirect `/specsync` and every nested Specsync
path to `https://specsync.se/` with HTTP status 301.

#### Scenario: A visitor follows an old link

- **WHEN** the visitor requests `tantonet.se/specsync` or a nested path
- **THEN** the visitor is permanently redirected to the canonical site
