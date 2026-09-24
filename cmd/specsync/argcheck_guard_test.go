package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// subcommandsExemptFromArgCheck lists run* functions taking `args []string`
// that intentionally do not call checkArgs, with the reason it's safe —
// keep this list short and every entry justified, since it's the one place
// a future subcommand could quietly opt back out of the check.
var subcommandsExemptFromArgCheck = map[string]string{
	"runDoctor":    "reorders flags first (reorderFlagsFirst) and errors loudly on any unrecognized subcommand word — the silent-drop failure mode doesn't apply",
	"runAgentHelp": "reorders flags first (reorderFlagsFirst) and errors loudly on any unrecognized subcommand word — the silent-drop failure mode doesn't apply",
	"runRelate":    "dead code: not dispatched from main() or knownSubcommands",
}

// TestEveryRunFuncChecksArgs guards against the exact way this incident's
// defect could silently regress: a future subcommand added to cmd/specsync
// that parses a flag.FlagSet but never calls checkArgs after fs.Parse.
// Hand-adding checkArgs at each call site (rather than one central dispatch
// point) means nothing else stops that omission — this test is that stop.
func TestEveryRunFuncChecksArgs(t *testing.T) {
	fset := token.NewFileSet()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}

	checked := 0
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Recv != nil {
				continue
			}
			name := fn.Name.Name
			if !strings.HasPrefix(name, "run") || !takesArgsSlice(fn) {
				continue
			}
			if _, exempt := subcommandsExemptFromArgCheck[name]; exempt {
				continue
			}
			checked++
			if !callsCheckArgs(fn.Body) {
				t.Errorf("%s (%s) parses a flag.FlagSet from an `args []string` parameter but never calls checkArgs after fs.Parse — call it, or add %q to subcommandsExemptFromArgCheck with a reason", name, path, name)
			}
		}
	}
	if checked == 0 {
		t.Fatal("scan found no run*(args []string) functions — the scan itself is broken")
	}
}

// takesArgsSlice reports whether fn has exactly one parameter of type
// []string — the signature every dispatched subcommand handler uses.
func takesArgsSlice(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
		return false
	}
	arr, ok := fn.Type.Params.List[0].Type.(*ast.ArrayType)
	if !ok || arr.Len != nil {
		return false
	}
	ident, ok := arr.Elt.(*ast.Ident)
	return ok && ident.Name == "string"
}

// callsCheckArgs reports whether body contains a call to checkArgs anywhere.
func callsCheckArgs(body ast.Node) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "checkArgs" {
				found = true
			}
		}
		return true
	})
	return found
}
