package specsync

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// RetentionPolicy is the retention action applied when archiving a change.
type RetentionPolicy string

const (
	RetentionPolicyMove RetentionPolicy = "move"  // relocate to openspec/changes/archive/
	RetentionPolicyPrune RetentionPolicy = "prune" // remove local folder entirely
)

// Config holds repository-level specsync configuration from .specsync/config.
type Config struct {
	RetentionPolicy RetentionPolicy // "move" or "prune"; empty = use heuristic
}

// ConfigPath is the conventional config file location relative to the repo root.
const ConfigPath = ".specsync/config"

// ReadConfig reads the repo-local config file at <root>/.specsync/config.
// Returns the zero config when the file is absent or unreadable.
// Format: plain key=value lines; comments start with #.
func ReadConfig(root string) Config {
	path := filepath.Join(root, ConfigPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}
	}
	return parseConfig(data)
}

// parseConfig parses a simple key=value config file. Supports lines of the
// form "key=value". Lines starting with # are comments. Unknown keys are ignored.
func parseConfig(data []byte) Config {
	cfg := Config{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		switch k {
		case "retain":
			switch RetentionPolicy(v) {
			case RetentionPolicyMove, RetentionPolicyPrune:
				cfg.RetentionPolicy = RetentionPolicy(v)
			}
		}
	}
	return cfg
}

// ResolveRetentionPolicy determines the retention policy using the resolution
// order: flag value → .specsync/config file → heuristic default.
//
// The heuristic default is "prune" — trivial changes default to pruning the
// local folder since the tracker issue holds the intent. Significant changes
// (determined by a later significance signal) default to "move".
func ResolveRetentionPolicy(flagRetain RetentionPolicy, repoRoot string) RetentionPolicy {
	// 1. Flag wins.
	if flagRetain != "" {
		return flagRetain
	}

	// 2. Config file.
	cfg := ReadConfig(repoRoot)
	if cfg.RetentionPolicy != "" {
		return cfg.RetentionPolicy
	}

	// 3. Heuristic default: prune (anti-bloat).
	// The significance heuristic (task 2) will refine this to "move" for
	// significant changes, but the default remains prune.
	return RetentionPolicyPrune
}
