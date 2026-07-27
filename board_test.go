package specsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// stockFake mirrors an unmodified GitHub board, whose Status options are
// "Todo" / "In Progress" / "Done" — note the capital P, which the default
// mapping ("In progress") must match case-insensitively.
func stockFake() *fakeBoard {
	f := defaultFake()
	f.options = []struct{ id, name string }{
		{"OPT_TODO", "Todo"},
		{"OPT_PROG", "In Progress"},
		{"OPT_DONE", "Done"},
	}
	return f
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

func orgTarget() BoardTarget { return BoardTarget{Owner: "org", Number: 6} }

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

// A stock GitHub board names its option "In Progress" (capital P) while the
// built-in default is "In progress": matching must be case-insensitive, resolve
// to the real option (not fall back positionally to "Todo"), and report the
// board's canonical casing.
func TestBoardStockCasingMatchesCaseInsensitively(t *testing.T) {
	f := stockFake()
	f.onBoardItemID = "ITEM_1"
	plan := project(t, f, orgTarget(), activeRef(), activeItem(), false)
	if plan.StatusName != "In Progress" {
		t.Fatalf("active stage on a stock board should map to \"In Progress\", got %q", plan.StatusName)
	}
	if !f.mutated("setStatus") {
		t.Fatalf("expected a Status update mutation")
	}
	// The mutation must target the In Progress option, not the positional Todo.
	var sawOption string
	for _, call := range f.calls {
		for _, a := range call {
			if strings.HasPrefix(a, "optionId=") {
				sawOption = strings.TrimPrefix(a, "optionId=")
			}
		}
	}
	if sawOption != "OPT_PROG" {
		t.Fatalf("setStatus used option %q, want OPT_PROG", sawOption)
	}
}

// Canonical-name resolution means no spurious rewrite when the board already
// carries the desired Status in its own casing.
func TestBoardStockCasingNoWriteWhenAlreadyCorrect(t *testing.T) {
	f := stockFake()
	f.onBoardItemID = "ITEM_1"
	f.currentStatus = "In Progress"
	project(t, f, orgTarget(), activeRef(), activeItem(), false)
	if f.mutated("setStatus") {
		t.Fatalf("must not write Status when it already matches (case-insensitively)")
	}
}

// managedStatusNames must recognize stock-cased values as specsync-managed:
// "Done" (archived default) on a stock board is overwritable, not human-set.
func TestBoardStockCasingManagedStatusOverwritten(t *testing.T) {
	f := stockFake()
	f.onBoardItemID = "ITEM_1"
	f.currentStatus = "Done"
	plan := project(t, f, orgTarget(), activeRef(), activeItem(), false)
	if !f.mutated("setStatus") {
		t.Fatalf("expected to overwrite a specsync-managed Status on a stock board")
	}
	if plan.StatusName != "In Progress" {
		t.Fatalf("StatusName = %q, want In Progress", plan.StatusName)
	}
}

// A configured mapping with different casing than the board resolves too.
func TestBoardConfiguredMappingIsCaseInsensitive(t *testing.T) {
	f := stockFake()
	f.onBoardItemID = "ITEM_1"
	target := orgTarget()
	target.StatusMapping = map[Stage]string{StageActive: "in progress"}
	plan := project(t, f, target, activeRef(), activeItem(), false)
	if plan.StatusName != "In Progress" {
		t.Fatalf("configured \"in progress\" should resolve to board option \"In Progress\", got %q", plan.StatusName)
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

// === Three-Way Merge Tests (Phase C) ===

// TestThreeWayMergeNoChange verifies that no update is pushed when neither local nor remote changed.
func TestThreeWayMergeNoChange(t *testing.T) {
	base := BoardBinding{
		LocalStageBase:     StageActive,
		RemoteOptionIDBase: "OPT_PROG",
	}

	decision := threeWayMerge(StageActive, "OPT_PROG", "", base)

	if decision.Action != "none" {
		t.Errorf("action = %q, want %q", decision.Action, "none")
	}
	if decision.LocalChanged || decision.RemoteChanged {
		t.Errorf("changes detected when nothing changed")
	}
}

// TestThreeWayMergeLocalChanged verifies that local progress is pushed when remote unchanged.
func TestThreeWayMergeLocalChanged(t *testing.T) {
	base := BoardBinding{
		LocalStageBase:     StageActive,
		RemoteOptionIDBase: "OPT_PROG",
	}

	decision := threeWayMerge(StageComplete, "OPT_PROG", "", base)

	if decision.Action != "push-local" {
		t.Errorf("action = %q, want %q", decision.Action, "push-local")
	}
	if !decision.LocalChanged {
		t.Errorf("LocalChanged should be true")
	}
	if decision.RemoteChanged {
		t.Errorf("RemoteChanged should be false")
	}
}

// TestThreeWayMergeRemoteChanged verifies human board move detection (CRITICAL: prevents clobbering).
// This is the key safety feature: when a human moves a card on the board, specsync respects it.
func TestThreeWayMergeRemoteChanged(t *testing.T) {
	base := BoardBinding{
		LocalStageBase:     StageActive,
		RemoteOptionIDBase: "OPT_PROG",
	}

	// Remote changed (human moved card from "In progress" to "Done")
	// but local didn't change
	decision := threeWayMerge(StageActive, "OPT_DONE", "", base)

	if decision.Action != "report-remote-move" {
		t.Errorf("action = %q, want %q (human move detection)", decision.Action, "report-remote-move")
	}
	if decision.LocalChanged {
		t.Errorf("LocalChanged should be false")
	}
	if !decision.RemoteChanged {
		t.Errorf("RemoteChanged should be true")
	}
}

// TestThreeWayMergeConflict verifies conflict detection when both sides changed.
func TestThreeWayMergeConflict(t *testing.T) {
	base := BoardBinding{
		LocalStageBase:     StageActive,
		RemoteOptionIDBase: "OPT_PROG",
	}

	// Both changed: local to complete, remote to blocked
	decision := threeWayMerge(StageComplete, "OPT_BLOCKED", "", base)

	if decision.Action != "report-conflict" {
		t.Errorf("action = %q, want %q", decision.Action, "report-conflict")
	}
	if !decision.LocalChanged {
		t.Errorf("LocalChanged should be true")
	}
	if !decision.RemoteChanged {
		t.Errorf("RemoteChanged should be true")
	}
}

// TestThreeWayMergeConvergenceCompleteToDone verifies that when both sides changed but
// converged to the same state, no conflict is reported.
func TestThreeWayMergeConvergenceCompleteToDone(t *testing.T) {
	base := BoardBinding{
		LocalStageBase:     StageActive,
		RemoteOptionIDBase: "OPT_PROG",
	}

	// Both changed: local to complete, remote to "Done" (which is what complete maps to)
	decision := threeWayMerge(StageComplete, "OPT_DONE", "OPT_DONE", base)

	if decision.Action != "converged" {
		t.Errorf("action = %q, want %q", decision.Action, "converged")
	}
}

// TestThreeWayMergeConflictWithExpected verifies that when both sides changed and didn't converge,
// conflict is still reported even with expectedRemote set.
func TestThreeWayMergeConflictWithExpected(t *testing.T) {
	base := BoardBinding{
		LocalStageBase:     StageActive,
		RemoteOptionIDBase: "OPT_PROG",
	}

	// Both changed: local to complete (expects OPT_DONE), but remote moved to BLOCKED
	decision := threeWayMerge(StageComplete, "OPT_BLOCKED", "OPT_DONE", base)

	if decision.Action != "report-conflict" {
		t.Errorf("action = %q, want %q", decision.Action, "report-conflict")
	}
}

// TestThreeWayMergeHumanMoveToBacklog verifies a specific real scenario.
func TestThreeWayMergeHumanMoveToBacklog(t *testing.T) {
	base := BoardBinding{
		LocalStageBase:     StageActive,
		RemoteOptionIDBase: "OPT_PROG",
	}

	// Human moved card back to backlog/todo
	decision := threeWayMerge(StageActive, "OPT_TODO", "", base)

	if decision.Action != "report-remote-move" {
		t.Errorf("action = %q, want %q", decision.Action, "report-remote-move")
	}
	if !strings.Contains(decision.Reason, "human") {
		t.Errorf("reason should mention human: %q", decision.Reason)
	}
}

// TestLoadBoardState verifies board state can be loaded from .specsync/board.json.
func TestLoadBoardState(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "test-change")

	// Create board state file
	mustWrite(t, filepath.Join(changeDir, ".specsync", "board.json"),
		`{"version":1,"bindings":{"github:owner/5":{"provider":"github","project_id":"PROJ_1","item_id":"ITEM_1","local_stage_base":"active","remote_option_id_base":"OPT_PROG","synced_at":"2026-07-16T00:00:00Z"}}}`)

	state, err := LoadBoardState(changeDir)
	if err != nil {
		t.Fatalf("LoadBoardState: %v", err)
	}

	if state.Version != 1 {
		t.Errorf("version = %d, want 1", state.Version)
	}
	if len(state.Bindings) != 1 {
		t.Errorf("bindings count = %d, want 1", len(state.Bindings))
	}

	binding, ok := state.Bindings["github:owner/5"]
	if !ok {
		t.Fatalf("binding not found")
	}
	if binding.ProjectID != "PROJ_1" {
		t.Errorf("project_id = %q, want %q", binding.ProjectID, "PROJ_1")
	}
}

// TestLoadBoardStateAbsent returns empty state when file is absent (not an error).
func TestLoadBoardStateAbsent(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "nonexistent-change")

	state, err := LoadBoardState(changeDir)
	if err != nil {
		t.Fatalf("LoadBoardState should not error on absent file: %v", err)
	}

	if state.Version != 1 {
		t.Errorf("version = %d, want 1", state.Version)
	}
	if len(state.Bindings) != 0 {
		t.Errorf("bindings should be empty, got %d", len(state.Bindings))
	}
}

// TestSaveBoardState verifies board state can be saved and loaded.
func TestSaveBoardState(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "test-change")

	state := BoardState{
		Version: 1,
		Bindings: map[string]BoardBinding{
			"owner:5:github": {
				Provider:           "github",
				ProjectID:          "PROJ_1",
				ItemID:             "ITEM_1",
				LocalStageBase:     StageActive,
				RemoteOptionIDBase: "OPT_PROG",
				SyncedAt:           time.Now().UTC(),
			},
		},
	}

	if err := SaveBoardState(changeDir, state); err != nil {
		t.Fatalf("SaveBoardState: %v", err)
	}

	// Verify no temp files remain
	tmps, _ := filepath.Glob(filepath.Join(changeDir, ".specsync", "*.tmp"))
	if len(tmps) > 0 {
		t.Fatalf("expected no temp files, got %v", tmps)
	}

	// Load and verify
	loaded, err := LoadBoardState(changeDir)
	if err != nil {
		t.Fatalf("LoadBoardState: %v", err)
	}

	if loaded.Version != 1 {
		t.Errorf("version = %d, want 1", loaded.Version)
	}
	binding, ok := loaded.Bindings["owner:5:github"]
	if !ok {
		t.Fatal("binding not found")
	}
	if binding.ProjectID != "PROJ_1" {
		t.Errorf("project_id = %q, want %q", binding.ProjectID, "PROJ_1")
	}
	if binding.LocalStageBase != StageActive {
		t.Errorf("local_stage_base = %q, want %q", binding.LocalStageBase, StageActive)
	}
}

