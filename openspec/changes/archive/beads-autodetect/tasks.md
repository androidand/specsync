# Tasks

- [x] Detect `bd` on PATH: `exec.LookPath("bd")` succeeds → auto-select Beads provider
- [x] Detect `.beads/` in repo root: `os.Stat(".beads/")` succeeds → auto-select Beads provider
- [x] Explicit `-provider` flag always overrides auto-detection
- [x] Dry-run prints which provider was auto-selected and why
- [x] Tests: PATH detection, .beads/ detection, flag override
- [x] `go build ./...`, `go test ./...`, `gofmt` clean
