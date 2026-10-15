package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This is the acceptance test for make-commands-discoverable. It reads what
// each visible surface actually shows and compares it, in both directions,
// with the set of commands that exist.
//
// Until the Command registry lands, "exists" means: a run*(args []string)
// function is implemented in this package. Switch registeredNames to the
// registry when it exists (tasks 2.1, 2.4); the surfaces below stay as is.

var (
	camelLower = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	camelAcron = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
)

func kebab(s string) string {
	s = camelAcron.ReplaceAllString(s, "${1}-${2}")
	s = camelLower.ReplaceAllString(s, "${1}-${2}")
	return strings.ToLower(s)
}

// registeredNames returns the commands that exist: every run*(args []string)
// handler in the non-test sources.
func registeredNames(t *testing.T) []string {
	t.Helper()
	var names []string
	for _, path := range nonTestSources(t) {
		f := parseFile(t, path)
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Recv != nil {
				continue
			}
			// A free run* handler is a command that is not registered.
			if !strings.HasPrefix(fn.Name.Name, "run") || !takesArgsSlice(fn) {
				continue
			}
			names = append(names, kebab(strings.TrimPrefix(fn.Name.Name, "run")))
		}
	}
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatal("found no run*(args []string) handlers; the scan is broken")
	}
	return names
}

func nonTestSources(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, p := range paths {
		if !strings.HasSuffix(p, "_test.go") {
			out = append(out, p)
		}
	}
	return out
}

func parseFile(t *testing.T, path string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return f
}

// diff returns the names in want but not got, and in got but not want.
func diff(want, got []string) (missing, extra []string) {
	w, g := map[string]bool{}, map[string]bool{}
	for _, n := range want {
		w[n] = true
	}
	for _, n := range got {
		g[n] = true
	}
	for n := range w {
		if !g[n] {
			missing = append(missing, n)
		}
	}
	for n := range g {
		if !w[n] {
			extra = append(extra, n)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return
}

func namesFromAgentHelpJSON(t *testing.T) []string {
	out := captureStdout(t, func() { renderAgentHelpOverview(true) })
	var v struct {
		Commands []string `json:"commands"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("agent-help -json is not JSON: %v", err)
	}
	return v.Commands
}

var helpLine = regexp.MustCompile(`(?m)^  ([a-z][a-z-]*)\s{2,}\S`)

func namesFromAgentHelpList(t *testing.T) []string {
	out := captureStdout(t, func() { renderAgentHelpOverview(false) })
	var names []string
	for _, m := range helpLine.FindAllStringSubmatch(out, -1) {
		names = append(names, m[1])
	}
	return names
}

var (
	mentionCmd = regexp.MustCompile("`?specsync ([a-z][a-z-]+)")
	headingCmd = regexp.MustCompile("(?m)^#{2,4} `([a-z][a-z-]+)`")
)

// namesFromProse lists commands a prose file mentions as `specsync <name>` or
// as a `### name` heading. Prose also says "specsync is" and the like, so only
// the missing direction is checked until the file has a generated, marked
// command section (task 3.3).
func namesFromProse(path string) func(*testing.T) []string {
	return func(t *testing.T) []string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var names []string
		for _, m := range mentionCmd.FindAllStringSubmatch(string(b), -1) {
			names = append(names, m[1])
		}
		for _, m := range headingCmd.FindAllStringSubmatch(string(b), -1) {
			names = append(names, m[1])
		}
		return names
	}
}

// namesThatDispatch returns the registered names that resolve to themselves
// and have a case in main's dispatch switch.
func namesThatDispatch(want []string) func(*testing.T) []string {
	return func(t *testing.T) []string {
		cases := map[string]bool{}
		for _, path := range nonTestSources(t) {
			ast.Inspect(parseFile(t, path), func(n ast.Node) bool {
				if sw, ok := n.(*ast.SwitchStmt); ok {
					for _, s := range switchStrings(sw) {
						cases[s] = true
					}
				}
				return true
			})
		}
		var got []string
		for _, name := range want {
			cmd, _, err := resolveSubcommand([]string{name})
			_, registered := lookupCommand(name)
			if err == nil && cmd == name && (registered || cases[name]) {
				got = append(got, name)
			}
		}
		return got
	}
}

func switchStrings(sw *ast.SwitchStmt) []string {
	var out []string
	for _, stmt := range sw.Body.List {
		cc, ok := stmt.(*ast.CaseClause)
		if !ok {
			continue
		}
		for _, e := range cc.List {
			if s, ok := stringLit(e); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

func stringLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

// handKeptLists reports every map, slice or switch outside a command
// declaration that holds three or more registered command names.
func handKeptLists(want []string) func(*testing.T) []string {
	return func(t *testing.T) []string {
		is := map[string]bool{}
		for _, n := range want {
			is[n] = true
		}
		var found []string
		for _, path := range nonTestSources(t) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			report := func(pos token.Pos, what string, strs []string) {
				seen := map[string]bool{}
				for _, s := range strs {
					if is[s] {
						seen[s] = true
					}
				}
				if len(seen) >= 3 {
					found = append(found, fset.Position(pos).String()+" ("+what+")")
				}
			}
			ast.Inspect(f, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.FuncDecl:
					// A command's Spec declares its own name and related commands.
					return !(n.Recv != nil && n.Name.Name == "Spec")
				case *ast.CompositeLit:
					var strs []string
					for _, el := range n.Elts {
						if kv, ok := el.(*ast.KeyValueExpr); ok {
							el = kv.Key
						}
						if s, ok := stringLit(el); ok {
							strs = append(strs, s)
						}
					}
					report(n.Pos(), "map or slice", strs)
				case *ast.SwitchStmt:
					report(n.Pos(), "switch", switchStrings(n))
				}
				return true
			})
		}
		return found
	}
}

func TestEverySurfaceListsExactlyTheRegisteredCommands(t *testing.T) {
	want := registeredNames(t)

	surfaces := []struct {
		name       string
		got        func(*testing.T) []string
		checkExtra bool
	}{
		{"agent-help list", namesFromAgentHelpList, true},
		{"agent-help -json", namesFromAgentHelpJSON, true},
		{"README", namesFromProse("../../README.md"), false},
		{"skill (canonical)", namesFromProse("../../skills/specsync/SKILL.md"), false},
		{"skill (cmd/specsync copy)", namesFromProse("SKILL.md"), false},
		{"skill (npm copy)", namesFromProse("../../npm/skills/specsync/SKILL.md"), false},
		{"skill (.claude copy)", namesFromProse("../../.claude/skills/specsync/SKILL.md"), false},
		{"dispatch", namesThatDispatch(want), true},
	}
	for _, s := range surfaces {
		t.Run(s.name, func(t *testing.T) {
			missing, extra := diff(want, s.got(t))
			if !s.checkExtra {
				extra = nil
			}
			if len(missing)+len(extra) > 0 {
				t.Fatalf("%s is out of step\n  missing: %v\n  not a command: %v\n"+
					"fix: declare the command once (Spec) and run `go generate`",
					s.name, missing, extra)
			}
		})
	}

	t.Run("source: no second list of commands", func(t *testing.T) {
		if lists := handKeptLists(want)(t); len(lists) > 0 {
			t.Fatalf("hand-kept lists of command names found:\n  %s\n"+
				"fix: derive them from the registry",
				strings.Join(lists, "\n  "))
		}
	})
}
