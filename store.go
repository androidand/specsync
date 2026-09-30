package specsync

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ChangeConfigPath is the committed per-change config, relative to a change
// folder. It is deliberately NOT under .specsync/, which is gitignored: a
// change's targets must travel with the change to every teammate's clone.
const ChangeConfigPath = "specsync.yml"

// StoreMarkerPath identifies a standalone OpenSpec store, relative to the
// store root (the parent of its openspec/ directory).
const StoreMarkerPath = ".openspec-store/store.yaml"

// RootRule names which branch of the spec-root resolution order was taken.
type RootRule string

const (
	RootRuleFlag    RootRule = "flag"         // -openspec
	RootRuleStore   RootRule = "store-flag"   // -store
	RootRuleDeclare RootRule = "config-store" // openspec/config.yaml store:
	RootRuleLocal   RootRule = "repo-local"   // ./openspec
)

// ResolvedRoot is a spec root and how it was found.
type ResolvedRoot struct {
	Dir     string // absolute path to the openspec/ directory
	Rule    RootRule
	StoreID string // non-empty when the root is a store
}

// IsStore reports whether the root lives in a standalone store repo.
func (r ResolvedRoot) IsStore() bool { return r.StoreID != "" }

// ResolveSpecRoot locates the openspec/ directory to operate on.
// Order: -openspec flag → -store id → config.yaml store: → ./openspec.
//
// flagSet distinguishes "-openspec was passed" from "-openspec defaulted",
// because the default value is itself a valid relative path.
func ResolveSpecRoot(flagPath string, flagSet bool, storeID, cwd string) (ResolvedRoot, error) {
	if flagSet && flagPath != "" {
		abs, err := filepath.Abs(flagPath)
		if err != nil {
			return ResolvedRoot{}, err
		}
		return ResolvedRoot{Dir: abs, Rule: RootRuleFlag, StoreID: storeIDAt(abs)}, nil
	}
	if storeID != "" {
		dir, err := StoreLocation(storeID)
		if err != nil {
			return ResolvedRoot{}, err
		}
		return ResolvedRoot{Dir: filepath.Join(dir, "openspec"), Rule: RootRuleStore, StoreID: storeID}, nil
	}
	local := filepath.Join(cwd, "openspec")
	if declared := ReadDeclaredStore(local); declared != "" {
		dir, err := StoreLocation(declared)
		if err != nil {
			return ResolvedRoot{}, fmt.Errorf("%s declares store %q: %w", filepath.Join(local, "config.yaml"), declared, err)
		}
		return ResolvedRoot{Dir: filepath.Join(dir, "openspec"), Rule: RootRuleDeclare, StoreID: declared}, nil
	}
	abs, err := filepath.Abs(flagPath)
	if err != nil {
		return ResolvedRoot{}, err
	}
	return ResolvedRoot{Dir: abs, Rule: RootRuleLocal, StoreID: storeIDAt(abs)}, nil
}

// Validate reports an actionable error when the resolved root holds no
// changes directory, rather than letting callers print an empty result.
func (r ResolvedRoot) Validate() error {
	if info, err := os.Stat(r.Dir); err != nil || !info.IsDir() {
		if r.IsStore() {
			return fmt.Errorf("store %q has no openspec/ directory at %s", r.StoreID, r.Dir)
		}
		return fmt.Errorf("no openspec/ directory at %s (pass -openspec, -store, or declare `store:` in openspec/config.yaml)", r.Dir)
	}
	changes := filepath.Join(r.Dir, "changes")
	if info, err := os.Stat(changes); err != nil || !info.IsDir() {
		return fmt.Errorf("no changes/ directory at %s", changes)
	}
	return nil
}

// storeIDAt returns the store id when the given openspec/ dir sits inside a
// store repo, or "" when it does not.
func storeIDAt(openspecDir string) string {
	marker := filepath.Join(filepath.Dir(openspecDir), StoreMarkerPath)
	data, err := os.ReadFile(marker)
	if err != nil {
		return ""
	}
	if id := scalarField(string(data), "id"); id != "" {
		return id
	}
	return filepath.Base(filepath.Dir(openspecDir))
}

// ReadDeclaredStore returns the `store:` id declared in <openspecDir>/config.yaml,
// or "" when the file is absent or declares none.
func ReadDeclaredStore(openspecDir string) string {
	data, err := os.ReadFile(filepath.Join(openspecDir, "config.yaml"))
	if err != nil {
		return ""
	}
	return scalarField(string(data), "store")
}

