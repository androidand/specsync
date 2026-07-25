# Explicit Repo Resolution

## Why

specsync never resolves the target repository itself. When `-repo` is not supplied it omits `--repo` from the `gh` invocations entirely (`github.go:26`, `:42`, `:99`) and lets `gh` infer the target from the working directory.

For a **fork**, `gh` does not infer `origin`. It resolves to the upstream parent unless a default has been set with `gh repo set-default`. So in any fork — the normal shape for a customised tool — specsync silently targets **someone else's repository**.

Observed 2026-07-25 in a fork whose `origin` is the user's own repo and whose `upstream` is the parent project. Running specsync without `-repo` produced:

```
gh label create specsync --force: exit status 1
HTTP 404: Not Found (https://api.github.com/repos/<upstream-owner>/<upstream-name>/labels)
```

It failed safe **only because the user lacked write access to the parent.** The failure mode is entirely permission-dependent:

- No write access to the parent → 404, confusing but harmless.
- **Write access to the parent** (a maintainer, an org member, a fork inside the same org) → specsync **creates labels and files issues on the parent repository**, publishing internal planning content to a project the user does not own. On a public parent, that is an irreversible disclosure — issues are indexed quickly and deleting them does not reliably un-publish.

The user's `origin` was correct in every case. Nothing about the local configuration was ambiguous; specsync simply never looked at it.

This is a correctness and blast-radius defect, not an ergonomics one: the tool writes to a repository the user never named, chosen by an implicit rule most users do not know applies.

## What Changes

- **specsync resolves the target repository explicitly and always passes `--repo`.** No `gh` invocation may rely on implicit working-directory inference.
- **Resolution order**, first match wins:
  1. `-repo owner/name` (explicit flag — unchanged).
  2. The repository configured via `gh repo set-default`, when set, since that is the user's stated intent for this checkout.
  3. The `origin` remote.
- **Fork divergence is reported, never silently resolved.** When `origin` and `upstream` both exist and disagree, specsync states which it chose and how to override, so the choice is visible in the output rather than inferred.
- **A guard against writing to a repository the user does not own.** Before any write, specsync confirms the resolved target is the intended one; when the resolved target is a fork parent rather than `origin`, it refuses without an explicit `-repo`.
- **Dry-run reports the resolved target.** `-dry-run` currently prints `target: auto-detected from the current repo's git remote`, which is precisely the ambiguity that caused this. It prints the concrete `owner/name` instead.

## Capabilities

### New Capabilities
- `explicit-repo-resolution`: deterministic, stated resolution of the target repository, with fork-parent writes refused by default.

### Modified Capabilities
- `github-provider`: every `gh` invocation carries an explicit `--repo`.

## Non-Goals

- No change to board/project targeting (`-project`), which is already explicit.
- No change to the resolution order's first entry — `-repo` remains authoritative.
- specsync does not attempt to configure `gh repo set-default` on the user's behalf.

## Impact

- `github.go` — `repo` is resolved eagerly rather than left empty; all `gh` command construction gains `--repo`.
- Dry-run output — prints the resolved `owner/name` and which rule produced it.
- Behaviour change for existing fork users: runs that previously targeted the parent now target `origin`, or refuse. This is the fix, but it is a visible change and belongs in release notes.
- Any workflow that deliberately relied on implicit parent targeting must now pass `-repo` explicitly.
