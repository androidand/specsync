# Tasks

## Fixes

- [ ] Fix YAML indentation in `.github/workflows/ci.yml` line 21
  - [ ] Verify correct indentation matches other steps
  - [ ] Test YAML parses correctly

## Verification

- [ ] Verify all dogfooding gates work in CI
  - [ ] build gate
  - [ ] vet gate
  - [ ] test gate
  - [ ] archive-hygiene gate
  - [ ] changelog-linking gate
  - [ ] task-dogfooding gate
  - [ ] structure-validation gate

- [ ] Run local test of workflow validation
  - [ ] Try: `gh workflow view ci.yml`
  - [ ] Try: `yamllint .github/workflows/ci.yml` (if available)
  - [ ] Try: running the workflow manually

## Documentation

- [ ] Update README if it mentions CI gates (verify they're accurate)
- [ ] Add comment to ci.yml if helpful

## Validation

- [ ] `make test` passes
- [ ] CI passes
- [ ] No regressions
