package specsync

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// fakeBoard is a configurable fake of `gh api graphql` for the board projection.
// It answers queries by inspecting the GraphQL query text and records every call
// (and, separately, every mutation) so tests can assert idempotency and the
// zero-mutation dry-run contract.
type fakeBoard struct {
	isOrg         bool   // org namespace resolves the project; else user (org query errors)
	projectID     string // node id the fake resolves the project to
	statusFieldID string
	options       []struct{ id, name string } // Status options, in board order

	issueNodeID   string
	onBoardItemID string // "" => issue is not yet on the target board
	currentStatus string // current Status on the target board item
	assigneeCount int

	viewerLogin string
	viewerID    string

	calls     [][]string
	mutations []string
}

func (f *fakeBoard) run(_ context.Context, args ...string) (string, error) {
	f.calls = append(f.calls, args)
	q := graphqlQuery(args)
	switch {
	case strings.Contains(q, "addProjectV2ItemById"):
		f.mutations = append(f.mutations, "add")
		return `{"addProjectV2ItemById":{"item":{"id":"ITEM_NEW"}}}`, nil
	case strings.Contains(q, "updateProjectV2ItemFieldValue"):
		f.mutations = append(f.mutations, "setStatus")
		return `{"updateProjectV2ItemFieldValue":{"projectV2Item":{"id":"ITEM_NEW"}}}`, nil
	case strings.Contains(q, "addAssigneesToAssignable"):
		f.mutations = append(f.mutations, "assign")
		return `{"addAssigneesToAssignable":{"assignable":{"id":"ISSUE"}}}`, nil
	case strings.Contains(q, "projectItems"):
		return f.membershipJSON(), nil
	case strings.Contains(q, "fields(first"):
		return f.schemaJSON(), nil
	case strings.Contains(q, "viewer"):
		return fmt.Sprintf(`{"viewer":{"login":%q,"id":%q}}`, f.viewerLogin, f.viewerID), nil
	case strings.Contains(q, "organization(login"):
		if !f.isOrg {
			return "", fmt.Errorf("gh: Could not resolve to an Organization with the login")
		}
		return fmt.Sprintf(`{"organization":{"projectV2":{"id":%q}}}`, f.projectID), nil
	case strings.Contains(q, "user(login: $owner)"):
		return fmt.Sprintf(`{"user":{"projectV2":{"id":%q}}}`, f.projectID), nil
	case strings.Contains(q, "user(login: $login)"):
		return `{"user":{"id":"USER_X"}}`, nil
	default:
		return "", fmt.Errorf("fakeBoard: unhandled query: %s", q)
	}
}

func (f *fakeBoard) schemaJSON() string {
	var opts []string
	for _, o := range f.options {
		opts = append(opts, fmt.Sprintf(`{"id":%q,"name":%q}`, o.id, o.name))
	}
	// A leading non-single-select node (empty object) exercises the field filter.
	return fmt.Sprintf(`{"node":{"fields":{"nodes":[{},{"id":%q,"name":"Status","options":[%s]}]}}}`,
		f.statusFieldID, strings.Join(opts, ","))
}

func (f *fakeBoard) membershipJSON() string {
	items := ""
	if f.onBoardItemID != "" {
		fieldVals := ""
		if f.currentStatus != "" {
			fieldVals = fmt.Sprintf(`{"__typename":"ProjectV2ItemFieldSingleSelectValue","name":%q,"field":{"id":%q}}`,
				f.currentStatus, f.statusFieldID)
		}
		items = fmt.Sprintf(`{"id":%q,"project":{"id":%q},"fieldValues":{"nodes":[%s]}}`,
			f.onBoardItemID, f.projectID, fieldVals)
	}
	return fmt.Sprintf(`{"repository":{"issue":{"id":%q,"assignees":{"totalCount":%d},"projectItems":{"nodes":[%s]}}}}`,
		f.issueNodeID, f.assigneeCount, items)
}

func graphqlQuery(args []string) string {
	for _, a := range args {
		if strings.HasPrefix(a, "query=") {
			return strings.TrimPrefix(a, "query=")
		}
	}
	return ""
}

func (f *fakeBoard) mutated(name string) bool {
	for _, m := range f.mutations {
		if m == name {
			return true
		}
	}
	return false
}

func defaultFake() *fakeBoard {
	return &fakeBoard{
		isOrg:         true,
		projectID:     "PVT_1",
		statusFieldID: "FIELD_STATUS",
		options: []struct{ id, name string }{
			{"OPT_TODO", "Ready for development"},
			{"OPT_PROG", "In progress"},
			{"OPT_DONE", "Done"},
		},
		issueNodeID: "ISSUE_1",
		viewerLogin: "octocat",
		viewerID:    "USER_ME",
	}
}

