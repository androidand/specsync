# Tasks

## 1. Library

- [x] 1.1 `BuildChangelog`: shipped changes from trace input, entry per change,
      category from deltas + commit types, breaking flag
- [x] 1.2 `ReleaseNote(change)`: extract `## Release note` section from
      proposal.md, fall back to title
- [x] 1.3 Loose-commit fallback entries + omitted-plumbing count
- [x] 1.4 `RenderChangelogSection`: Keep a Changelog markdown, deterministic order
- [x] 1.5 `ApplyChangelog`: create/prepend/replace version section idempotently

## 2. CLI

- [x] 2.1 `specsync changelog` subcommand: -since/-until/-version/-json/
      -release-notes/-apply/-force, version default from advisory bump
- [x] 2.2 Defer -apply to a changelog-owning release tool unless -force

## 3. Verification

- [x] 3.1 Unit tests: categories, release-note extraction, loose commits,
      idempotent apply, tool deference
- [x] 3.2 Dogfood: generate this repo's next release section from real history
