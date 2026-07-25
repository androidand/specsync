# Tasks

- [ ] Detect `bd` on PATH: `exec.LookPath("bd")` succeeds → auto-select Beads provider
- [ ] Detect `.beads/` in repo root: `os.Stat(".beads/")` succeeds → auto-select Beads provider
- [ ] Explicit `-provider` flag always overrides auto-detection
- [ ] Dry-run prints which provider was auto-selected and why
- [ ] Tests: PATH detection, .beads/ detection, flag override
- [ ] `go build ./...`, `go test ./...`, `gofmt` clean
