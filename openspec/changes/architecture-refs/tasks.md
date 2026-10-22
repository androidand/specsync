# Tasks: architecture references in links.md

## Parse (`change.go`)
- [ ] `parseLinksMD` returns architecture refs from `## Architecture` entries of the form `<scheme>:<path>`
- [ ] Architecture refs never appear in related links or dependencies
- [ ] Other entries under `## Architecture` keep today's behaviour
- [ ] Malformed scheme entries are skipped with a warning naming the file and line

## Work graph (`graph.go`, `graph_build.go`)
- [ ] Node kind `arch` (ID kind-prefixed, label = element id)
- [ ] Edge kind `spec-arch`, source `links.md`
- [ ] `relate -change` and `trace -json` include arch nodes and edges
- [ ] `relate -arch <scheme:repo/id>` lists covering changes and their issues

## Issue body (`render`)
- [ ] Sync renders `## Architecture` with the element ids when refs exist; nothing when none

## Tests
- [ ] Probe from the architecture tool: entries `archsync:owner/repo/x` and a URL control under `## Architecture`; arch refs parsed, URL still related, rendered Related section has no arch entry
- [ ] Graph: one change with two arch refs yields two `spec-arch` edges with source `links.md`
- [ ] `relate -arch` returns exactly the changes that assert the element (negative: name match without an entry returns nothing)
- [ ] Issue body golden with and without an Architecture section
- [ ] Full suite green; `boundary_test.go` (stdlib-only) unaffected
