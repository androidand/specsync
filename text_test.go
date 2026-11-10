package specsync

import (
	"slices"
	"strings"
	"testing"
)

func TestTextDefaults(t *testing.T) {
	w, issues := ParseWays(nil)
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	d := w.Text
	if d.Profile != "conventional" || d.PRTitle != 72 || d.IssueTitle != 60 || d.HeaderWords != 3 || d.MaxHeaders != 7 ||
		d.SentenceWords != 20 || d.ParagraphLines != 4 || d.IssueWords != 150 || d.PRWords != 200 {
		t.Fatalf("%+v", d)
	}
	for _, h := range []string{"Release note", "Review focus", "Design notes", "Discoveries", "Original ask", "Tasks", "Plan changes", "Related"} {
		if !slices.Contains(d.Headers, h) {
			t.Errorf("default headers lack %q", h)
		}
	}
}

func TestTextSectionOverridesAndExtends(t *testing.T) {
	w, issues := ParseWays([]byte("text:\n  profile: plain\n  pr_title: 50\n  pr_words: 0\n  headers: [Non-goals, Impact]\n  filler: [leverage]\n  template: docs/pr.md\n"))
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	d := w.Text
	if d.Profile != "plain" || d.PRTitle != 50 || d.PRWords != 0 || d.Template != "docs/pr.md" {
		t.Fatalf("%+v", d)
	}
	if !slices.Contains(d.Headers, "Non-goals") || !slices.Contains(d.Headers, "Why") {
		t.Fatalf("headers extend the default set: %v", d.Headers)
	}
	if !slices.Contains(d.Filler, "leverage") || !slices.Contains(d.Filler, "basically") {
		t.Fatalf("filler extends the default list: %v", d.Filler)
	}
}

func TestTextSectionReportsBadValues(t *testing.T) {
	_, issues := ParseWays([]byte("text:\n  profile: strict\n  pr_title: many\n  headers: Why\n  sentence_wrods: 10\n"))
	var msgs []string
	for _, i := range issues {
		msgs = append(msgs, i.Msg)
	}
	all := strings.Join(msgs, "|")
	for _, want := range []string{"profile must be", "pr_title must be a whole number", "headers must be a list", "unknown key \"sentence_wrods\""} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in %v", want, msgs)
		}
	}
}

func TestPlainProfileDoesNotMakeTextProfileReserved(t *testing.T) {
	w, _ := ParseWays([]byte("text: {profile: plain}\n"))
	for _, r := range w.Reserved {
		if r.Key == "text.profile" {
			t.Fatal("text.profile is in use now")
		}
	}
}

func TestTextIssuesAreInAFixedOrder(t *testing.T) {
	cfg := []byte("text:\n  headers: nope\n  filler: nope\n")
	first := ""
	for i := 0; i < 20; i++ {
		_, issues := ParseWays(cfg)
		var keys []string
		for _, is := range issues {
			keys = append(keys, is.Key)
		}
		got := strings.Join(keys, ",")
		if i == 0 {
			first = got
		}
		if got != first {
			t.Fatalf("issue order changed: %q vs %q", got, first)
		}
	}
}