// TestSaveBoardStateUpdate verifies binding is updated on re-sync (syncedAt changes).
func TestSaveBoardStateUpdate(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "test-change")

	oldTime := time.Now().UTC().Add(-time.Hour)
	state := BoardState{
		Version: 1,
		Bindings: map[string]BoardBinding{
			"owner:5:github": {
				Provider:           "github",
				ProjectID:          "PROJ_1",
				ItemID:             "ITEM_1",
				LocalStageBase:     StageActive,
				RemoteOptionIDBase: "OPT_PROG",
				SyncedAt:           oldTime,
			},
		},
	}

	if err := SaveBoardState(changeDir, state); err != nil {
		t.Fatalf("SaveBoardState: %v", err)
	}

	time.Sleep(time.Millisecond * 10)

	// Update syncedAt
	newTime := time.Now().UTC()
	binding := state.Bindings["owner:5:github"]
	binding.SyncedAt = newTime
	state.Bindings["owner:5:github"] = binding

	if err := SaveBoardState(changeDir, state); err != nil {
		t.Fatalf("SaveBoardState update: %v", err)
	}

	loaded, err := LoadBoardState(changeDir)
	if err != nil {
		t.Fatalf("LoadBoardState: %v", err)
	}

	loadedBinding := loaded.Bindings["owner:5:github"]
	if loadedBinding.SyncedAt.Before(newTime) {
		t.Errorf("syncedAt not updated: got %v, want >= %v", loadedBinding.SyncedAt, newTime)
	}
}

