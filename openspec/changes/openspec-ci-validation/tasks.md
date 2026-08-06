# Tasks

- [x] Add `specsync validate` subcommand: checks all change folders for required files (proposal.md, tasks.md), valid metadata, and well-formed stage mappings
- [x] CI step: `specsync validate` as an explicit gate before sync
- [x] Fail fast: report all structural issues in one pass, exit non-zero on any error
- [x] Tests: missing proposal.md, missing tasks.md, invalid metadata, valid changes pass
- [x] `go build ./...`, `go test ./...`, `gofmt` clean
