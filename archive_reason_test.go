package specsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type reasonRunner struct {
	calls       [][]string
	closed      bool
	failComment bool
	slug        string
}

func (r *reasonRunner) run(_ context.Context, args ...string) (string, error) {
	r.calls = append(r.calls, args)
	switch {
	case args[0] == "issue" && args[1] == "create":
		return "https://github.com/acme/app/issues/7", nil
	case args[0] == "issue" && args[1] == "list":
		return fmt.Sprintf(`[{"number":7,"url":"https://github.com/acme/app/issues/7","body":%q}]`, marker(r.slug)), nil
	case args[0] == "issue" && args[1] == "view":
		state := "OPEN"
		if r.closed {
			state = "CLOSED"
		}
		return `{"number":7,"state":"` + state + `","labels":[]}`, nil
	case args[0] == "issue" && args[1] == "comment":
		if r.failComment {
			return "", fmt.Errorf("gh: comment failed")
		}
	case args[0] == "issue" && args[1] == "close":
		r.closed = true
	}
	return "", nil
}

func (r *reasonRunner) closeCall() []string { return findCall(r.calls, "issue", "close") }

func reasonChange(t *testing.T, slug, tasks string) (root string) {
	t.Helper()
	root = t.TempDir()
	cdir := filepath.Join(root, "changes", slug)
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Abandoned idea\n\nbody\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), tasks)
	return root
}

func TestGitHubCloseMapsReasonToFlags(t *testing.T) {
	cases := []struct {
		name string
		spec func(*Options)
		want []string
	}{
		{"completed is unchanged", func(o *Options) { o.CloseReason = CloseReasonCompleted }, []string{"issue", "close", "7", "--repo", "acme/app"}},
		{"not planned", func(o *Options) { o.CloseReason = CloseReasonNotPlanned }, []string{"issue", "close", "7", "--repo", "acme/app", "--reason", "not planned"}},
		{"duplicate", func(o *Options) { o.CloseReason = CloseReasonDuplicate; o.DuplicateOf = "5" }, []string{"issue", "close", "7", "--repo", "acme/app", "--reason", "duplicate", "--duplicate-of", "5"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := reasonChange(t, "idea", "- [x] done\n")
			r := &reasonRunner{slug: "idea"}
			opts := Options{OpenSpecDir: root, Provider: NewGitHubProviderFuncWithRepo("acme/app", r.run), Slug: "idea", CloseCompleted: true}
			c.spec(&opts)
			if _, err := Sync(context.Background(), opts); err != nil {
				t.Fatal(err)
			}
			got := r.closeCall()
			if strings.Join(got, " ") != strings.Join(c.want, " ") {
				t.Fatalf("close call = %v, want %v", got, c.want)
			}
		})
	}
}

func TestCloseCommentIsPostedBeforeTheClose(t *testing.T) {
	root := reasonChange(t, "idea", "- [x] done\n")
	r := &reasonRunner{slug: "idea"}
	_, err := Sync(context.Background(), Options{OpenSpecDir: root, Provider: NewGitHubProviderFuncWithRepo("acme/app", r.run), Slug: "idea",
		CloseCompleted: true, CloseReason: CloseReasonNotPlanned, CloseComment: "superseded by the registry work"})
	if err != nil {
		t.Fatal(err)
	}
	ci, xi := -1, -1
	for i, c := range r.calls {
		if len(c) > 1 && c[0] == "issue" && c[1] == "comment" {
			ci = i
			if !strings.Contains(strings.Join(c, " "), "superseded by the registry work") {
				t.Fatalf("comment call = %v", c)
			}
		}
		if len(c) > 1 && c[0] == "issue" && c[1] == "close" {
			xi = i
		}
	}
	if ci < 0 || xi < 0 || ci > xi {
		t.Fatalf("comment must come before close; calls: %v", r.calls)
	}
}