// TestMultipleBindingsCoexist verifies multiple bindings per change coexist.
func TestMultipleBindingsCoexist(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "test-change")

	state := BoardState{
		Version: 1,
		Bindings: map[string]BoardBinding{
			"owner1:5:github": {
				Provider:           "github",
				ProjectID:          "PROJ_1",
				ItemID:             "ITEM_1",
				LocalStageBase:     StageActive,
				RemoteOptionIDBase: "OPT_PROG",
				SyncedAt:           time.Now().UTC(),
			},
			"owner2:10:github": {
				Provider:           "github",
				ProjectID:          "PROJ_2",
				ItemID:             "ITEM_2",
				LocalStageBase:     StageBacklog,
				RemoteOptionIDBase: "OPT_TODO",
				SyncedAt:           time.Now().UTC(),
			},
		},
	}

	if err := SaveBoardState(changeDir, state); err != nil {
		t.Fatalf("SaveBoardState: %v", err)
	}

	loaded, err := LoadBoardState(changeDir)
	if err != nil {
		t.Fatalf("LoadBoardState: %v", err)
	}

	if len(loaded.Bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(loaded.Bindings))
	}

	b1 := loaded.Bindings["owner1:5:github"]
	if b1.ProjectID != "PROJ_1" {
		t.Errorf("b1 project_id = %q, want %q", b1.ProjectID, "PROJ_1")
	}

	b2 := loaded.Bindings["owner2:10:github"]
	if b2.ProjectID != "PROJ_2" {
		t.Errorf("b2 project_id = %q, want %q", b2.ProjectID, "PROJ_2")
	}
}

