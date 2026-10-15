package main

import (
	"fmt"
	"strings"
)

//go:generate go test . -run TestGeneratedDocsAreCurrent -update

const (
	commandsStart = "<!-- specsync:commands:start -->"
	commandsEnd   = "<!-- specsync:commands:end -->"
)

// renderCommandTable renders the commands as one table per group, for the README.
func renderCommandTable() string {
	var b strings.Builder
	for _, g := range groupInfo {
		rows := commandsIn(g.Group)
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(&b, "**%s.** Start with `%s`.\n\n| Command | What it does |\n|---|---|\n", g.Title, g.Start)
		for _, s := range rows {
			fmt.Fprintf(&b, "| `%s` | %s |\n", s.Name, s.Summary)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// renderCommandList renders the commands as a compact list, for the skill.
func renderCommandList() string {
	var b strings.Builder
	for _, g := range groupInfo {
		rows := commandsIn(g.Group)
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(&b, "**%s** (start: `%s`)\n", g.Title, g.Start)
		for _, s := range rows {
			fmt.Fprintf(&b, "- `%s` — %s\n", s.Name, s.Summary)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func commandsIn(g Group) []Spec {
	var out []Spec
	for _, c := range registeredCommands() {
		if s := c.Spec(); s.Group == g {
			out = append(out, s)
		}
	}
	return out
}

// spliceCommands replaces the text between the command markers in doc.
func spliceCommands(doc, body string) (string, error) {
	a := strings.Index(doc, commandsStart)
	b := strings.Index(doc, commandsEnd)
	if a < 0 || b < a {
		return "", fmt.Errorf("missing %s ... %s markers", commandsStart, commandsEnd)
	}
	return doc[:a+len(commandsStart)] + "\n" + body + doc[b:], nil
}
