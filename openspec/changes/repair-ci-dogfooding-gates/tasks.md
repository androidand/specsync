# Tasks

## 1. Enforce the gate

- [x] 1.1 Check out full history and tags in CI
- [x] 1.2 Run `changelog -resolve-refs -fail-on-unlinked-commits`

## 2. Keep it enforced

- [x] 2.1 A test fails if the flag, the full checkout, or consistent step indentation is lost

## 3. Validation

- [x] 3.1 `go test ./...` passes
- [ ] 3.2 CI passes on the PR (it runs the new gate on the merge ref)
