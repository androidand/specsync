# Tasks: Adopt an existing issue into an existing change

## 1. The adopt command

- [x] 1.1 `specsync adopt -issue <n> [-change <slug>] [-repo owner/name] [-dry-run] [-force]`
- [x] 1.2 Resolve the change the same way `sync` does when `-change` is omitted
- [x] 1.3 Write `.specsync/refs.json` before touching the issue, so the link survives a failed edit
- [x] 1.4 Add the marker to the issue body, preserving existing content
- [x] 1.5 Refuse when the change is already bound elsewhere, or the issue carries another change's marker, unless `-force`

## 2. Guards on sync

- [x] 2.1 Stop when marker search returns more than one issue; list candidates and name `adopt`
- [x] 2.2 Refuse to write to a closed issue unless `-force`, naming the stale ref
- [x] 2.3 Treat a freshly written ref as authoritative over a lagging marker search

## 3. Verification

- [x] 3.1 Tests: adopt writes ref and marker; rejects conflicting bindings; dry-run writes nothing
- [x] 3.2 Test: sync stops on two marker matches instead of choosing
- [x] 3.3 Test: sync refuses a closed issue, and proceeds with `-force`
- [x] 3.4 Regression test reproducing the observed race: marker added, search not yet indexed, sync must not create a duplicate
- [x] 3.5 `agent-help adopt`, and a note in the skill on adopting rather than hand-editing bodies
