package main

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// A Command is everything specsync knows about one subcommand: its
// documentation, its flags and the code that runs it. Dispatch, help, the
// README table and the skill are all derived from registered Commands, so a
// command cannot exist without being described.
type Command interface {
	// Spec is the command's identity and documentation.
	Spec() Spec
	// Flags defines the command's flags on fs and binds them to the receiver.
	Flags(fs *flag.FlagSet)
	// Run does the work. args are the arguments after the subcommand name.
	Run(ctx context.Context, args []string) error
}

// Group is the intent a command serves. A typo does not compile.
type Group string

const (
	GroupOverview     Group = "overview"
	GroupPlan         Group = "plan"
	GroupSync         Group = "sync"
	GroupReview       Group = "review"
	GroupHousekeeping Group = "housekeeping"
)

// groupInfo orders the groups and says where to start in each.
var groupInfo = []struct {
	Group Group
	Title string
	Start string
}{
	{GroupOverview, "See what is going on", "specsync changes"},
	{GroupPlan, "Plan work", "specsync pull -issue <n>"},
	{GroupSync, "Sync with the tracker", "specsync -dry-run -change <slug>"},
	{GroupReview, "Review and release", "specsync verify"},
	{GroupHousekeeping, "Housekeeping", "specsync doctor"},
}

// Arg is a positional argument.
type Arg struct {
	Name        string
	Description string
	Required    bool
}

// Spec describes a command.
type Spec struct {
	Name     string
	Group    Group
	Summary  string   // one line
	Intents  []string // phrases a person would use to look for it
	Mutates  bool
	Workflow AgentCommandWorkflow
	Args     []Arg    // positional arguments, in order
	Required []string // names of flags that must be given
	Safety   []string
	Examples []string // full command lines
	// Confusions maps a habit word (e.g. git's "push") to the note shown when
	// someone types it and this command is what they meant. It is not an
	// alias: the word still fails to dispatch.
	Confusions map[string]string
	JSONOutput map[string]interface{}
}

var registry = map[string]Command{}

// Register adds c to the registry. It panics on an incomplete declaration, so
// an undocumented command cannot start. Call it from init() in the command's
// own file.
func Register(c Command) {
	if err := validateCommand(c); err != nil {
		panic("specsync: " + err.Error())
	}
	name := c.Spec().Name
	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("specsync: command %q registered twice", name))
	}
	registry[name] = c
}

// lookupCommand returns the registered command with this name.
func lookupCommand(name string) (Command, bool) {
	c, ok := registry[name]
	return c, ok
}

// registeredCommands returns every command ordered by group, then name.
func registeredCommands() []Command {
	order := map[Group]int{}
	for i, g := range groupInfo {
		order[g.Group] = i
	}
	cmds := make([]Command, 0, len(registry))
	for _, c := range registry {
		cmds = append(cmds, c)
	}
	sort.Slice(cmds, func(i, j int) bool {
		a, b := cmds[i].Spec(), cmds[j].Spec()
		if order[a.Group] != order[b.Group] {
			return order[a.Group] < order[b.Group]
		}
		return a.Name < b.Name
	})
	return cmds
}

// flagSetFor builds a fresh flag set for c with its flags defined.
func flagSetFor(c Command) *flag.FlagSet {
	fs := flag.NewFlagSet(c.Spec().Name, flag.ExitOnError)
	c.Flags(fs)
	return fs
}

func validateCommand(c Command) error {
	s := c.Spec()
	who := fmt.Sprintf("command %q", s.Name)
	if s.Name == "" || s.Name != strings.ToLower(s.Name) || strings.ContainsAny(s.Name, " \t") {
		return fmt.Errorf("command name %q must be non-empty lowercase without spaces", s.Name)
	}
	if strings.TrimSpace(s.Summary) == "" {
		return fmt.Errorf("%s has no summary", who)
	}
	known := false
	for _, g := range groupInfo {
		known = known || g.Group == s.Group
	}
	if !known {
		return fmt.Errorf("%s has no group or an undeclared group %q", who, s.Group)
	}
	if strings.TrimSpace(s.Workflow.Position) == "" {
		return fmt.Errorf("%s has no workflow position", who)
	}
	if len(s.Examples) == 0 {
		return fmt.Errorf("%s has no examples", who)
	}
	for _, a := range s.Args {
		if a.Name == "" || strings.TrimSpace(a.Description) == "" {
			return fmt.Errorf("%s: argument %q needs a name and a description", who, a.Name)
		}
	}
	fs := flag.NewFlagSet(s.Name, flag.ContinueOnError)
	c.Flags(fs)
	have := map[string]bool{}
	var err error
	fs.VisitAll(func(f *flag.Flag) {
		have[f.Name] = true
		if strings.TrimSpace(f.Usage) == "" && err == nil {
			err = fmt.Errorf("%s: flag -%s has no usage text", who, f.Name)
		}
	})
	if err != nil {
		return err
	}
	for _, r := range s.Required {
		if !have[r] {
			return fmt.Errorf("%s: required flag -%s is not defined", who, r)
		}
	}
	return nil
}

// flagType names a flag's type the way agent-help reports it.
func flagType(f *flag.Flag) string {
	switch fmt.Sprintf("%T", f.Value) {
	case "*flag.boolValue":
		return "boolean"
	case "*flag.intValue", "*flag.int64Value", "*flag.uintValue", "*flag.uint64Value", "*flag.float64Value":
		return "number"
	}
	return "string"
}

// helpFromCommand derives the agent-help entry from the command's own
// declaration and flag set, so each flag is described exactly once.
func helpFromCommand(c Command) AgentCommandHelp {
	s := c.Spec()
	required := map[string]bool{}
	for _, r := range s.Required {
		required[r] = true
	}
	flags := []AgentCommandFlag{}
	flagSetFor(c).VisitAll(func(f *flag.Flag) {
		af := AgentCommandFlag{Name: f.Name, Type: flagType(f), Required: required[f.Name], Description: f.Usage}
		switch af.Type {
		case "boolean":
			af.Default = f.DefValue == "true"
		case "number":
			if n, err := strconv.ParseFloat(f.DefValue, 64); err == nil {
				af.Default = n
			}
		default:
			af.Default = f.DefValue
		}
		flags = append(flags, af)
	})
	var args []AgentCommandArg
	for _, a := range s.Args {
		args = append(args, AgentCommandArg(a))
	}
	return AgentCommandHelp{
		Command:     s.Name,
		Args:        args,
		Description: s.Summary,
		Mutates:     s.Mutates,
		Workflow:    s.Workflow,
		Flags:       flags,
		SafetyRules: s.Safety,
		JSONOutput:  s.JSONOutput,
		Examples:    s.Examples,
	}
}

// confusionFor returns the command a habit word is confused with, and the
// note explaining why the word is not an alias, from the declarations.
func confusionFor(word string) (command, note string, ok bool) {
	for _, cmd := range registeredCommands() {
		if note, ok := cmd.Spec().Confusions[word]; ok {
			return cmd.Spec().Name, note, true
		}
	}
	return "", "", false
}

// commandNames returns every registered command name, ordered by group.
func commandNames() []string {
	var names []string
	for _, c := range registeredCommands() {
		names = append(names, c.Spec().Name)
	}
	return names
}
