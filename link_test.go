package specsync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestUpsertRelatedSection(t *testing.T) {
	links := []Ref{
		{Provider: "github:owner/repo", ID: "42", URL: "https://github.com/owner/repo/issues/42"},
	}

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "append when absent",
			body: "Hello world",
			want: "Hello world\n\n## Related\n\n- [owner/repo#42](https://github.com/owner/repo/issues/42)\n",
		},
		{
			name: "replace existing block",
			body: "Hello\n\n## Related\n\n- [old#1](https://github.com/owner/repo/issues/1)\n\n## Other\n\nstuff",
			want: "Hello\n\n## Related\n\n- [owner/repo#42](https://github.com/owner/repo/issues/42)\n\n## Other\n\nstuff",
		},
		{
			name: "replace existing block at EOF",
			body: "Hello\n\n## Related\n\n- [old#1](https://github.com/owner/repo/issues/1)",
			want: "Hello\n\n## Related\n\n- [owner/repo#42](https://github.com/owner/repo/issues/42)",
		},
		{
			name: "idempotent",
			body: "Hello\n\n## Related\n\n- [owner/repo#42](https://github.com/owner/repo/issues/42)\n",
			want: "Hello\n\n## Related\n\n- [owner/repo#42](https://github.com/owner/repo/issues/42)\n",
		},
		{
			name: "empty body",
			body: "",
			want: "\n\n## Related\n\n- [owner/repo#42](https://github.com/owner/repo/issues/42)\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UpsertRelatedSection(tt.body, links)
			if got != tt.want {
				t.Errorf("UpsertRelatedSection() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUpsertRelatedSection_NoLinks(t *testing.T) {
	body := "Hello\n\n## Related\n\n- old\n"
	got := UpsertRelatedSection(body, nil)
	if got != body {
		t.Errorf("expected no change with nil links, got %q", got)
	}
}

func TestUpsertRelatedSection_MultipleLinks(t *testing.T) {
	links := []Ref{
		{Provider: "github:owner/repo", ID: "1", URL: "https://github.com/owner/repo/issues/1"},
		{Provider: "github:other/repo", ID: "2", URL: "https://github.com/other/repo/issues/2"},
	}
	got := UpsertRelatedSection("Hello", links)
	want := "Hello\n\n## Related\n\n- [owner/repo#1](https://github.com/owner/repo/issues/1)\n- [other/repo#2](https://github.com/other/repo/issues/2)\n"
	if got != want {
		t.Errorf("UpsertRelatedSection() = %q, want %q", got, want)
	}
}

func TestRefsExcept(t *testing.T) {
	all := []Ref{
		{Provider: "github:owner/repo", ID: "1", URL: "https://github.com/owner/repo/issues/1"},
		{Provider: "github:owner/repo", ID: "2", URL: "https://github.com/owner/repo/issues/2"},
	}
	except := all[0]
	got := refsExcept(all, except)
	if len(got) != 1 || got[0].ID != "2" {
		t.Errorf("refsExcept() = %v, want [{ID:2}]", got)
	}
}

func TestClassifyArg(t *testing.T) {
	root := t.TempDir()
	openspecDir := filepath.Join(root, "openspec")
	cdir := filepath.Join(openspecDir, "changes", "my-change")
	os.MkdirAll(cdir, 0o755)
	os.WriteFile(filepath.Join(cdir, "proposal.md"), []byte("# my-change\n\nbody\n"), 0o644)
	os.WriteFile(filepath.Join(cdir, "tasks.md"), []byte("- [ ] task\n"), 0o644)
	os.MkdirAll(filepath.Join(cdir, ".specsync"), 0o755)
	os.WriteFile(filepath.Join(cdir, ".specsync", "refs.json"), []byte(`{"github":{"provider":"github","id":"42","url":"https://github.com/o/r/issues/42"}}`), 0o644)

	tests := []struct {
		name     string
		arg      string
		repo     string
		wantKind linkEntryKind
		wantSlug string
		wantID   string
		wantErr  bool
	}{
		{
			name:     "slug",
			arg:      "my-change",
			repo:     "",
			wantKind: kindSlug,
			wantSlug: "my-change",
			wantID:   "42",
		},
		{
			name:     "bare hash N",
			arg:      "#10",
			repo:     "owner/repo",
			wantKind: kindIssueRef,
			wantID:   "10",
		},
		{
			name:     "bare N without hash",
			arg:      "10",
			repo:     "owner/repo",
			wantKind: kindIssueRef,
			wantID:   "10",
		},
		{
			name:     "bare N no repo errors",
			arg:      "10",
			repo:     "",
			wantKind: kindIssueRef,
			wantErr:  true,
		},
		{
			name:     "owner/repo#N shorthand",
			arg:      "owner/repo#55",
			repo:     "",
			wantKind: kindIssueRef,
			wantID:   "55",
		},
		{
			name:     "full URL",
			arg:      "https://github.com/other/repo/issues/7",
			repo:     "",
			wantKind: kindIssueRef,
			wantID:   "7",
		},
		{
			name:     "unknown slug errors",
			arg:      "nonexistent",
			repo:     "",
			wantKind: kindSlug,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := classifyArg(tt.arg, openspecDir, tt.repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("classifyArg() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.kind != tt.wantKind {
				t.Errorf("kind = %v, want %v", got.kind, tt.wantKind)
			}
			if got.slug != tt.wantSlug {
				t.Errorf("slug = %q, want %q", got.slug, tt.wantSlug)
			}
			if got.ref.ID != tt.wantID {
				t.Errorf("ref.ID = %q, want %q", got.ref.ID, tt.wantID)
			}
		})
	}
}

func TestLink_ReferenceOnly(t *testing.T) {
	root := t.TempDir()
	openspecDir := filepath.Join(root, "openspec")
	os.MkdirAll(openspecDir, 0o755)

	// Link two issue references only (no slugs).
	result, err := Link(context.Background(), LinkOptions{
		OpenSpecDir: openspecDir,
		Args:        []string{"owner/repo#1", "owner/repo#2"},
		Repo:        "owner/repo",
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("Link: %v", err)
	}

	if len(result.Pairs) != 0 {
		t.Errorf("want 0 slug pairs, got %d", len(result.Pairs))
	}
	if len(result.Refs) != 2 {
		t.Fatalf("want 2 refs, got %d", len(result.Refs))
	}

	// First ref should link to second, and vice versa.
	r1 := result.Refs[0]
	if r1.ID != "1" || r1.Repo != "owner/repo" {
		t.Errorf("ref 0 = %+v, want ID=1, Repo=owner/repo", r1)
	}
	r2 := result.Refs[1]
	if r2.ID != "2" || r2.Repo != "owner/repo" {
		t.Errorf("ref 1 = %+v, want ID=2, Repo=owner/repo", r2)
	}
}

func TestLink_ReferenceWithFakeProvider(t *testing.T) {
	root := t.TempDir()
	openspecDir := filepath.Join(root, "openspec")
	os.MkdirAll(openspecDir, 0o755)

	prov := &fakeLinkProvider{
		ref: Ref{Provider: "github:owner/repo", ID: "42", URL: "https://github.com/owner/repo/issues/42"},
		body: "# My Issue\n\nSome content\n\n## Tasks\n\n- [ ] task\n",
	}

	result, err := Link(context.Background(), LinkOptions{
		OpenSpecDir: openspecDir,
		Args:        []string{"owner/repo#42", "owner/repo#99"},
		Repo:        "owner/repo",
		DryRun:      false,
	})
	if err != nil {
		t.Fatalf("Link: %v", err)
	}

	if len(result.Refs) != 2 {
		t.Fatalf("want 2 refs, got %d", len(result.Refs))
	}

	// Fetch issue #42 and upsert Related with #99.
	item, err := prov.Get(context.Background(), "42")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	edited := UpsertRelatedSection(item.Body, []Ref{result.Refs[1].Ref})
	if !containsStr(edited, "## Related") {
		t.Errorf("edited body missing Related section:\n%s", edited)
	}
	if !containsStr(edited, "owner/repo#99") {
		t.Errorf("edited body missing link to #99:\n%s", edited)
	}

	// Push edited body.
	prov.Push(context.Background(), WorkItem{
		Title: item.Title,
		Body:  edited,
	}, &result.Refs[0].Ref)

	if !containsStr(prov.pushed.Body, "## Related") {
		t.Errorf("pushed body missing Related section:\n%s", prov.pushed.Body)
	}

	// Verify no file was written (reference path writes no links.md).
	linksMD := filepath.Join(openspecDir, "changes", "links.md")
	if _, err := os.Stat(linksMD); !os.IsNotExist(err) {
		t.Error("reference path should not write any files")
	}
}

func TestLink_MixedSlugAndReference(t *testing.T) {
	root := t.TempDir()
	openspecDir := filepath.Join(root, "openspec")
	cdir := filepath.Join(openspecDir, "changes", "my-change")
	os.MkdirAll(cdir, 0o755)
	os.WriteFile(filepath.Join(cdir, "proposal.md"), []byte("# my-change\n\nbody\n"), 0o644)
	os.WriteFile(filepath.Join(cdir, "tasks.md"), []byte("- [ ] task\n"), 0o644)
	os.MkdirAll(filepath.Join(cdir, ".specsync"), 0o755)
	os.WriteFile(filepath.Join(cdir, ".specsync", "refs.json"), []byte(`{"github":{"provider":"github","id":"42","url":"https://github.com/o/r/issues/42"}}`), 0o644)

	result, err := Link(context.Background(), LinkOptions{
		OpenSpecDir: openspecDir,
		Args:        []string{"my-change", "owner/repo#99"},
		Repo:        "owner/repo",
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("Link: %v", err)
	}

	// Slug should be in Pairs.
	if len(result.Pairs) != 1 {
		t.Fatalf("want 1 pair, got %d", len(result.Pairs))
	}
	if result.Pairs[0].Slug != "my-change" {
		t.Errorf("want slug my-change, got %q", result.Pairs[0].Slug)
	}

	// Reference should be in Refs.
	if len(result.Refs) != 1 {
		t.Fatalf("want 1 ref, got %d", len(result.Refs))
	}
	if result.Refs[0].ID != "99" {
		t.Errorf("want ref ID 99, got %q", result.Refs[0].ID)
	}

	// Dry-run should not write files.
	if _, err := os.Stat(filepath.Join(cdir, "links.md")); !os.IsNotExist(err) {
		t.Error("dry-run should not write links.md")
	}
}

// fakeLinkProvider implements WorkProvider + IssueReader for link tests.
type fakeLinkProvider struct {
	ref    Ref
	body   string
	pushed WorkItem
}

func (f *fakeLinkProvider) Name() string { return "github" }
func (f *fakeLinkProvider) Push(_ context.Context, item WorkItem, _ *Ref) (Ref, error) {
	f.pushed = item
	return f.ref, nil
}
func (f *fakeLinkProvider) Find(_ context.Context, _ string) (*Ref, error) { return &f.ref, nil }
func (f *fakeLinkProvider) Get(_ context.Context, id string) (FetchedItem, error) {
	return FetchedItem{ID: id, URL: f.ref.URL, Body: f.body}, nil
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsAt(s, sub))
}

func containsAt(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
