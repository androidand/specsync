# Tasks

- [ ] Add `specsync validate` subcommand: checks all change folders for required files (proposal.md, tasks.md), valid metadata, and well-formed stage mappings
- [ ] CI step: `specsync validate` as an explicit gate before sync
- [ ] Fail fast: report all structural issues in one pass, exit non-zero on any error
- [ ] Tests: missing proposal.md, missing tasks.md, invalid metadata, valid changes pass
- [ ] `go build ./...`, `go test ./...`, `gofmt` clean
