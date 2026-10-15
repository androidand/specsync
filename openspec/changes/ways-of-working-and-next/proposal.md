# Define your ways of working once and get the next step

## Why

The same flow should run every time, for people and for agents.

Today the flow lives in personal scripts, skills and docs. They differ per person and per repo. Nothing tells you where you are in it, or what comes next.

Each project has its own gates, branch names and review rules. A general tool cannot hard-code them. It can read them from one place and act on them.

## What

- A `flow` section in `openspec/specsync.yml`. It names the steps, their order and their gates.
- A default flow: new, start, build, gate, review, merge, clean up.
- `specsync next`: read the current state, print the next step and the exact command. Read-only.
- `specsync flow`: print the flow and mark where the current work is.
- Gates defined in config: commands per repo, run in order. Some are optional.
- Presets: `minimal`, `pr-review` (default), `strict`.
- `doctor` validates the config and says what is wrong.
- **Trust.** Gates and hooks are commands from a file in the repository. specsync runs them only after you trust that exact config with `specsync trust`. A changed config needs trust again. Otherwise it prints the commands and refuses.

Example:

```yaml
flow: pr-review
gates:
  - {name: lint, run: "make lint"}
  - {name: test, run: "make test"}
  - {name: e2e, run: "make e2e", optional: true}
worktrees: {dir: "../worktrees", branch: "{type}/{issue}-{slug}"}
text: {profile: conventional}
cleanup: {remote_branch: keep, archive: true}
sync: {strategy: rebase, after_review: merge, force: with-lease}
```

`next` reads facts from `status`: change, issue, branch, PR, checks, review, local state. It applies the flow. It never guesses.

```
$ specsync next
Tasks 7/9, gates not run for HEAD.
Next: specsync gates
```

## Security

A cloned repo must not run code just because you ran specsync in it. Trust is recorded per user, per repo, as a hash of the config. It is the same idea as `direnv allow`.

## Out of scope

- No agent launching, no orchestration, no daemon. A step runs only when someone invokes a command.
- No second source of truth. State comes from git, the provider and the change files.
- `next` suggests. It does not run.

## Fits

The principles say specsync is not a control plane. This stays true. `next` is advice from facts. Hooks are user-configured commands, like git hooks. The principle text gains one line: specsync provides mechanical verbs and a next-step oracle for the work itself, and never orchestrates agents.

## Related

- `status-pr-and-worktree-state`: the facts `next` reads.
- `work-relations-and-overview`: the related work `next` considers, such as blockers.
- `flow-verbs`: the commands `next` points to.
- `concise-change-text`, `worktree-cleanup`: add their config sections.
- `make-commands-discoverable`: `next` and `flow` are declared once like any command.

## Release note

Add `specsync next` and a `flow` section in `openspec/specsync.yml`: define your gates and steps once, and see the next step for the work in front of you.