// TestLoadBoardStateMalformed verifies malformed board.json is handled.
func TestLoadBoardStateMalformed(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "test-change")

	// Write malformed JSON
	mustWrite(t, filepath.Join(changeDir, ".specsync", "board.json"), `not json`)

	_, err := LoadBoardState(changeDir)
	if err == nil {
		t.Fatal("expected error for malformed board.json")
	}
}

// TestSaveBoardStateAtomicWrite verifies no temp files remain after save.
func TestSaveBoardStateAtomicWrite(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "test-change")

	state := BoardState{
		Version: 1,
		Bindings: map[string]BoardBinding{
			"owner:5:github": {
				Provider:           "github",
				ProjectID:          "PROJ_1",
				ItemID:             "ITEM_1",
				LocalStageBase:     StageActive,
				RemoteOptionIDBase: "OPT_PROG",
				SyncedAt:           time.Now().UTC(),
			},
		},
	}

	// Save multiple times to ensure no temp files accumulate
	for i := 0; i < 3; i++ {
		if err := SaveBoardState(changeDir, state); err != nil {
			t.Fatalf("SaveBoardState %d: %v", i, err)
		}

		tmps, _ := filepath.Glob(filepath.Join(changeDir, ".specsync", "*.tmp"))
		if len(tmps) > 0 {
			t.Fatalf("iter %d: expected no temp files, got %v", i, tmps)
		}
	}
}

// TestArchivedStageOnBoard verifies archived changes map to terminal status.
func TestArchivedStageOnBoard(t *testing.T) {
	target := BoardTarget{
		Owner:  "owner",
		Number: 5,
	}

	// Archived stage should map to "Done" (defaultArchivedStatus)
	name, explicit := target.statusNameFor(StageArchived)
	if name != defaultArchivedStatus {
		t.Errorf("archived stage status = %q, want %q", name, defaultArchivedStatus)
	}
	if explicit {
		t.Error("archived stage should not be explicit without status mapping")
	}
}

// TestThreeWayMergeConvergence verifies both sides changed to the same value.
func TestThreeWayMergeConvergence(t *testing.T) {
	base := BoardBinding{
		LocalStageBase:     StageBacklog,
		RemoteOptionIDBase: "OPT_TODO",
	}

	// Both changed: local from backlog→active, remote from OPT_TODO→OPT_PROG
	// This is a "both changed" case — reported as conflict.
	decision := threeWayMerge(StageActive, "OPT_PROG", "", base)
	if decision.Action != "report-conflict" {
		t.Errorf("action = %q, want %q", decision.Action, "report-conflict")
	}
}

// TestThreeWayMergeConvergenceBothChanged verifies that when both sides changed
// but the local stage maps to the same option ID as the remote, it converges.
func TestThreeWayMergeConvergenceBothChanged(t *testing.T) {
	base := BoardBinding{
		LocalStageBase:     StageBacklog,
		RemoteOptionIDBase: "OPT_TODO",
	}

	// Both changed: local from backlog→active, remote from OPT_TODO→OPT_PROG
	// Local stage (active) maps to OPT_PROG, which matches the remote.
	decision := threeWayMerge(StageActive, "OPT_PROG", "OPT_PROG", base)
	if decision.Action != "converged" {
		t.Errorf("action = %q, want %q", decision.Action, "converged")
	}
}

