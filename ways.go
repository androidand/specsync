package specsync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Gate is a named command from the repository's config.
type Gate struct {
	Name     string
	Run      string
	Optional bool
}

type WorktreeConfig struct{ Dir, Branch string }
type CleanupConfig struct {
	RemoteBranch string
	Archive      bool
	MinAgeDays   int
	Hooks        []string
}

// Ways is the repository's declared way of working, read from
// openspec/specsync.yml with defaults applied.
type Ways struct {
	Flow      string   // preset name; "" when the steps were listed
	Steps     []string // effective steps, in order
	Gates     []Gate
	Worktrees WorktreeConfig
	Text      TextConfig
	Cleanup   CleanupConfig
	Sync      SyncConfig
	// Reserved lists keys that are accepted but not used yet.
	Reserved []ConfigIssue
}

// ConfigIssue is one problem found in the config, with its line.
type ConfigIssue struct {
	Line int    `json:"line"`
	Key  string `json:"key,omitempty"`
	Msg  string `json:"message"`
}

func (i ConfigIssue) String() string { return fmt.Sprintf("line %d: %s", i.Line, i.Msg) }

const DefaultFlow = "pr-review"

var flowSteps = []string{"new", "start", "build", "gate", "review", "merge", "cleanup"}

var presets = map[string][]string{
	"minimal":   {"new", "build", "gate"},
	"pr-review": {"new", "start", "build", "gate", "review", "merge", "cleanup"},
	"strict":    {"new", "start", "build", "gate", "review", "merge", "cleanup"},
}

func presetNames() string { return "minimal, pr-review, strict" }

var topLevelKeys = []string{"board", "ideas_repo", "flow", "gates", "worktrees", "text", "cleanup", "sync"}

var sectionKeys = map[string][]string{
	"worktrees": {"dir", "branch"},
	"text":      append([]string{"profile", "headers", "filler", "template"}, textIntKeys...),
	"cleanup":   {"remote_branch", "archive", "min_age_days", "hooks"},
	"sync":      {"strategy", "after_review", "force"},
}

// LoadWays reads the config under repoRoot. A missing file is the defaults.
func LoadWays(repoRoot string) (Ways, []ConfigIssue) {
	data, err := os.ReadFile(filepath.Join(repoRoot, SpecSyncConfigPath))
	if err != nil {
		return ParseWays(nil)
	}
	return ParseWays(data)
}

// ParseWays parses the config text, applying defaults and reporting every
// problem it can find with its line.
func ParseWays(data []byte) (Ways, []ConfigIssue) {
	w := Ways{
		Flow:    DefaultFlow,
		Steps:   presets[DefaultFlow],
		Sync:    SyncConfig{Strategy: "rebase", AfterReview: "merge", Force: "with-lease"},
		Text:    DefaultText(),
		Cleanup: CleanupConfig{RemoteBranch: "keep"},
	}
	root, err := ParseConfigYAML(data)
	if err != nil {
		var ye *YAMLError
		if errors.As(err, &ye) {
			return w, []ConfigIssue{{Line: ye.Line, Msg: ye.Msg}}
		}
		return w, []ConfigIssue{{Line: 1, Msg: err.Error()}}
	}
	if root.Kind != YMap {
		return w, []ConfigIssue{{Line: root.Line, Msg: "the config must be a map of keys (key: value)"}}
	}
	var issues []ConfigIssue
	add := func(line int, key, format string, a ...any) {
		issues = append(issues, ConfigIssue{Line: line, Key: key, Msg: fmt.Sprintf(format, a...)})
	}
	for _, k := range root.Keys {
		if !slices.Contains(topLevelKeys, k) {
			add(root.KeyLine[k], k, "unknown key %q%s", k, didYouMean(k, topLevelKeys))
		}
	}
	if n := root.Get("flow"); n != nil {
		w.readFlow(n, root.KeyLine["flow"], add)
	}
	if n := root.Get("gates"); n != nil {
		w.readGates(n, root.KeyLine["gates"], add)
	}
	for _, sec := range []string{"worktrees", "text", "cleanup", "sync"} {
		n := root.Get(sec)
		if n == nil {
			continue
		}
		if n.Kind != YMap {
			add(root.KeyLine[sec], sec, "%s must be a map", sec)
			continue
		}
		for _, k := range n.Keys {
			if !slices.Contains(sectionKeys[sec], k) {
				add(n.KeyLine[k], sec+"."+k, "unknown key %q in %s%s", k, sec, didYouMean(k, sectionKeys[sec]))
			}
		}
		w.readSection(sec, n, add)
	}
	sort.SliceStable(issues, func(i, j int) bool { return issues[i].Line < issues[j].Line })
	return w, issues
}

type addIssue func(line int, key, format string, a ...any)

func (w *Ways) readFlow(n *YNode, line int, add addIssue) {
	switch n.Kind {
	case YScalar:
		steps, ok := presets[n.Scalar]
		if !ok {
			add(line, "flow", "unknown preset %q: use one of %s", n.Scalar, presetNames())
			return
		}
		w.Flow, w.Steps = n.Scalar, steps
	case YList:
		if len(n.List) == 0 {
			add(line, "flow", "flow needs at least one step, or use a preset: %s", presetNames())
			return
		}
		var steps []string
		for _, s := range n.List {
			if s.Kind != YScalar || !slices.Contains(flowSteps, s.Scalar) {
				add(line, "flow", "unknown step %q: use steps from %s", s.Scalar, strings.Join(flowSteps, ", "))
				return
			}
			steps = append(steps, s.Scalar)
		}
		w.Flow, w.Steps = "", steps
	default:
		add(line, "flow", "flow must be a preset name or a list of steps")
	}
}

