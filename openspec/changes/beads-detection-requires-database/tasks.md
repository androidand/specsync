# Tasks: Beads auto-detection requires a Beads database

## Detection (`cmd/specsync/main.go`)
- [x] Drop `bd`-on-PATH as a sufficient signal
- [x] Require a `.beads/` **directory** (repo root from `-openspec`, then working dir)
- [x] Keep `bd` on PATH necessary — `.beads/` without the binary falls back to github
- [x] Reject a `.beads` regular file
- [x] Explicit `-provider` still short-circuits detection

## Reporting
- [x] Print provider + reason on real runs when auto-detection leaves the github default
- [x] Stay silent when detection resolves to github

## Tests
- [x] No `.beads/` → github, whether or not `bd` is installed (the regression)
- [x] `.beads/` at repo root → beads, with the reason naming the directory
- [x] `.beads/` in working dir → beads
- [x] `.beads/` without `bd` on PATH → github
- [x] `.beads` regular file → github
- [x] Explicit `-provider github` / `-provider beads` win, with no reason string
- [x] Full suite green
