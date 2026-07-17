# Complete schema-driven migration for simple integrations (edit-path only)

## Status update (2026-07-13)

**Create-path migration: ✅ DONE** — Portal#4084 (PR #4182) deployed to production.
- All 7 sources (finago, infor-m3, monitor-g5, next-tech, tripletex, upsales, vitec-hyra) now add via `DynamicFieldFormModal`
- Registry entries deleted, hardcoded add-containers removed
- Blocking dependency FusionHub#3489 is deployed and live

**Remaining scope: Edit-path migration for 2 sources** (next-tech, vitec-hyra)
- Edit flow still uses custom modals (`EditNextTechIntegrationModal.tsx`, `EditVitecHyraIntegrationModal.tsx`)
- Now unblocked: FusionHub#3489 provides uniform `PATCH /integrations/{id}` endpoint + `verifyCredentials`

Other 5 sources (finago, infor-m3, monitor-g5, tripletex, upsales) have no custom edit paths; they are fully migrated.

## Context

Portal#4084 built the schema-driven dynamic engine (`DynamicFieldFormModal`) for rendering integration add/edit flows from FusionHub's `fields` schema. It deliberately split out the cleanup—deleting hardcoded source folders and custom modals—into this issue.

`SourcePicker` uses **registry-first** routing: hand-built components win; dynamic modal is the fallback for sources without a registry entry. This enables fail-safe rollout: migrating a source = deleting its registry entry; the switch happens explicitly per source, with no regressions at deploy time.

## What changes (remaining)

**Edit-path migration (next-tech, vitec-hyra only):**

- Add `updateIntegration` mutation → `PATCH /integrations/{id}` (FusionHub#3489 endpoint is ready)
- Render edit via `DynamicFieldFormModal` in `"edit"` mode (already supports `buildConfigFromFields(fields, "edit")`)
- Prefill integration data (can read from existing per-source config GETs, or accept blank-keeps-existing UX)
- Rewire `IntegrationDetailPage` to use the new mutation
- Delete `EditNextTechIntegrationModal.tsx`, `EditVitecHyraIntegrationModal.tsx`, and their associated test files
- Delete trimmed api files for next-tech/vitec-hyra (edit-only remnants)

**Cleanup:**
- `pnpm knip` for orphaned exports
- `pnpm test:types`, `pnpm lint`
- No breaking changes to other 5 sources (finago, infor-m3, monitor-g5, tripletex, upsales already fully migrated)

## Out of scope

- Create-path further work — all 7 sources done
- procountor — stays custom (dimensions pre-fetch)
- Finish-setup UX for finago/tripletex/monitor-g5 — tracked in separate change `show-finish-setup-on-integration-detail-page`

## Acceptance criteria

- [ ] `updateIntegration` mutation added (`PATCH /integrations/{id}`)
- [ ] Edit flow for next-tech & vitec-hyra renders via `DynamicFieldFormModal` in edit mode
- [ ] Prefill works correctly (integration data loads before user edits)
- [ ] `EditNextTechIntegrationModal.tsx` and `EditVitecHyraIntegrationModal.tsx` deleted
- [ ] Per-source edit api files (trimmed) deleted
- [ ] `pnpm knip`, `pnpm test:types`, `pnpm lint` clean
- [ ] End-to-end edit flow works: user can edit credentials and see changes reflected in list

## Related

- ExopenGitHub/portal#4084 — parent (create-path migration, ✅ done)
- ExopenGitHub/FusionHub#3489 — backend `fields` + `PATCH /integrations/{id}` + `verifyCredentials` (✅ deployed)
- `show-finish-setup-on-integration-detail-page` — separate change for ER-layer post-create flow
