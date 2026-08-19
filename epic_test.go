package specsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fakeEpicIssue is one in-memory issue tracked by fakeEpicProvider.
type fakeEpicIssue struct {
	title  string
	body   string
	labels []string
}

// fakeEpicProvider is an in-memory WorkProvider + IssueReader that mirrors
// GitHubProvider's Push/Find/marker semantics closely enough to exercise
// Epic's idempotency without shelling out to gh. One instance models one repo.
type fakeEpicProvider struct {
	repo    string
	issues  map[string]*fakeEpicIssue
	nextID  int
	creates int // number of "issue create" calls made, for idempotency assertions
}

func newFakeEpicProvider(repo string) *fakeEpicProvider {
	return &fakeEpicProvider{repo: repo, issues: map[string]*fakeEpicIssue{}}
}

func (f *fakeEpicProvider) Name() string { return "github:" + f.repo }

func (f *fakeEpicProvider) Push(_ context.Context, item WorkItem, existing *Ref) (Ref, error) {
	body := marker(item.Slug) + "\n\n" + item.Body
	if existing == nil {
		if found, _ := f.Find(context.Background(), item.Slug); found != nil {
			existing = found
		}
	}
	if existing == nil {
		f.nextID++
		f.creates++
		id := strconv.Itoa(f.nextID)
		f.issues[id] = &fakeEpicIssue{title: item.Title, body: body, labels: item.Labels}
		return Ref{Provider: f.Name(), ID: id, URL: f.urlFor(id)}, nil
	}
	iss, ok := f.issues[existing.ID]
	if !ok {
		return Ref{}, fmt.Errorf("fakeEpicProvider: issue %s not found", existing.ID)
	}
	iss.title = item.Title
	iss.body = body
	// Mirror labelDelta's "add" side: desired-but-not-current labels are
	// always added, unconditionally.
	current := map[string]bool{}
	for _, l := range iss.labels {
		current[l] = true
	}
	for _, l := range item.Labels {
		if !current[l] {
			iss.labels = append(iss.labels, l)
		}
	}
	return *existing, nil
}

func (f *fakeEpicProvider) Find(_ context.Context, slug string) (*Ref, error) {
	want := marker(slug)
	for id, iss := range f.issues {
		if strings.Contains(iss.body, want) {
			return &Ref{Provider: f.Name(), ID: id, URL: f.urlFor(id)}, nil
		}
	}
	return nil, nil
}

func (f *fakeEpicProvider) Get(_ context.Context, id string) (FetchedItem, error) {
	iss, ok := f.issues[id]
	if !ok {
		return FetchedItem{}, fmt.Errorf("fakeEpicProvider: issue %s not found", id)
	}
	return FetchedItem{ID: id, URL: f.urlFor(id), Title: iss.title, Body: iss.body, Labels: iss.labels}, nil
}

func (f *fakeEpicProvider) urlFor(id string) string {
	return "https://github.com/" + f.repo + "/issues/" + id
}

// seed pre-populates issue id with a body containing no marker (an existing
// bare issue reference, not a specsync-managed change).
func (f *fakeEpicProvider) seed(id, title, body string, labels []string) {
	if f.nextID < mustAtoi(id) {
		f.nextID = mustAtoi(id)
	}
	f.issues[id] = &fakeEpicIssue{title: title, body: body, labels: labels}
}

func mustAtoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func TestEpic_CreatesEpicAndWiresIssueRefChildren(t *testing.T) {
	root := t.TempDir()
	openspecDir := filepath.Join(root, "openspec")
	os.MkdirAll(openspecDir, 0o755)

	planning := newFakeEpicProvider("org/planning")
	backend := newFakeEpicProvider("org/backend")
	backend.seed("12", "Add backend support", "some existing description", nil)

	providerFor := func(repo string) WorkProvider {
		if repo == "org/backend" {
			return backend
		}
		return planning
	}

	res, err := Epic(context.Background(), EpicOptions{
		OpenSpecDir:      openspecDir,
		Title:            "Feature X: cross-repo widgets",
		Repo:             "org/planning",
		Children:         []string{"org/backend#12"},
		EpicProvider:     planning,
		ChildProviderFor: providerFor,
	})
	if err != nil {
		t.Fatalf("Epic: %v", err)
	}
	if !res.Created {
		t.Error("want Created=true on first invocation")
	}
	if len(res.Children) != 1 {
		t.Fatalf("want 1 child, got %d", len(res.Children))
	}
	if res.Children[0].Ref.ID != "12" || res.Children[0].Repo != "org/backend" {
		t.Errorf("child = %+v", res.Children[0])
	}

	// Epic issue's body must list the child in a "## Related" section.
	epicIssue := planning.issues[res.Ref.ID]
	if !strings.Contains(epicIssue.body, "## Related") || !strings.Contains(epicIssue.body, "org/backend#12") {
		t.Errorf("epic body missing Related child:\n%s", epicIssue.body)
	}
	// Epic issue must carry the type:epic + specsync labels.
	if !containsStr2(epicIssue.labels, "type:epic") || !containsStr2(epicIssue.labels, "specsync") {
		t.Errorf("epic labels = %v, want type:epic and specsync", epicIssue.labels)
	}

	// Child issue's body must be edited to point back at the epic.
	childIssue := backend.issues["12"]
	if !strings.Contains(childIssue.body, "## Related") || !strings.Contains(childIssue.body, epicIssue.title) && !strings.Contains(childIssue.body, res.Ref.URL) {
		t.Errorf("child body missing backlink to epic:\n%s", childIssue.body)
	}
	if !strings.Contains(childIssue.body, "some existing description") {
		t.Errorf("child body lost its original content:\n%s", childIssue.body)
	}
}

func TestEpic_IdempotentRerun(t *testing.T) {
	root := t.TempDir()
	openspecDir := filepath.Join(root, "openspec")
	os.MkdirAll(openspecDir, 0o755)

	planning := newFakeEpicProvider("org/planning")
	backend := newFakeEpicProvider("org/backend")
	backend.seed("12", "Add backend support", "desc", nil)

	providerFor := func(repo string) WorkProvider {
		if repo == "org/backend" {
			return backend
		}
		return planning
	}

	opts := EpicOptions{
		OpenSpecDir:      openspecDir,
		Title:            "Feature X: cross-repo widgets",
		Repo:             "org/planning",
		Children:         []string{"org/backend#12"},
		EpicProvider:     planning,
		ChildProviderFor: providerFor,
	}

	first, err := Epic(context.Background(), opts)
	if err != nil {
		t.Fatalf("Epic (first): %v", err)
	}
	firstBody := planning.issues[first.Ref.ID].body

	second, err := Epic(context.Background(), opts)
	if err != nil {
		t.Fatalf("Epic (second): %v", err)
	}

	if planning.creates != 1 {
		t.Errorf("want exactly 1 epic issue created across both runs, got %d", planning.creates)
	}
	if second.Ref.ID != first.Ref.ID {
		t.Errorf("second run created a different epic issue: %s vs %s", second.Ref.ID, first.Ref.ID)
	}
	if second.Created {
		t.Error("second run must converge onto the existing epic, not report Created")
	}
	if len(planning.issues) != 1 {
		t.Errorf("want exactly 1 epic issue in the store, got %d", len(planning.issues))
	}
	secondBody := planning.issues[second.Ref.ID].body
	if secondBody != firstBody {
		t.Errorf("re-run changed the epic body content:\nfirst:  %q\nsecond: %q", firstBody, secondBody)
	}

	// The child issue's marker must not have duplicated across re-runs.
	childBody := backend.issues["12"].body
	if n := strings.Count(childBody, "<!-- specsync:change="); n != 1 {
		t.Errorf("want exactly 1 identity marker in child body after 2 runs, got %d:\n%s", n, childBody)
	}
}

