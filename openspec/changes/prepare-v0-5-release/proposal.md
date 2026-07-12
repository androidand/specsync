# Prepare v0.5 release and truthful promotion

After lifecycle and packaging fixes land, prepare a release and update the
promotional surfaces from verified behavior. Current site copy overclaims that
every command supports dry-run and under-describes the supported agent skills.

Treat the repository README, generated site, release notes, GitHub release
artifacts, and npm package as one release contract. Promote only commands and
behavior demonstrated by tests or smoke checks.

## Recovery note

`v0.5.0` was published from `ee7664b` before the dependent fixes were committed.
Do not move the public tag. Ship the complete, verified work as `v0.5.1`.

## Dependencies

- `fix-completion-lifecycle`
- `sync-skill-artifacts`
- `harden-npm-installer`

## Non-goals

- A visual redesign of the promotional site.
- Shipping planned commands as completed features.
- Publishing before the dependent changes pass validation.