// storeListEntry tolerates the several names OpenSpec has used for a store's
// local path while stores are beta.
type storeListEntry struct {
	ID       string `json:"id"`
	Root     string `json:"root"`
	Location string `json:"location"`
	Path     string `json:"path"`
}

func (e storeListEntry) location() string {
	for _, v := range []string{e.Root, e.Location, e.Path} {
		if v != "" {
			return v
		}
	}
	return ""
}

// StoreLocation resolves a store id to its local path using OpenSpec's own
// machine registry. specsync keeps no second registry; the format stays
// OpenSpec's to change.
func StoreLocation(id string) (string, error) {
	out, err := exec.Command("openspec", "store", "list", "--json").Output()
	if err != nil {
		return "", fmt.Errorf("openspec store list --json (is openspec installed?): %w", err)
	}
	entries, err := parseStoreList(out)
	if err != nil {
		return "", err
	}
	var known []string
	for _, e := range entries {
		loc := e.location()
		if e.ID == id {
			if loc == "" {
				return "", fmt.Errorf("store %q is registered without a location", id)
			}
			return loc, nil
		}
		known = append(known, e.ID)
	}
	if len(known) == 0 {
		return "", fmt.Errorf("store %q is not registered; run: openspec store register <path>", id)
	}
	return "", fmt.Errorf("store %q is not registered (known: %s); run: openspec store register <path>", id, strings.Join(known, ", "))
}

// parseStoreList accepts either a bare array or an object wrapping one, since
// stores are beta and the envelope has moved between releases.
func parseStoreList(data []byte) ([]storeListEntry, error) {
	var direct []storeListEntry
	if err := json.Unmarshal(data, &direct); err == nil {
		return direct, nil
	}
	var wrapped struct {
		Stores []storeListEntry `json:"stores"`
		Items  []storeListEntry `json:"items"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, fmt.Errorf("parse openspec store list output: %w", err)
	}
	if len(wrapped.Stores) > 0 {
		return wrapped.Stores, nil
	}
	return wrapped.Items, nil
}

// ChangeTargets returns the provider keys a change declares in its committed
// specsync.yml, or nil when it declares none.
func ChangeTargets(changeDir string) []string {
	data, err := os.ReadFile(filepath.Join(changeDir, ChangeConfigPath))
	if err != nil {
		return nil
	}
	return parseTargets(string(data))
}

// WriteChangeTargets records a change's target provider keys, preserving any
// other keys already in the file.
func WriteChangeTargets(changeDir string, targets []string) error {
	if len(targets) == 0 {
		return nil
	}
	path := filepath.Join(changeDir, ChangeConfigPath)
	var kept []string
	if data, err := os.ReadFile(path); err == nil {
		inTargets := false
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "targets:") {
				inTargets = true
				continue
			}
			if inTargets {
				if strings.HasPrefix(trimmed, "-") || trimmed == "" {
					continue
				}
				inTargets = false
			}
			if trimmed != "" {
				kept = append(kept, line)
			}
		}
	}
	var b strings.Builder
	for _, line := range kept {
		b.WriteString(line + "\n")
	}
	b.WriteString("targets:\n")
	for _, t := range targets {
		b.WriteString("  - " + t + "\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// RepoFromProviderKey extracts "owner/name" from a "github:owner/name" key.
// Keys for other providers, and bare keys, yield "".
func RepoFromProviderKey(key string) string {
	rest, ok := strings.CutPrefix(key, "github:")
	if !ok || rest == "" {
		return ""
	}
	return rest
}

// GitHubTargetRepos returns the GitHub repos a change targets, in order.
func GitHubTargetRepos(changeDir string) []string {
	var out []string
	for _, t := range ChangeTargets(changeDir) {
		if repo := RepoFromProviderKey(t); repo != "" {
			out = append(out, repo)
		}
	}
	return out
}

func parseTargets(src string) []string {
	var out []string
	inTargets := false
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "targets:") {
			inTargets = true
			if v := strings.TrimSpace(strings.TrimPrefix(trimmed, "targets:")); v != "" && v != "[]" {
				for _, part := range strings.Split(strings.Trim(v, "[]"), ",") {
					if p := strings.Trim(strings.TrimSpace(part), `"'`); p != "" {
						out = append(out, p)
					}
				}
				inTargets = false
			}
			continue
		}
		if !inTargets {
			continue
		}
		if !strings.HasPrefix(trimmed, "-") {
			break
		}
		if v := strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "-")), `"'`); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// scalarField reads a top-level "key: value" scalar from minimal YAML.
func scalarField(src, key string) string {
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		k, v, ok := strings.Cut(trimmed, ":")
		if !ok || strings.TrimSpace(k) != key {
			continue
		}
		return strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return ""
}
