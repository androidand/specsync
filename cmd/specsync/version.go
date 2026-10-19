package main

import (
	"context"
	"flag"
	"fmt"
	"runtime/debug"
)

// versionString renders the version line printed by `specsync version` (and
// `--version`). A released binary has version stamped via -ldflags "-X
// main.version=..." (see .goreleaser.yaml) and is returned as-is. A local/dev
// build (version == "dev", the source default) instead reports the VCS
// revision Go's toolchain embeds automatically via runtime/debug — no ldflags
// needed for this path — so "specsync dev" from a checkout can be told apart
// from another, and from a released binary, without guessing.
func versionString() string {
	if version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	var revision string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if revision == "" {
		return version
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	if dirty {
		return fmt.Sprintf("%s (%s, dirty)", version, revision)
	}
	return fmt.Sprintf("%s (%s)", version, revision)
}

// versionCmd prints the binary version. `-version` and `--version` select it
// too (see resolveSubcommand).
type versionCmd struct{}

func init() { Register(&versionCmd{}) }

func (*versionCmd) Spec() Spec {
	return Spec{
		Name:     "version",
		Group:    GroupHousekeeping,
		Summary:  "Print the specsync version.",
		Workflow: AgentCommandWorkflow{Position: "any"},
		Safety:   []string{"Read-only"},
		Examples: []string{"specsync version"},
	}
}

func (*versionCmd) Flags(*flag.FlagSet) {}

func (cm *versionCmd) Run(_ context.Context, args []string) error {
	fs := flagSetFor(cm)
	if err := fs.Parse(args); err != nil {
		return err
	}
	fmt.Println("specsync " + versionString())
	return nil
}