func TestEpic_MixedSlugAndIssueRefChildren(t *testing.T) {
	root := t.TempDir()
	openspecDir := filepath.Join(root, "openspec")
	cdir := filepath.Join(openspecDir, "changes", "frontend-widget-view")
	os.MkdirAll(cdir, 0o755)
	os.WriteFile(filepath.Join(cdir, "proposal.md"), []byte("# frontend-widget-view\n\nbody\n"), 0o644)
	os.WriteFile(filepath.Join(cdir, "tasks.md"), []byte("- [ ] task\n"), 0o644)

	planning := newFakeEpicProvider("org/planning")
	frontendRepo := newFakeEpicProvider("") // "" = current repo (auto-detect), matches slug children
	backend := newFakeEpicProvider("org/backend")
	backend.seed("12", "Add backend support", "desc", nil)

	providerFor := func(repo string) WorkProvider {
		switch repo {
		case "org/backend":
			return backend
		case "":
			return frontendRepo
		default:
			return planning
		}
	}

	res, err := Epic(context.Background(), EpicOptions{
		OpenSpecDir:      openspecDir,
		Title:            "Feature X: cross-repo widgets",
		Repo:             "org/planning",
		Children:         []string{"org/backend#12", "frontend-widget-view"},
		EpicProvider:     planning,
		ChildProviderFor: providerFor,
	})
	if err != nil {
		t.Fatalf("Epic: %v", err)
	}
	if len(res.Children) != 2 {
		t.Fatalf("want 2 children, got %d", len(res.Children))
	}

	var sawSlug, sawIssue bool
	for _, c := range res.Children {
		switch c.Kind {
		case "slug":
			sawSlug = true
			if c.Slug != "frontend-widget-view" {
				t.Errorf("slug child slug = %q", c.Slug)
			}
			if !c.Synced {
				t.Error("unsynced slug child must be reported as synced")
			}
		case "issue":
			sawIssue = true
			if c.Ref.ID != "12" {
				t.Errorf("issue child ID = %q", c.Ref.ID)
			}
		default:
			t.Errorf("unexpected kind %q", c.Kind)
		}
	}
	if !sawSlug || !sawIssue {
		t.Errorf("want both kinds present: slug=%v issue=%v", sawSlug, sawIssue)
	}

	// The slug child must now have a ref persisted on disk.
	refs, err := LoadRefs(cdir)
	if err != nil {
		t.Fatalf("LoadRefs: %v", err)
	}
	if len(refs) == 0 {
		t.Error("slug child should have a persisted ref after being synced")
	}

	// The slug child's issue must carry a backlink to the epic.
	var slugChildRef Ref
	for _, c := range res.Children {
		if c.Kind == "slug" {
			slugChildRef = c.Ref
		}
	}
	iss, ok := frontendRepo.issues[slugChildRef.ID]
	if !ok {
		t.Fatalf("slug child issue %s not found in frontend provider", slugChildRef.ID)
	}
	if !strings.Contains(iss.body, "## Related") {
		t.Errorf("slug child body missing backlink to epic:\n%s", iss.body)
	}
	if n := strings.Count(iss.body, "<!-- specsync:change="); n != 1 {
		t.Errorf("want exactly 1 identity marker in slug child body, got %d:\n%s", n, iss.body)
	}
}

