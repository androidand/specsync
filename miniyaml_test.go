package specsync

import (
	"reflect"
	"strings"
	"testing"
)

func mustParse(t *testing.T, src string) *YNode {
	t.Helper()
	n, err := ParseConfigYAML([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, src)
	}
	return n
}

func TestParseConfigYAMLTopLevelKeys(t *testing.T) {
	n := mustParse(t, "# comment\nboard: acme/7\n\nideas_repo: \"acme/ideas\"  # trailing\n")
	if got := n.Str("board"); got != "acme/7" {
		t.Fatalf("board = %q", got)
	}
	if got := n.Str("ideas_repo"); got != "acme/ideas" {
		t.Fatalf("ideas_repo = %q", got)
	}
}

func TestParseConfigYAMLNestedMapsAndLists(t *testing.T) {
	n := mustParse(t, `flow: pr-review
gates:
  - name: lint
    run: make lint
  - {name: test, run: "make test, fast", optional: true}
worktrees:
  dir: ../worktrees
  branch: "{type}/{issue}-{slug}"
tags: [a, 'b c', "d"]
`)
	gates := n.Get("gates")
	if gates == nil || len(gates.List) != 2 {
		t.Fatalf("gates = %+v", gates)
	}
	if gates.List[0].Str("run") != "make lint" || gates.List[1].Str("run") != "make test, fast" || gates.List[1].Str("optional") != "true" {
		t.Fatalf("gates = %+v", gates.List)
	}
	if got := n.Get("worktrees").Str("branch"); got != "{type}/{issue}-{slug}" {
		t.Fatalf("branch = %q", got)
	}
	var tags []string
	for _, e := range n.Get("tags").List {
		tags = append(tags, e.Scalar)
	}
	if !reflect.DeepEqual(tags, []string{"a", "b c", "d"}) {
		t.Fatalf("tags = %v", tags)
	}
}

func TestParseConfigYAMLListAtKeyIndent(t *testing.T) {
	n := mustParse(t, "steps:\n- new\n- build\n")
	if l := n.Get("steps"); l == nil || len(l.List) != 2 || l.List[1].Scalar != "build" {
		t.Fatalf("steps = %+v", l)
	}
}

func TestParseConfigYAMLCommentCharInsideQuotes(t *testing.T) {
	n := mustParse(t, "run: \"make # not a comment\"\nother: a#b\n")
	if n.Str("run") != "make # not a comment" || n.Str("other") != "a#b" {
		t.Fatalf("run=%q other=%q", n.Str("run"), n.Str("other"))
	}
}

func TestParseConfigYAMLNestedBoardDoesNotLeak(t *testing.T) {
	cfg := parseSpecSyncConfig([]byte("board: acme/1\nworktrees:\n  board: other/9\n"))
	if cfg.Board != "acme/1" {
		t.Fatalf("board = %q", cfg.Board)
	}
	cfg = parseSpecSyncConfig([]byte("worktrees:\n  board: other/9\n"))
	if cfg.Board != "" {
		t.Fatalf("a nested board must not become the top-level one, got %q", cfg.Board)
	}
}

func TestParseConfigYAMLExistingConfigUnchanged(t *testing.T) {
	cfg := parseSpecSyncConfig([]byte("board: acme/3\nideas_repo: acme/ideas\nunknown: x\n"))
	if cfg.Board != "acme/3" || cfg.IdeasRepo != "acme/ideas" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestParseConfigYAMLUnsupportedConstructsReportLines(t *testing.T) {
	cases := []struct{ name, src, line, want string }{
		{"anchor", "a: 1\nb: &x 2\n", "line 2", "anchor"},
		{"alias", "a: *x\n", "line 1", "alias"},
		{"tag", "a: !!str 1\n", "line 1", "tag"},
		{"multiline", "a: |\n  text\n", "line 1", "multi-line"},
		{"nested flow", "a: {b: [1, 2]}\n", "line 1", "nested"},
		{"tab", "a:\n\tb: 1\n", "line 2", "tab"},
		{"duplicate", "a: 1\nb: 2\na: 3\n", "line 3", "duplicate"},
		{"no colon", "a: 1\njust text\n", "line 2", "key: value"},
		{"bad indent", "a:\n    b: 1\n  c: 2\n", "line 3", "indent"},
		{"document marker", "---\na: 1\n", "line 1", "document"},
		{"unterminated quote", "a: \"x\n", "line 1", "quote"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseConfigYAML([]byte(c.src))
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), c.line) || !strings.Contains(strings.ToLower(err.Error()), c.want) {
				t.Fatalf("err = %q, want %q and %q", err, c.line, c.want)
			}
		})
	}
}

