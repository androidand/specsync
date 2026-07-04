# Tasks: Launch readiness — confident public promotion

## 1. Safe, mechanical fixes (agent can do unsupervised)

- [ ] 1.1 Add npm metadata to `npm/package.json`: `"keywords"` (e.g. `openspec`,
      `github-issues`, `spec-driven`, `issue-tracker`, `sync`, `cli`,
      `planning`), `"author": "androidand"`, and
      `"bugs": "https://github.com/androidand/specsync/issues"`. JSON only;
      do not bump `version`.
- [ ] 1.2 README: document every shipped subcommand in the Usage section —
      `scan`, `trace`, `link`, `release-plan`, `install-skill` — with one-line
      descriptions and one example each, matching the flags in
      `cmd/specsync/main.go` and `.github/copilot-instructions.md` (note:
      `scan` flags come before positional args). Also document
      `-provider beads`.
- [ ] 1.3 README: add a short "Requirements" subsection under Install: no Go
      toolchain needed for npm/binary installs; the GitHub provider shells out
      to an authenticated `gh` CLI; Node >= 16 for the npm wrapper;
      linux/darwin on amd64/arm64 only (no Windows binary today).
- [ ] 1.4 README: add badges at the top — npm version
      (`@androidand/specsync`), CI status (`.github/workflows/ci.yml`), and
      license — using plain shields.io URLs.
- [ ] 1.5 Add version reporting to the binary: a `version` subcommand and
      `-version` flag in `cmd/specsync/main.go` printing a package-level
      `var version = "dev"`, and inject the real value in `.goreleaser.yaml`
      ldflags (`-X main.version={{ .Version }}`). Add a test that the
      subcommand path is wired (see `cmd` switch at `cmd/specsync/main.go:22`).
- [ ] 1.6 Run `go vet ./...` and `go test ./...`; both must pass before
      committing.

## 2. Fixes needing a running app or human judgment

- [ ] 2.1 Verify the npm install path end-to-end in a scratch directory:
      `npm i -g @androidand/specsync` on this machine, confirm the postinstall
      downloads the v0.4.0 binary and `specsync -dry-run` runs in a repo with
      an `openspec/` dir. Fix README wording if any step surprises.
- [ ] 2.2 Read the rewritten README start-to-finish as a stranger from npm:
      can you install and sync your first change using only the README? Adjust
      ordering/wording where you stumble.
- [ ] 2.3 Rebuild the site (`cd site && node build.sh`) and commit the result
      only if the diff is a truthful regeneration (version/changelog regions
      between the markers); otherwise leave it.

## 3. Blockers needing the owner

- [ ] 3.1 Confirm the GitHub repo `androidand/specsync` is public and set a
      repo description and topics (e.g. `openspec`, `github-issues`, `cli`,
      `go`) via the GitHub UI or `gh repo edit` (requires authenticated `gh`;
      not available in the audit environment).
- [ ] 3.2 Decide on the unscoped npm name: `specsync` is currently unclaimed
      on the registry (404). Either publish the wrapper there too / as an
      alias, or standardize all promotion copy on `@androidand/specsync`.
      Requires npm publish rights.
- [ ] 3.3 Accept (or not) the commit-author email situation: history contains
      the owner's work and personal emails as ordinary git author metadata.
      Recommendation: accept as-is — no history rewrite; optionally set a
      noreply email in this repo's git config going forward.
- [ ] 3.4 Optional promotion follow-ups: tag/announce a v0.4.1 with the new
      `version` flag, and verify the Cloudflare Pages site deploy still works
      after the README/site changes.