func TestEpic_DryRunDoesNotPersistSlugRef(t *testing.T) {
	root := t.TempDir()
	openspecDir := filepath.Join(root, "openspec")
	cdir := filepath.Join(openspecDir, "changes", "frontend-widget-view")
	os.MkdirAll(cdir, 0o755)
	os.WriteFile(filepath.Join(cdir, "proposal.md"), []byte("# frontend-widget-view\n\nbody\n"), 0o644)
	os.WriteFile(filepath.Join(cdir, "tasks.md"), []byte("- [ ] task\n"), 0o644)

	planning := newFakeEpicProvider("org/planning")
	frontendRepo := newFakeEpicProvider("")

	providerFor := func(repo string) WorkProvider {
		if repo == "" {
			return frontendRepo
		}
		return planning
	}

	res, err := Epic(context.Background(), EpicOptions{
		OpenSpecDir:      openspecDir,
		Title:            "Feature X: cross-repo widgets",
		Repo:             "org/planning",
		Children:         []string{"frontend-widget-view"},
		DryRun:           true,
		EpicProvider:     planning,
		ChildProviderFor: providerFor,
	})
	if err != nil {
		t.Fatalf("Epic: %v", err)
	}
	if len(res.Children) != 1 {
		t.Fatalf("want 1 child, got %d", len(res.Children))
	}

	refs, err := LoadRefs(cdir)
	if err != nil {
		t.Fatalf("LoadRefs: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("dry run must not persist a ref to disk, got %v", refs)
	}
}

func TestEpic_UnknownChildErrors(t *testing.T) {
	root := t.TempDir()
	openspecDir := filepath.Join(root, "openspec")
	os.MkdirAll(openspecDir, 0o755)

	planning := newFakeEpicProvider("org/planning")
	_, err := Epic(context.Background(), EpicOptions{
		OpenSpecDir:      openspecDir,
		Title:            "Feature X",
		Repo:             "org/planning",
		Children:         []string{"nonexistent-slug"},
		EpicProvider:     planning,
		ChildProviderFor: func(string) WorkProvider { return planning },
	})
	if err == nil {
		t.Fatal("want an error for an unresolvable --child argument")
	}
	if planning.creates != 0 {
		t.Error("a bad --child argument must fail before the epic issue is created")
	}
}

func TestEpic_RequiresTitle(t *testing.T) {
	planning := newFakeEpicProvider("org/planning")
	_, err := Epic(context.Background(), EpicOptions{
		OpenSpecDir:      t.TempDir(),
		Title:            "   ",
		EpicProvider:     planning,
		ChildProviderFor: func(string) WorkProvider { return planning },
	})
	if err == nil {
		t.Fatal("want an error for an empty title")
	}
}

func TestStripLeadingMarker(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"no marker", "hello world", "hello world"},
		{"marker present", "<!-- specsync:change=foo -->\n\nhello world", "hello world"},
		{"marker with leading blank lines", "\n<!-- specsync:change=foo -->\n\nhello", "hello"},
		{"empty body", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripLeadingMarker(tt.body)
			if got != tt.want {
				t.Errorf("stripLeadingMarker(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestPushRelatedEdit_NoDoubleMarkerAcrossRuns(t *testing.T) {
	prov := newFakeEpicProvider("owner/repo")
	prov.seed("1", "My Change", marker("my-change")+"\n\noriginal content", []string{"specsync"})

	ref := Ref{Provider: prov.Name(), ID: "1", URL: prov.urlFor("1")}
	other := Ref{Provider: "github:owner/other", ID: "9", URL: "https://github.com/owner/other/issues/9"}

	if _, err := PushRelatedEdit(context.Background(), prov, ref, "my-change", []Ref{other}); err != nil {
		t.Fatalf("PushRelatedEdit (first): %v", err)
	}
	if _, err := PushRelatedEdit(context.Background(), prov, ref, "my-change", []Ref{other}); err != nil {
		t.Fatalf("PushRelatedEdit (second): %v", err)
	}

	body := prov.issues["1"].body
	if n := strings.Count(body, "<!-- specsync:change="); n != 1 {
		t.Errorf("want exactly 1 marker after 2 runs, got %d:\n%s", n, body)
	}
	if !strings.Contains(body, "original content") {
		t.Errorf("original content lost:\n%s", body)
	}
	if !strings.Contains(body, "owner/other#9") {
		t.Errorf("missing related link:\n%s", body)
	}
}

func containsStr2(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
