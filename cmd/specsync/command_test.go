package main

import (
	"context"
	"flag"
	"strings"
	"testing"
)

// fixtureCmd is a configurable Command for testing registration rules.
type fixtureCmd struct {
	spec  Spec
	usage string
}

func (f *fixtureCmd) Spec() Spec { return f.spec }
func (f *fixtureCmd) Flags(fs *flag.FlagSet) {
	fs.Bool("quiet", false, f.usage)
}
func (f *fixtureCmd) Run(context.Context, []string) error { return nil }

func completeFixture() *fixtureCmd {
	return &fixtureCmd{
		usage: "suppress output",
		spec: Spec{
			Name:     "fixture",
			Group:    GroupHousekeeping,
			Summary:  "A fixture.",
			Workflow: AgentCommandWorkflow{Position: "any"},
			Examples: []string{"specsync fixture -quiet"},
		},
	}
}

func TestValidateCommandRejectsIncompleteDeclarations(t *testing.T) {
	if err := validateCommand(completeFixture()); err != nil {
		t.Fatalf("complete declaration rejected: %v", err)
	}
	cases := map[string]func(*fixtureCmd){
		"no summary":        func(f *fixtureCmd) { f.spec.Summary = " " },
		"no group":          func(f *fixtureCmd) { f.spec.Group = "" },
		"undeclared group":  func(f *fixtureCmd) { f.spec.Group = "nonsense" },
		"no examples":       func(f *fixtureCmd) { f.spec.Examples = nil },
		"no position":       func(f *fixtureCmd) { f.spec.Workflow.Position = "" },
		"flag without text": func(f *fixtureCmd) { f.usage = "" },
		"unknown required":  func(f *fixtureCmd) { f.spec.Required = []string{"missing"} },
		"bad name":          func(f *fixtureCmd) { f.spec.Name = "Two Words" },
		"arg without text":  func(f *fixtureCmd) { f.spec.Args = []Arg{{Name: "x"}} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := completeFixture()
			mutate(f)
			if err := validateCommand(f); err == nil {
				t.Fatal("expected the declaration to be rejected")
			}
		})
	}
}

func TestRegisterPanicsOnDuplicate(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic registering a command twice")
		}
	}()
	// "note" is registered by its own file's init.
	f := completeFixture()
	f.spec.Name = "note"
	Register(f)
}

// TestEveryExampleParsesAgainstItsFlagSet keeps examples honest: an example
// naming a flag its command does not define fails.
func TestEveryExampleParsesAgainstItsFlagSet(t *testing.T) {
	for _, c := range registeredCommands() {
		s := c.Spec()
		defined := map[string]bool{}
		flagSetFor(c).VisitAll(func(f *flag.Flag) { defined[f.Name] = true })
		for _, ex := range s.Examples {
			toks := strings.Fields(ex)
			for i, tok := range toks {
				if tok == "|" { // a piped example: check the specsync side
					toks = toks[i+1:]
					break
				}
			}
			if len(toks) < 2 || toks[0] != "specsync" {
				t.Errorf("%s: example %q must start with `specsync`", s.Name, ex)
				continue
			}
			if !strings.HasPrefix(toks[1], "-") && toks[1] != s.Name {
				t.Errorf("%s: example %q runs a different command", s.Name, ex)
			}
			for _, tok := range toks[2:] {
				if !strings.HasPrefix(tok, "-") {
					continue
				}
				name := strings.SplitN(strings.TrimLeft(tok, "-"), "=", 2)[0]
				if !defined[name] {
					t.Errorf("%s: example %q uses undefined flag -%s", s.Name, ex, name)
				}
			}
			if strings.HasPrefix(toks[1], "-") {
				name := strings.SplitN(strings.TrimLeft(toks[1], "-"), "=", 2)[0]
				if !defined[name] {
					t.Errorf("%s: example %q uses undefined flag -%s", s.Name, ex, name)
				}
			}
		}
	}
}
