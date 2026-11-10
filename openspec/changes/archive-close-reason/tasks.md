# Tasks

## 1. Provider

- [ ] 1.1 Test first: `close` maps `completed`, `not-planned`, `duplicate` to the right `gh issue close` flags
- [ ] 1.2 Give the provider `close` a reason and an optional comment; providers without reasons close as today and note it

## 2. archive command

- [ ] 2.1 Add `-reason`, `-duplicate-of` and `-comment`, with the validation in the spec
- [ ] 2.2 Skip the unchecked-tasks refusal for `not-planned` and `duplicate`
- [ ] 2.3 Test: the recorded closed-state base is unchanged, so the next sync still defers

## 3. Docs

- [ ] 3.1 Regenerate `agent-help` and the README; document the abandoned-change path in AGENTS.md
