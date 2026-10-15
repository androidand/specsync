package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// generatedDocs lists every file that holds a generated command section and
// the renderer that fills it. Run `go generate ./cmd/specsync` to refresh them.
var generatedDocs = []struct {
	path   string
	render func() string
}{
	{"../../README.md", renderCommandTable},
	{"../../skills/specsync/SKILL.md", renderCommandList},
	{"SKILL.md", renderCommandList},
	{"../../npm/skills/specsync/SKILL.md", renderCommandList},
	{"../../.claude/skills/specsync/SKILL.md", renderCommandList},
}

func TestGeneratedDocsAreCurrent(t *testing.T) {
	for _, d := range generatedDocs {
		raw, err := os.ReadFile(d.path)
		if err != nil {
			t.Fatalf("read %s: %v", d.path, err)
		}
		want, err := spliceCommands(string(raw), d.render())
		if err != nil {
			t.Fatalf("%s: %v", d.path, err)
		}
		if *updateGolden {
			if err := os.WriteFile(d.path, []byte(want), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if want != string(raw) {
			t.Errorf("%s is stale; run: go generate ./cmd/specsync", d.path)
		}
	}
}

var markedRow = regexp.MustCompile("(?m)^(?:\\| |- )`([a-z][a-z-]*)`")

// namesFromMarkedSection lists the commands in the generated section of a file.
func namesFromMarkedSection(path string) func(*testing.T) []string {
	return func(t *testing.T) []string {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		doc := string(raw)
		a, b := strings.Index(doc, commandsStart), strings.Index(doc, commandsEnd)
		if a < 0 || b < a {
			t.Fatalf("%s has no generated command section", path)
		}
		var names []string
		for _, m := range markedRow.FindAllStringSubmatch(doc[a:b], -1) {
			names = append(names, m[1])
		}
		return names
	}
}
