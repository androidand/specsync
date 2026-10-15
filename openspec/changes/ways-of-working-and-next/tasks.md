# Tasks

## Slice 1: Config and validation
- contract: A repo can declare its flow and gates, and `doctor` rejects a bad config.
- budget: files<=10, lines<=400

- [ ] 1.1 Define the `flow`, `gates`, `worktrees`, `text` and `cleanup` sections and a schema
- [ ] 1.2 Parse `openspec/specsync.yml` with defaults; keep the existing board setting working
- [ ] 1.3 `doctor` validates the schema: unknown keys, wrong types, missing gate commands, with line numbers
- [ ] 1.4 Presets `minimal`, `pr-review`, `strict`
- [ ] 1.5 `sync:` section: strategy, after_review, force; documented defaults
- [ ] 1.6 `specsync trust`: record the config hash per user and repo; refuse repo-defined commands without it; tests for fresh clone and changed config

## Slice 2: Next and flow
- after: 1
- contract: `next` and `flow` report the right step from asserted facts, and change nothing.
- budget: files<=10, lines<=400

- [ ] 2.1 Express the flow as steps with a precondition over status facts
- [ ] 2.2 `specsync next` with `-json`; read-only
- [ ] 2.3 `specsync flow` with the current-step marker
- [ ] 2.4 Fixture tests: no change, tasks open, gates missing, PR in review, merged and clean
- [ ] 2.5 Read-only guarantee test

## Slice 3: Docs
- after: 2
- contract: The flow is documented and discoverable.

- [ ] 3.1 Declare `next` and `flow` once as `Command` types; docs are generated
- [ ] 3.2 Add the principle line to the agent guide; update `site/features.json`
- [ ] 3.3 Run `specsync changelog -fail-on-unlinked-commits`