func TestResolveBoard(t *testing.T) {
	tests := []struct {
		name         string
		projectFlag  string
		configBoard  string
		wantConfigured bool
		wantOwner    string
		wantNumber   int
		wantRule     BoardRule
	}{
		{
			name:         "flag wins over config",
			projectFlag:  "acme/6",
			configBoard:  "other/7",
			wantConfigured: true,
			wantOwner:    "acme",
			wantNumber:   6,
			wantRule:     BoardRuleFlag,
		},
		{
			name:         "config used when no flag",
			projectFlag:  "",
			configBoard:  "org/3",
			wantConfigured: true,
			wantOwner:    "org",
			wantNumber:   3,
			wantRule:     BoardRuleConfig,
		},
		{
			name:         "no board when neither flag nor config",
			projectFlag:  "",
			configBoard:  "",
			wantConfigured: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temp dir with optional config.
			dir := t.TempDir()
			if tt.configBoard != "" {
				cfgPath := filepath.Join(dir, SpecSyncConfigPath)
				cfgDir := filepath.Dir(cfgPath)
				if err := os.MkdirAll(cfgDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(cfgPath, []byte("board: "+tt.configBoard+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			got, err := ResolveBoard(tt.projectFlag, dir)
			if err != nil {
				t.Fatal(err)
			}
			if got.Configured() != tt.wantConfigured {
				t.Errorf("configured = %v, want %v", got.Configured(), tt.wantConfigured)
			}
			if tt.wantConfigured {
				if got.Owner != tt.wantOwner {
					t.Errorf("owner = %q, want %q", got.Owner, tt.wantOwner)
				}
				if got.Number != tt.wantNumber {
					t.Errorf("number = %d, want %d", got.Number, tt.wantNumber)
				}
				if got.Rule != tt.wantRule {
					t.Errorf("rule = %v, want %v", got.Rule, tt.wantRule)
				}
			}
		})
	}
}

func TestBoardRefusal(t *testing.T) {
	tests := []struct {
		name      string
		board     ResolvedBoard
		repo      ResolvedRepo
		wantError bool
	}{
		{
			name:      "no board — no refusal",
			board:     ResolvedBoard{},
			repo:      ResolvedRepo{Repo: "user/repo"},
			wantError: false,
		},
		{
			name:      "explicit flag — no refusal even on mismatch",
			board:     ResolvedBoard{BoardTarget: BoardTarget{Owner: "org", Number: 1}, Rule: BoardRuleFlag},
			repo:      ResolvedRepo{Repo: "user/repo"},
			wantError: false,
		},
		{
			name:      "config board matches repo owner — no refusal",
			board:     ResolvedBoard{BoardTarget: BoardTarget{Owner: "user", Number: 1}, Rule: BoardRuleConfig},
			repo:      ResolvedRepo{Repo: "user/repo"},
			wantError: false,
		},
		{
			name:      "config board on different owner — refusal",
			board:     ResolvedBoard{BoardTarget: BoardTarget{Owner: "org", Number: 1}, Rule: BoardRuleConfig},
			repo:      ResolvedRepo{Repo: "user/repo"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := BoardRefusal(tt.board, tt.repo)
			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

func TestParseSpecSyncConfig(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    SpecSyncConfig
	}{
		{
			name:  "board only",
			input: "board: acme/6\n",
			want:  SpecSyncConfig{Board: "acme/6"},
		},
		{
			name:  "with comments and blank lines",
			input: "# specsync config\n\nboard: org/3\n",
			want:  SpecSyncConfig{Board: "org/3"},
		},
		{
			name:  "empty",
			input: "",
			want:  SpecSyncConfig{},
		},
		{
			name:    "unknown key ignored",
			input:   "unknown: value\nboard: acme/6\n",
			want:    SpecSyncConfig{Board: "acme/6"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSpecSyncConfig([]byte(tt.input))
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestBoardRuleString(t *testing.T) {
	if got := BoardRuleFlag.String(); got != "flag" {
		t.Errorf("BoardRuleFlag.String() = %q, want %q", got, "flag")
	}
	if got := BoardRuleConfig.String(); got != "config" {
		t.Errorf("BoardRuleConfig.String() = %q, want %q", got, "config")
	}
	if got := BoardRule(99).String(); got != "unknown" {
		t.Errorf("unknown rule String() = %q, want %q", got, "unknown")
	}
}
