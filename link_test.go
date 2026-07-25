package specsync

import (
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