func TestParseConfigYAMLUnsupportedFileStillReadsTopLevelBoard(t *testing.T) {
	cfg := parseSpecSyncConfig([]byte("board: acme/5\nflow: &anchor pr-review\n"))
	if cfg.Board != "acme/5" {
		t.Fatalf("an unsupported construct elsewhere must not lose the board, got %q", cfg.Board)
	}
}

func TestYNodeLines(t *testing.T) {
	n := mustParse(t, "a: 1\nb:\n  c: 2\n")
	if n.KeyLine["b"] != 2 || n.Get("b").KeyLine["c"] != 3 {
		t.Fatalf("lines = %v / %v", n.KeyLine, n.Get("b").KeyLine)
	}
}

func TestParseConfigYAMLHashInsideQuotesAfterAnyCharacter(t *testing.T) {
	n := mustParse(t, "a: x=\"p # q\"\nb: go test -run='A#B' ./...\nc: it's fine # note\nd: \"say \\\"# hi\\\"\"\n")
	if got := n.Str("a"); got != `x="p # q"` {
		t.Fatalf("a = %q", got)
	}
	if got := n.Str("b"); got != "go test -run='A#B' ./..." {
		t.Fatalf("b = %q", got)
	}
	if got := n.Str("c"); got != "it's fine" {
		t.Fatalf("an apostrophe with no closing quote is plain text, got %q", got)
	}
	if got := n.Str("d"); got != `say "# hi"` {
		t.Fatalf("d = %q", got)
	}
}

func TestParseConfigYAMLShellLikeBareScalars(t *testing.T) {
	n := mustParse(t, "a: [[ -f x ]] && make\nb: *.sh\nc: > out.txt\nd: !important\ne: & wait\nf: {a} text\ng: | tee log\n")
	want := map[string]string{
		"a": "[[ -f x ]] && make", "b": "*.sh", "c": "> out.txt",
		"d": "!important", "e": "& wait", "f": "{a} text", "g": "| tee log",
	}
	for k, v := range want {
		if got := n.Str(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	l := mustParse(t, "gates:\n  - run: [[ -f x ]] && make\n  - [ -f y ] && make\n")
	if l.Get("gates").List[0].Str("run") != "[[ -f x ]] && make" || l.Get("gates").List[1].Scalar != "[ -f y ] && make" {
		t.Fatalf("%+v", l.Get("gates").List)
	}
}

func TestParseConfigYAMLRealCollectionsAndConstructsStillWork(t *testing.T) {
	n := mustParse(t, "a: [x, y]\nb: {k: v}\n")
	if len(n.Get("a").List) != 2 || n.Get("b").Str("k") != "v" {
		t.Fatalf("%+v", n)
	}
	for _, src := range []string{"a: &x\n", "a: *x\n", "a: !!str\n", "a: |\n", "a: >-\n"} {
		if _, err := ParseConfigYAML([]byte(src)); err == nil {
			t.Errorf("%q must still be rejected", src)
		}
	}
}

func TestParseConfigYAMLStripsByteOrderMark(t *testing.T) {
	n := mustParse(t, "\ufeffboard: acme/1\n")
	if n.Str("board") != "acme/1" {
		t.Fatalf("%+v", n.Keys)
	}
	if cfg := parseSpecSyncConfig([]byte("\ufeffboard: acme/2\n")); cfg.Board != "acme/2" {
		t.Fatalf("board = %q", cfg.Board)
	}
}
