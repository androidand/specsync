# Deploy the standalone Specsync site

Make `specsync.se` the canonical product site and deploy the existing static
site from this repository after changes land on `main`. Keep the Tantonet route
only as a permanent redirect so product documentation and release promotion
live beside the code they describe.

The deployment remains disabled until the Cloudflare Pages project, repository
secrets, and custom domain are configured.

## Non-goals

- Managing Cloudflare DNS from this repository.
- Moving the Specsync site into a separate repository or web framework.
- Deploying pull requests to the production domain.
