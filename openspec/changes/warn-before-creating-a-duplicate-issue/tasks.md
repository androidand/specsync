## Phase 1 — make the create path legible

- [ ] Distinguish the create branch from the update branch in the dry-run printer
  (`cmd/specsync/main.go`), so a preview about to open an issue cannot read as a projection
- [ ] Stop printing the `issues/0` placeholder URL on the create path; state that no issue exists yet
- [ ] A test asserting a create-path dry run and an update-path dry run do not produce the same verb

## Phase 2 — surface prior art

- [ ] Add distinctive-word extraction: normalise, drop words under 4 characters, drop a small
  stopword list, dedupe
- [ ] Before a create, list open issues in the target repo and rank by shared distinctive words
- [ ] Print the top few with the shared words shown, and the `specsync adopt -issue N -change <slug>`
  line ready to copy
- [ ] Fetch failures must degrade to printing nothing — a hint is never worth failing a sync over

## Phase 3 — prove it

- [ ] A test with two differently-phrased titles sharing one distinctive word asserting the match is
  found. This is the case that motivated the change, so it is the one that must pass
- [ ] A test asserting unrelated titles produce no suggestions
- [ ] A test asserting a provider error during the lookup leaves the sync result unchanged
