# Tasks — Edit-path migration for next-tech & vitec-hyra

## Add updateIntegration mutation

- [ ] Create `updateIntegration` mutation in the integrations query file
  - Target: `PATCH /integrations/{id}` (FusionHub#3489 endpoint)
  - Invalidate `Integrations` query cache on success
  - Handle 4xx credential-verification errors sensibly (fast-follow: surface credential-specific message)

## Migrate next-tech edit flow

- [ ] Render edit via `DynamicFieldFormModal` in edit mode (`mode="edit"`, `buildConfigFromFields(fields, "edit")`)
  - Prefill: read integration config via existing API or accept blank-keeps-existing UX
  - Call `updateIntegration` mutation on submit
- [ ] Rewire `IntegrationDetailPage` to route next-tech edit through the new flow
- [ ] Delete `EditNextTechIntegrationModal.tsx` + its test file
- [ ] Delete `nextTechApi.ts` edit-only functions (keep or remove based on cleanup audit)

## Migrate vitec-hyra edit flow

- [ ] Same as next-tech: render via `DynamicFieldFormModal` edit mode + rewire `IntegrationDetailPage`
- [ ] Delete `EditVitecHyraIntegrationModal.tsx` + its test file
- [ ] Delete `vitecHyraApi.ts` edit-only functions

## Cleanup

- [ ] `pnpm knip` — remove any newly-orphaned exports
- [ ] `pnpm test:types` — verify no regressions
- [ ] `pnpm lint` — code style clean
- [ ] Verify 5 fully-migrated sources still work (finago, infor-m3, monitor-g5, tripletex, upsales)

## Testing

- [ ] End-to-end: edit next-tech integration, change a field, save, verify change appears in the list
- [ ] End-to-end: same for vitec-hyra
- [ ] Credential error handling: submit bad credentials, verify error message is sensible
