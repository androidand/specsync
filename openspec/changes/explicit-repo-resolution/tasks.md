# Tasks: Explicit Repo Resolution

## 1. Resolver

- [ ] 1.1 Add a `resolveRepo()` returning `(owner/name, rule, error)` implementing the order: explicit flag → CLI-configured default for this checkout → `origin`. `rule` names which branch produced the result, for reporting. Verify: `go test ./... -run ResolveRepo -count=1` covers each branch.
- [ ] 1.2 Read the CLI-configured default rather than assuming it is absent (`gh repo set-default` records it in the repo's git config). Verify: with a default configured, the resolver returns it and reports the rule; with none, it falls through to `origin`.
- [ ] 1.3 Detect fork divergence: `origin` and `upstream` both present and naming different repositories. Verify: test with both remotes returns divergence true; with only `origin`, false.
- [ ] 1.4 Refuse when the resolved target would be the fork parent and no explicit repository was supplied — permission is not intent, so the refusal must not depend on access checks. Verify: test asserts refusal with a clear message naming both candidates and the override flag.

## 2. Thread it through the provider

- [ ] 2.1 `github.go`: resolve eagerly at construction instead of leaving `repo` empty (`:26`, `:42`, `:99`), and pass an explicit repository argument on **every** command — label create, issue list, issue create, issue edit, issue close. Verify: `grep` shows no command construction path that omits the repo argument.
- [ ] 2.2 Audit for any other provider command built without the repo argument, including the board/project paths. Verify: every constructed command in the package carries it.

## 3. Reporting

- [ ] 3.1 Replace the dry-run line `target: auto-detected from the current repo's git remote` with the concrete `owner/name` plus the rule that selected it. Verify: a dry run in a fork prints the origin repository by name, not the word "auto-detected".
- [ ] 3.2 On fork divergence, state which repository was chosen and how to override. Verify: a dry run in a fork with a diverging upstream prints both candidates and the override flag.
- [ ] 3.3 Print the resolved target before any write in normal (non-dry-run) mode too. Verify: the first output line of a real sync names the target.

## 3b. Board resolution is per-repository

- [ ] 3b.1 Add repository-local board declaration (a tracked file in the repo, e.g. `openspec/specsync.yml` with `board: owner/number`). Resolution order: `-project` flag → repository-local declaration → **no board**. Verify: `go test ./... -run ResolveBoard -count=1` covers all three.
- [ ] 3b.2 Demote `SPECSYNC_PROJECT` so it can no longer select a board on its own. A shell-wide value that spans every repository is the exact mechanism by which personal work reaches a work board. Either remove it or require it to agree with the repository-local declaration. Verify: with the variable set and no local declaration, a sync touches no board.
- [ ] 3b.3 Refuse when the declared board's owner differs from the resolved repository's owner, unless the board was named explicitly. Verify: test asserts refusal for a personal repo against an org board and the reverse.
- [ ] 3b.4 Print the resolved board (or "no board") before any write. Verify: dry-run output names the board and the rule, or states that none is configured.
- [ ] 3b.5 Document in the README that there is deliberately **no global board default**, and why. Verify: the README states the rule and the reasoning.

## 4. Regression coverage

- [ ] 4.1 Regression test for the reported defect: in a checkout whose `origin` is the user's repository and whose `upstream` is a different parent, a sync with no explicit repository targets `origin` and never the parent. Verify: the test fails against the current implementation and passes after task 2.
- [ ] 4.2 Test that write access to the parent does not change the outcome — resolution is independent of permissions. Verify: the resolver is not permission-aware and no access check appears in its path.

## 5. Release

- [ ] 5.1 Release note: this is a **behaviour change**. Fork users whose runs previously targeted the parent will now target `origin` or be refused. State the `-repo` override for anyone who relied on the old behaviour. Verify: the note names the change and the override.
- [ ] 5.2 Document the resolution order in the README beside the `-repo` flag. Verify: the documented order matches the implementation.

## 6. Verification

- [ ] 6.1 `go build ./... && go vet ./...` clean; `go test ./... -count=1` with only pre-existing baseline failures. Record the baseline diff in task notes.
- [ ] 6.2 End-to-end in a real fork: confirm a dry run names `origin`, confirm a real sync creates the issue on `origin`, and confirm that explicitly naming the parent is still possible. Record the commands and output in task notes.