func activeRef() Ref       { return Ref{Provider: "github", ID: "5", URL: "https://github.com/o/r/issues/5"} }
func activeItem() WorkItem { return WorkItem{Slug: "s", Title: "T", Stage: StageActive} }

func project(t *testing.T, f *fakeBoard, target BoardTarget, ref Ref, item WorkItem, dry bool) BoardPlan {
	t.Helper()
	prov := NewGitHubProviderFunc(f.run)
	plan, err := prov.ProjectOntoBoard(context.Background(), target, ref, item, dry)
	if err != nil {
		t.Fatalf("ProjectOntoBoard: %v", err)
	}
	return plan
}

func orgTarget() BoardTarget { return BoardTarget{Owner: "ExopenGitHub", Number: 6} }

func TestBoardResolvesOrgProject(t *testing.T) {
	f := defaultFake()
	f.onBoardItemID = "ITEM_1" // already on board so we don't add
	plan := project(t, f, orgTarget(), activeRef(), activeItem(), false)
	if plan.ProjectID != "PVT_1" {
		t.Fatalf("project id = %q, want PVT_1", plan.ProjectID)
	}
}

func TestBoardFallsBackToUserProject(t *testing.T) {
	f := defaultFake()
	f.isOrg = false // org query errors; user query must resolve it
	f.onBoardItemID = "ITEM_1"
	plan := project(t, f, BoardTarget{Owner: "someuser", Number: 3}, activeRef(), activeItem(), false)
	if plan.ProjectID != "PVT_1" {
		t.Fatalf("expected user-namespace fallback to resolve PVT_1, got %q", plan.ProjectID)
	}
}

func TestBoardMapsActiveStatusToOptionAndSets(t *testing.T) {
	f := defaultFake()
	f.onBoardItemID = "ITEM_1" // present, no status yet
	plan := project(t, f, orgTarget(), activeRef(), activeItem(), false)
	if plan.StatusName != "In progress" {
		t.Fatalf("active stage should map to In progress, got %q", plan.StatusName)
	}
	if !f.mutated("setStatus") {
		t.Fatalf("expected a Status update mutation")
	}
	if f.mutated("add") {
		t.Fatalf("issue already on board: must not add again")
	}
}

func TestBoardArchivedMapsToTerminal(t *testing.T) {
	f := defaultFake()
	f.onBoardItemID = "ITEM_1"
	item := WorkItem{Slug: "s", Title: "T", Stage: StageArchived}
	plan := project(t, f, orgTarget(), activeRef(), item, false)
	if plan.StatusName != "Done" {
		t.Fatalf("archived stage should map to Done, got %q", plan.StatusName)
	}
}

func TestBoardUnknownConfiguredStatusFailsLoud(t *testing.T) {
	f := defaultFake()
	f.onBoardItemID = "ITEM_1"
	target := orgTarget()
	target.StatusMapping = map[Stage]string{StageActive: "Nonexistent"}
	prov := NewGitHubProviderFunc(f.run)
	_, err := prov.ProjectOntoBoard(context.Background(), target, activeRef(), activeItem(), false)
	if err == nil {
		t.Fatalf("expected an error for an unknown configured status")
	}
	// Must list the valid options so the operator can fix the config.
	for _, want := range []string{"In progress", "Done", "Ready for development"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error should list valid option %q, got: %v", want, err)
		}
	}
}

func TestBoardEnsureOnBoardWhenAbsent(t *testing.T) {
	f := defaultFake()
	f.onBoardItemID = "" // absent
	plan := project(t, f, orgTarget(), activeRef(), activeItem(), false)
	if !plan.AddedToBoard {
		t.Fatalf("expected AddedToBoard when the issue is absent")
	}
	if !f.mutated("add") {
		t.Fatalf("expected an addProjectV2ItemById mutation")
	}
	if plan.AlreadyOnBoard {
		t.Fatalf("AlreadyOnBoard must be false for an absent issue")
	}
}

func TestBoardIdempotentWhenPresent(t *testing.T) {
	f := defaultFake()
	f.onBoardItemID = "ITEM_1"
	plan := project(t, f, orgTarget(), activeRef(), activeItem(), false)
	if !plan.AlreadyOnBoard {
		t.Fatalf("expected AlreadyOnBoard when present")
	}
	if f.mutated("add") {
		t.Fatalf("re-running must not add the item again")
	}
}

