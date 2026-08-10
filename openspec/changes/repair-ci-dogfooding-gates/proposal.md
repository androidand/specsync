# Repair CI dogfooding gates

## Context

SpecSync dogfoods itself: every issue in the GitHub repository is generated from an OpenSpec change in `openspec/changes/` via the sync workflow. The CI pipeline enforces this dogfooding by validating:

1. Archive hygiene (completed changes are moved to `changes/archive/`)
2. Changelog commit linking (every commit references its change's issue)
3. Task-dogfooding (task checklist status matches code changes)
4. OpenSpec structure validation

During investigation, I found a bug in `.github/workflows/ci.yml` that could silently skip changelog validation.

## Issues Found

### 1. YAML Indentation Error in ci.yml (Line 21)

**File**: `.github/workflows/ci.yml`

**Current (broken)**:
```yaml
      - name: Enforce changelog commit linking
        run: go run ./cmd/specsync changelog -resolve-refs
```

Wait, let me check the actual file again. The indentation issue was on line 21.

Looking at the ci.yml I read earlier:

```yaml
      - run: go vet ./...
      - run: go test ./...
      - name: Enforce OpenSpec archive hygiene
        run: go run ./cmd/specsync release-plan -fail-on-archive-candidates
       - name: Enforce changelog commit linking
        run: go run ./cmd/specsync changelog -resolve-refs
```

The issue is clear: line 21 (`- name: Enforce changelog commit linking`) has incorrect indentation. It should have the same indentation as the line above it (`- name: Enforce OpenSpec archive hygiene`), but it's indented one space less.

This causes YAML parsing to fail or skip the step.

**Fix**: Correct the indentation:

```yaml
      - name: Enforce changelog commit linking
        run: go run ./cmd/specsync changelog -resolve-refs
```

## Proposed Changes

### 1. Fix YAML Indentation

Correct the indentation of the "Enforce changelog commit linking" step in `.github/workflows/ci.yml`.

### 2. Verify All Dogfooding Gates

Ensure all validation steps work:

- [ ] `go build ./...` — builds binary
- [ ] `go vet ./...` — code quality
- [ ] `go test ./...` — unit tests
- [ ] `release-plan -fail-on-archive-candidates` — archive hygiene
- [ ] `changelog -resolve-refs` — changelog linking
- [ ] `audit-tasks -fail-on-mismatch` — task dogfooding
- [ ] `validate` — OpenSpec structure

### 3. Add Validation for New Commands

If new validation commands are added in the future, ensure they're included in CI.

## Impact

**Severity**: Medium

- **Manifestation**: Changelog validation was potentially being skipped
- **Effect**: Commits without issue references could merge despite `-fail-on-unlinked-commits` flag
- **Duration**: Unclear; the bug may have been present for several releases

**Fix**: One-line indentation correction; no logic changes.

## Related Changes

None; this is a standalone bugfix.

## Release Notes

Fixed YAML indentation bug in CI workflow that could silently skip changelog validation. No functional changes to SpecSync; this is a CI/dogfooding improvement only.