func TestCloseCommentFailureLeavesTheIssueOpen(t *testing.T) {
	root := reasonChange(t, "idea", "- [x] done\n")
	r := &reasonRunner{slug: "idea", failComment: true}
	_, err := Archive(context.Background(), ArchiveOptions{OpenSpecDir: root, Slug: "idea", Provider: NewGitHubProviderFuncWithRepo("acme/app", r.run),
		Retain: RetentionPolicyMove, Reason: CloseReasonNotPlanned, Comment: "why"})
	if err == nil || !strings.Contains(err.Error(), "closing comment") {
		t.Fatalf("archive must report the failed comment, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "changes", "idea")); statErr != nil {
		t.Fatalf("the change must stay in place when the close failed: %v", statErr)
	}
	if r.closeCall() != nil || r.closed {
		t.Fatalf("the issue must stay open when the comment cannot be posted; calls: %v", r.calls)
	}
}

func TestArchiveNotPlannedNeedsNoForceAndClosesAsNotPlanned(t *testing.T) {
	root := reasonChange(t, "idea", "- [ ] never started\n- [ ] also not\n")
	r := &reasonRunner{slug: "idea"}
	res, err := Archive(context.Background(), ArchiveOptions{
		OpenSpecDir: root, Slug: "idea", Provider: NewGitHubProviderFuncWithRepo("acme/app", r.run),
		Retain: RetentionPolicyMove, Reason: CloseReasonNotPlanned,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.UncheckedTasks != 2 {
		t.Fatalf("unchecked = %d", res.UncheckedTasks)
	}
	got := r.closeCall()
	if strings.Join(got, " ") != "issue close 7 --repo acme/app --reason not planned" {
		t.Fatalf("close call = %v", got)
	}
	refs, err := LoadRefs(filepath.Join(root, "changes", "archive", "idea"))
	if err != nil {
		t.Fatal(err)
	}
	_, ref := firstRef(refs)
	if ref.BaseClosed == nil || !*ref.BaseClosed {
		t.Fatalf("the closed-state base must be recorded as today (true), got %v", ref.BaseClosed)
	}
}

func TestArchiveDefaultReasonStillRefusesUncheckedTasks(t *testing.T) {
	root := reasonChange(t, "idea", "- [ ] todo\n")
	r := &reasonRunner{slug: "idea"}
	res, err := Archive(context.Background(), ArchiveOptions{OpenSpecDir: root, Slug: "idea", Provider: NewGitHubProviderFuncWithRepo("acme/app", r.run)})
	if err != nil {
		t.Fatal(err)
	}
	if res.UncheckedTasks != 1 || len(r.calls) != 0 {
		t.Fatalf("default archive must refuse without calling the tracker; calls: %v", r.calls)
	}
}

func TestArchiveReasonValidationWritesNothing(t *testing.T) {
	cases := []struct {
		name string
		opts ArchiveOptions
		want string
	}{
		{"duplicate needs an original", ArchiveOptions{Reason: CloseReasonDuplicate}, "-duplicate-of"},
		{"duplicate-of needs duplicate", ArchiveOptions{Reason: CloseReasonNotPlanned, DuplicateOf: "5"}, "-duplicate-of"},
		{"duplicate-of with default reason", ArchiveOptions{DuplicateOf: "5"}, "-duplicate-of"},
		{"unknown reason", ArchiveOptions{Reason: "wontfix"}, "wontfix"},
		{"bad original", ArchiveOptions{Reason: CloseReasonDuplicate, DuplicateOf: "not a ref"}, "issue number or URL"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := reasonChange(t, "idea", "- [ ] todo\n")
			r := &reasonRunner{slug: "idea"}
			c.opts.OpenSpecDir, c.opts.Slug, c.opts.Provider = root, "idea", NewGitHubProviderFuncWithRepo("acme/app", r.run)
			_, err := Archive(context.Background(), c.opts)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to mention %q", err, c.want)
			}
			if len(r.calls) != 0 {
				t.Fatalf("validation must run before any tracker call; calls: %v", r.calls)
			}
		})
	}
}

func TestArchiveDuplicateAcceptsNumberHashAndURL(t *testing.T) {
	for in, want := range map[string]string{"5": "5", "#5": "5", "https://github.com/acme/app/issues/5": "https://github.com/acme/app/issues/5"} {
		root := reasonChange(t, "idea", "- [ ] todo\n")
		r := &reasonRunner{slug: "idea"}
		if _, err := Archive(context.Background(), ArchiveOptions{OpenSpecDir: root, Slug: "idea", Provider: NewGitHubProviderFuncWithRepo("acme/app", r.run),
			Reason: CloseReasonDuplicate, DuplicateOf: in}); err != nil {
			t.Fatal(err)
		}
		if got := r.closeCall(); got[len(got)-1] != want {
			t.Fatalf("%q: close call = %v", in, got)
		}
	}
}

type noReasonProvider struct{ pruneStubProvider }

func TestArchiveNotesWhenTheProviderHasNoCloseReasons(t *testing.T) {
	root := reasonChange(t, "idea", "- [ ] todo\n")
	p := &noReasonProvider{pruneStubProvider{ref: Ref{Provider: "other", ID: "9"}, closed: true}}
	res, err := Archive(context.Background(), ArchiveOptions{OpenSpecDir: root, Slug: "idea", Provider: p, Retain: RetentionPolicyMove, Reason: CloseReasonNotPlanned})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(res.Plan, "\n"), "no close reasons") {
		t.Fatalf("plan should note the provider closes as completed: %v", res.Plan)
	}
}