func TestBoardDoesNotClobberHumanStatus(t *testing.T) {
	f := defaultFake()
	f.onBoardItemID = "ITEM_1"
	f.currentStatus = "Ready for development" // a human-set, non-managed status
	plan := project(t, f, orgTarget(), activeRef(), activeItem(), false)
	if f.mutated("setStatus") {
		t.Fatalf("must not overwrite a human-set Status")
	}
	if plan.StatusSkipped == "" {
		t.Fatalf("expected a StatusSkipped reason, got plan %+v", plan)
	}
}

func TestBoardOverwritesOwnStatus(t *testing.T) {
	f := defaultFake()
	f.onBoardItemID = "ITEM_1"
	f.currentStatus = "Done" // a specsync-managed value (archived default); active wants In progress
	plan := project(t, f, orgTarget(), activeRef(), activeItem(), false)
	if !f.mutated("setStatus") {
		t.Fatalf("expected to overwrite a specsync-managed Status")
	}
	if plan.StatusName != "In progress" {
		t.Fatalf("StatusName = %q, want In progress", plan.StatusName)
	}
}

func TestBoardNoStatusWriteWhenAlreadyCorrect(t *testing.T) {
	f := defaultFake()
	f.onBoardItemID = "ITEM_1"
	f.currentStatus = "In progress" // already the desired value
	project(t, f, orgTarget(), activeRef(), activeItem(), false)
	if f.mutated("setStatus") {
		t.Fatalf("must not write Status when it already matches")
	}
}

func TestBoardAssignsViewerWhenUnassigned(t *testing.T) {
	f := defaultFake()
	f.onBoardItemID = "ITEM_1"
	f.assigneeCount = 0
	plan := project(t, f, orgTarget(), activeRef(), activeItem(), false)
	if plan.AssigneeLogin != "octocat" {
		t.Fatalf("expected viewer octocat to be assigned, got %q", plan.AssigneeLogin)
	}
	if !f.mutated("assign") {
		t.Fatalf("expected an addAssigneesToAssignable mutation")
	}
}

func TestBoardDoesNotClobberExistingAssignee(t *testing.T) {
	f := defaultFake()
	f.onBoardItemID = "ITEM_1"
	f.assigneeCount = 1 // already assigned by a human
	plan := project(t, f, orgTarget(), activeRef(), activeItem(), false)
	if f.mutated("assign") {
		t.Fatalf("must not assign when the issue already has an assignee")
	}
	if plan.AssignSkipped == "" {
		t.Fatalf("expected an AssignSkipped reason")
	}
}

func TestBoardDryRunMakesNoCalls(t *testing.T) {
	f := defaultFake()
	f.onBoardItemID = "" // even for an off-board change
	plan := project(t, f, orgTarget(), activeRef(), activeItem(), true)
	if len(f.calls) != 0 {
		t.Fatalf("dry run must issue zero gh calls, got %d: %v", len(f.calls), f.calls)
	}
	if len(f.mutations) != 0 {
		t.Fatalf("dry run must make no mutations")
	}
	// The plan still previews the intended board changes.
	if plan.StatusName != "In progress" || plan.AssigneeLogin != "me" || !plan.AddedToBoard {
		t.Fatalf("dry-run plan should preview add/status/assign, got %+v", plan)
	}
}

func TestBoardUnconfiguredIsNoOp(t *testing.T) {
	f := defaultFake()
	plan := project(t, f, BoardTarget{}, activeRef(), activeItem(), false)
	if len(f.calls) != 0 {
		t.Fatalf("an unconfigured target must issue zero gh calls, got %v", f.calls)
	}
	if plan != (BoardPlan{}) {
		t.Fatalf("expected a zero plan for an unconfigured target, got %+v", plan)
	}
}

func TestSyncWithoutProjectMakesNoBoardCalls(t *testing.T) {
	dir := t.TempDir()
	cdir := filepath.Join(dir, "changes", "add-thing")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Add thing\n\nbody\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [ ] 1.1 do it\n")
	var calls [][]string
	seenGraphQL := false
	run := func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		if len(args) >= 2 && args[0] == "api" && args[1] == "graphql" {
			seenGraphQL = true
		}
		switch {
		case len(args) >= 2 && args[0] == "issue" && args[1] == "list":
			return "[]", nil
		case len(args) >= 2 && args[0] == "issue" && args[1] == "create":
			return "https://github.com/o/r/issues/9", nil
		default:
			return "", nil
		}
	}
	prov := NewGitHubProviderFunc(run)
	if _, err := Sync(context.Background(), Options{OpenSpecDir: dir, Provider: prov, Reconcile: false}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if seenGraphQL {
		t.Fatalf("sync without -project must make no `gh api graphql` calls")
	}
}
