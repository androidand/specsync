# change-slices

## ADDED Requirements

### Requirement: Declare slices in tasks.md
specsync SHALL recognise a heading `## Slice <n>: <title>` in a change's `tasks.md` as
the start of a slice, with optional lines `after: <n>[, <m>]`, `budget: <limits>` and
`contract: <sentence>`, and SHALL attribute the task checkboxes that follow to that
slice. A change with no slice headings SHALL behave exactly as before.

#### Scenario: Two ordered slices
- **WHEN** `tasks.md` declares slice 1 and slice 2 with `after: 1`
- **THEN** `specsync slices` lists both with slice 2 ordered after slice 1
- **AND** each shows the progress of its own tasks

#### Scenario: No slices declared
- **WHEN** `tasks.md` has no slice headings
- **THEN** `specsync slices` reports that the change is a single slice
- **AND** no existing command changes behaviour

### Requirement: Validate the slice plan
`specsync validate` and `specsync audit` SHALL report slice numbers that are not
contiguous, `after` targets that do not exist or form a cycle, tasks that belong to no
slice, and slices without a contract.

#### Scenario: Cycle
- **WHEN** slice 1 declares `after: 2` and slice 2 declares `after: 1`
- **THEN** validation fails and names both slices

### Requirement: Report slice state from asserted facts
`specsync slices` SHALL show, per slice, its derived state (not started, branch exists,
change request open, merged) using the status join, and SHALL NOT infer a slice's
state from naming or similarity.

#### Scenario: Slice with an open change request
- **WHEN** a slice's branch has an open change request
- **THEN** its state is shown as open with the request's review and check state

### Requirement: Start a slice
`specsync slice start <n>` SHALL create the slice's branch and worktree following the
existing worktree conventions, basing it on the preceding slice's branch when `after`
is declared and otherwise on the default branch, and SHALL record the binding. It SHALL
NOT create a change request.

#### Scenario: Stacked slice
- **WHEN** slice 2 declares `after: 1` and `slice start 2` runs
- **THEN** the new branch is based on slice 1's branch
- **AND** the binding is recorded so `slices` shows it

### Requirement: Project slices to the tracker on request
`specsync sync -slices` SHALL project each slice as a child item of the change's issue
and each `after` as a blocked-by dependency, using the provider's capabilities, and
SHALL do nothing for slices without `-slices`.

#### Scenario: Provider without hierarchy
- **WHEN** the active provider has no child-item or dependency support
- **THEN** the slices are listed in the change's issue body instead
- **AND** no error is reported

### Requirement: Never split automatically
specsync SHALL NOT propose, infer or apply slice boundaries; slices exist only where the
author declared them.

#### Scenario: Large change without slices
- **WHEN** a change has no slice headings
- **THEN** specsync does not generate any
