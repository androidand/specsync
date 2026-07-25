# CI validation for OpenSpec change structure

CI currently runs `go build`, `go vet`, `go test`, `release-plan`, and `changelog`. It does not validate that OpenSpec change folders are well-formed before sync. This change adds explicit CI gates for change structure.

## Out of scope

- Schema enforcement for `proposal.md` content (beyond file existence)
- Linting of markdown content
