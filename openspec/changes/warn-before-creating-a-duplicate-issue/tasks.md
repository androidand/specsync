# Tasks

## 1. Make the create path legible

- [x] 1.1 A dry run that would open an issue prints `NEW  no issue exists yet`, not `created … issues/0`
- [x] 1.2 The dry-run summary says "would be created as new issues"
- [x] 1.3 A sync that finds its issue by marker (no cached ref) reports an update, not a create
- [x] 1.4 A dry run reads the repo's issues for real, so it previews the same create-or-update a real run takes

## 2. Surface prior art

- [x] 2.1 Distinctive-word extraction: words of 4+ characters, minus a small stopword list, deduplicated
- [x] 2.2 Before a create, list open issues and rank by shared distinctive words
- [x] 2.3 Print the top three with the shared words and a ready `specsync adopt -issue N -change <slug>` line
- [x] 2.4 A failed lookup prints nothing and changes nothing

## 3. Prove it

- [x] 3.1 Two differently-phrased titles sharing one distinctive word match
- [x] 3.2 Unrelated titles and generic words produce no suggestions
- [x] 3.3 A provider error during the lookup leaves the sync result unchanged
- [x] 3.4 A marker-adopted issue is reported as updated (fails without the fix)
