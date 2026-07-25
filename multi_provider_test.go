package specsync

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// multiProvider is a fake provider that records Push calls and returns a
// configurable ref. It can be configured to fail on Find or Push.
type multiProvider struct {
	name   string
	ref    Ref
	fail   error
	pushed WorkItem
}

func (m *multiProvider) Name() string { return m.name }

func (m *multiProvider) Push(_ context.Context, item WorkItem, _ *Ref) (Ref, error) {
	if m.fail != nil {
		return Ref{}, m.fail
	}
	m.pushed = item
	return m.ref, nil
}

func (m *multiProvider) Find(_ context.Context, _ string) (*Ref, error) {
	if m.fail != nil {
		return nil, m.fail
	}
	return &m.ref, nil
}

func TestSyncFanOutToTwoProviders(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "c1")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# c1\n\nbody\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [ ] first task\n")

	// No refs yet — both providers will create new issues.
	provA := &multiProvider{
		name: "github",
		ref:  Ref{Provider: "github", ID: "42", URL: "https://github.com/o/r/issues/42"},
	}
	provB := &multiProvider{
		name: "beads",
		ref:  Ref{Provider: "beads", ID: "bd-1", URL: "bd://bd-1"},
	}

	res, err := Sync(context.Background(), Options{
		OpenSpecDir: root,
		Providers:   []WorkProvider{provA, provB},
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if res.Created != 2 {
		t.Errorf("want 2 created, got %d", res.Created)
	}

	if len(res.Items) != 1 {
		t.Fatalf("want 1 item, got %d", len(res.Items))
	}

	item := res.Items[0]
	if len(item.Providers) != 2 {
		t.Fatalf("want 2 provider results, got %d", len(item.Providers))
	}

	// Check both providers report created.
	for _, pr := range item.Providers {
		if !pr.Created {
			t.Errorf("provider %s: want created=true", pr.ProviderName)
		}
		if pr.URL == "" {
			t.Errorf("provider %s: want URL, got empty", pr.ProviderName)
		}
	}

	// Check refs.json has both entries.
	refsData, err := os.ReadFile(refCachePath(cdir))
	if err != nil {
		t.Fatalf("read refs.json: %v", err)
	}
	var refs map[string]Ref
	if err := json.Unmarshal(refsData, &refs); err != nil {
		t.Fatalf("unmarshal refs.json: %v", err)
	}
	if _, ok := refs["github"]; !ok {
		t.Error("refs.json missing github entry")
	}
	if _, ok := refs["beads"]; !ok {
		t.Error("refs.json missing beads entry")
	}
}

func TestSyncFanOutOneProviderFails(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "c1")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# c1\n\nbody\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [ ] first task\n")

	provOK := &multiProvider{
		name: "github",
		ref:  Ref{Provider: "github", ID: "42", URL: "https://github.com/o/r/issues/42"},
	}
	provFail := &multiProvider{
		name: "beads",
		fail: os.ErrNotExist, // beads is down
	}

	res, err := Sync(context.Background(), Options{
		OpenSpecDir: root,
		Providers:   []WorkProvider{provOK, provFail},
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// The OK provider should have succeeded.
	if res.Created != 1 {
		t.Errorf("want 1 created, got %d", res.Created)
	}

	if len(res.Items) != 1 {
		t.Fatalf("want 1 item, got %d", len(res.Items))
	}

	item := res.Items[0]
	if len(item.Providers) != 2 {
		t.Fatalf("want 2 provider results, got %d", len(item.Providers))
	}

	// Check github succeeded.
	var githubOK, beadsErr bool
	for _, pr := range item.Providers {
		if pr.ProviderName == "github" {
			if pr.Error != nil {
				t.Errorf("github should not have errored: %v", pr.Error)
			}
			githubOK = true
		}
		if pr.ProviderName == "beads" {
			if pr.Error == nil {
				t.Error("beads should have errored")
			}
			beadsErr = true
		}
	}
	if !githubOK || !beadsErr {
		t.Errorf("missing expected provider results: github=%v, beads=%v", githubOK, beadsErr)
	}

	// Check only github ref was saved.
	refsData, err := os.ReadFile(refCachePath(cdir))
	if err != nil {
		t.Fatalf("read refs.json: %v", err)
	}
	var refs map[string]Ref
	if err := json.Unmarshal(refsData, &refs); err != nil {
		t.Fatalf("unmarshal refs.json: %v", err)
	}
	if _, ok := refs["github"]; !ok {
		t.Error("refs.json missing github entry")
	}
	if _, ok := refs["beads"]; ok {
		t.Error("refs.json should not have beads entry after failure")
	}
}

func TestSyncFanOutInboundUnion(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "c1")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# c1\n\nbody\n")
	// Two tasks: issue A checks task 1, issue B checks task 2.
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [ ] first task\n- [ ] second task\n")

	// Pre-seed refs so reconcile runs pre-push.
	mustWrite(t, refCachePath(cdir), `{
		"github":{"provider":"github","id":"42","url":"https://github.com/o/r/issues/42"},
		"beads":{"provider":"beads","id":"bd-1","url":"bd://bd-1"}
	}`)

	// GitHub issue has first task checked.
	provA := &fakeIssueProvider{
		ref:  Ref{Provider: "github", ID: "42", URL: "https://github.com/o/r/issues/42"},
		body: "<!-- specsync:change=c1 -->\n\n## Tasks\n\n- [x] first task\n- [ ] second task\n",
	}

	// Beads has second task checked (via TaskStateReader).
	provB := &fakeTaskStateProvider{
		states: map[string]bool{
			"first task":  false,
			"second task": true,
		},
		ref: Ref{Provider: "beads", ID: "bd-1", URL: "bd://bd-1"},
	}

	res, err := Sync(context.Background(), Options{
		OpenSpecDir: root,
		Providers:   []WorkProvider{provA, provB},
		Reconcile:   true,
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// Both tasks should be checked now (union from both providers).
	got, _ := os.ReadFile(filepath.Join(cdir, "tasks.md"))
	if !strings.Contains(string(got), "- [x] first task") {
		t.Errorf("first task not checked after union:\n%s", got)
	}
	if !strings.Contains(string(got), "- [x] second task") {
		t.Errorf("second task not checked after union:\n%s", got)
	}

	// Two flips total: one from each provider.
	if len(res.Items[0].Flips) != 2 {
		t.Errorf("want 2 flips, got %d: %+v", len(res.Items[0].Flips), res.Items[0].Flips)
	}
}

// fakeTaskStateProvider implements TaskStateReader for testing.
type fakeTaskStateProvider struct {
	states map[string]bool
	ref    Ref
}

func (f *fakeTaskStateProvider) Name() string { return "beads" }

func (f *fakeTaskStateProvider) Push(_ context.Context, item WorkItem, _ *Ref) (Ref, error) {
	return f.ref, nil
}

func (f *fakeTaskStateProvider) Find(_ context.Context, _ string) (*Ref, error) {
	return &f.ref, nil
}

func (f *fakeTaskStateProvider) TaskStates(_ context.Context, _ string, _ *Ref) (map[string]bool, error) {
	return f.states, nil
}

func TestSyncSingleProviderBackwardCompat(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "c1")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# c1\n\nbody\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [ ] first task\n")

	prov := &multiProvider{
		name: "github",
		ref:  Ref{Provider: "github", ID: "42", URL: "https://github.com/o/r/issues/42"},
	}

	// Use the legacy Provider field instead of Providers.
	res, err := Sync(context.Background(), Options{
		OpenSpecDir: root,
		Provider:    prov,
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if res.Created != 1 {
		t.Errorf("want 1 created, got %d", res.Created)
	}

	if len(res.Items) != 1 {
		t.Fatalf("want 1 item, got %d", len(res.Items))
	}

	item := res.Items[0]
	// Single-provider mode still populates Providers for consistency.
	if len(item.Providers) != 1 {
		t.Errorf("want 1 provider result, got %d", len(item.Providers))
	}
}