func (w *Ways) readGates(n *YNode, line int, add addIssue) {
	if n.Kind != YList {
		add(line, "gates", "gates must be a list of {name, run} entries")
		return
	}
	for _, g := range n.List {
		if g.Kind != YMap {
			add(g.Line, "gates", "a gate must be a map with name and run")
			continue
		}
		for _, k := range g.Keys {
			if !slices.Contains([]string{"name", "run", "optional"}, k) {
				add(g.KeyLine[k], "gates."+k, "unknown key %q in a gate%s", k, didYouMean(k, []string{"name", "run", "optional"}))
			}
		}
		gate := Gate{Name: g.Str("name"), Run: g.Str("run")}
		switch {
		case gate.Name == "":
			add(g.Line, "gates", "a gate has no name")
		case gate.Run == "":
			add(g.Line, "gates", "gate %q has no run command", gate.Name)
		}
		if o := g.Get("optional"); o != nil {
			b, ok := parseBool(o.Scalar)
			if !ok {
				add(g.KeyLine["optional"], "gates.optional", "optional must be true or false, got %q", o.Scalar)
			}
			gate.Optional = b
		}
		w.Gates = append(w.Gates, gate)
	}
}

var reservedKeys = map[string]string{
	"worktrees.dir":    "flow-verbs",
	"worktrees.branch": "flow-verbs",
}

func (w *Ways) readSection(sec string, n *YNode, add addIssue) {
	for _, k := range n.Keys {
		if owner, ok := reservedKeys[sec+"."+k]; ok {
			w.Reserved = append(w.Reserved, ConfigIssue{Line: n.KeyLine[k], Key: sec + "." + k,
				Msg: fmt.Sprintf("%s.%s is reserved for %s and is not used yet", sec, k, owner)})
		}
	}
	enum := func(key string, allowed ...string) string {
		v := n.Str(key)
		if v != "" && !slices.Contains(allowed, v) {
			add(n.KeyLine[key], sec+"."+key, "%s must be %s, got %q", key, strings.Join(allowed, " or "), v)
			return ""
		}
		return v
	}
	boolean := func(key string) bool {
		v := n.Str(key)
		b, ok := parseBool(v)
		if v != "" && !ok {
			add(n.KeyLine[key], sec+"."+key, "%s must be true or false, got %q", key, v)
		}
		return b
	}
	switch sec {
	case "worktrees":
		w.Worktrees = WorktreeConfig{Dir: n.Str("dir"), Branch: n.Str("branch")}
	case "text":
		w.Text.read(n, add)
	case "cleanup":
		if v := enum("remote_branch", "keep", "delete"); v != "" {
			w.Cleanup.RemoteBranch = v
		}
		if n.Get("archive") != nil {
			w.Cleanup.Archive = boolean("archive")
		}
		if v := n.Str("min_age_days"); v != "" {
			days, err := strconv.Atoi(v)
			if err != nil || days < 0 {
				add(n.KeyLine["min_age_days"], "cleanup.min_age_days", "min_age_days must be a whole number of days, got %q", v)
			} else {
				w.Cleanup.MinAgeDays = days
			}
		}
		if h := n.Get("hooks"); h != nil {
			if h.Kind != YList {
				add(n.KeyLine["hooks"], "cleanup.hooks", "hooks must be a list of commands")
			}
			for _, item := range h.List {
				if item.Kind != YScalar || strings.TrimSpace(item.Scalar) == "" {
					add(item.Line, "cleanup.hooks", "each hook must be a command")
					continue
				}
				w.Cleanup.Hooks = append(w.Cleanup.Hooks, item.Scalar)
			}
		}
	case "sync":
		if v := enum("strategy", "rebase", "merge"); v != "" {
			w.Sync.Strategy = v
		}
		if v := enum("after_review", "merge", "rebase"); v != "" {
			w.Sync.AfterReview = v
		}
		if v := enum("force", "with-lease", "never"); v != "" {
			w.Sync.Force = v
		}
	}
}

// TrustedCommands is every command the config can make specsync run: the
// gates, including optional ones, then the cleanup hooks. Trusting a config
// trusts all of them.
func (w Ways) TrustedCommands() []string {
	return append(w.GateCommands(true, nil), w.Cleanup.Hooks...)
}

// GateCommands returns the commands to run, in order. Optional gates are
// included only when includeOptional is set or they are named in only.
func (w Ways) GateCommands(includeOptional bool, only []string) []string {
	var out []string
	for _, g := range w.Gates {
		switch {
		case len(only) > 0:
			if !slices.Contains(only, g.Name) {
				continue
			}
		case g.Optional && !includeOptional:
			continue
		}
		out = append(out, g.Run)
	}
	return out
}

func parseBool(s string) (bool, bool) {
	switch strings.ToLower(s) {
	case "true":
		return true, true
	case "false", "":
		return false, true
	}
	return false, false
}

func didYouMean(key string, known []string) string {
	best, bestD := "", 3
	for _, k := range known {
		if d := editDistance(key, k); d < bestD {
			best, bestD = k, d
		}
	}
	if best == "" {
		return ""
	}
	return fmt.Sprintf(`; did you mean "%s"?`, best)
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}
