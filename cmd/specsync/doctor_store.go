package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	specsync "github.com/androidand/specsync"
)

// StoreDiagnostic reports the health of the resolved spec root, with the
// checks that matter when planning lives in a store shared by a team.
type StoreDiagnostic struct {
	Root           string   `json:"root"`
	Rule           string   `json:"rule"`
	StoreID        string   `json:"store_id,omitempty"`
	IsStore        bool     `json:"is_store"`
	Registered     bool     `json:"registered"`
	BehindRemote   bool     `json:"behind_remote"`
	Dirty          bool     `json:"dirty"`
	Changes        int      `json:"changes"`
	MissingTargets []string `json:"missing_targets,omitempty"`
	Uncontracted   []string `json:"uncontracted,omitempty"`
	Problems       []string `json:"problems,omitempty"`
}

func doctorStore(asJSON bool) {
	cwd, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	root, err := specsync.ResolveSpecRoot("openspec", false, "", cwd)
	if err != nil {
		fail(err)
	}
	d := StoreDiagnostic{Root: root.Dir, Rule: string(root.Rule), StoreID: root.StoreID, IsStore: root.IsStore()}

	if err := root.Validate(); err != nil {
		d.Problems = append(d.Problems, err.Error())
		emitStoreDiagnostic(d, asJSON)
		return
	}

	if root.IsStore() {
		if _, err := specsync.StoreLocation(root.StoreID); err == nil {
			d.Registered = true
		} else {
			d.Problems = append(d.Problems, err.Error())
		}
		storeRoot := filepath.Dir(root.Dir)
		if out, err := exec.Command("git", "-C", storeRoot, "status", "--porcelain").Output(); err == nil && len(strings.TrimSpace(string(out))) > 0 {
			d.Dirty = true
			d.Problems = append(d.Problems, "store has uncommitted changes — commit and push so teammates see them")
		}
		_ = exec.Command("git", "-C", storeRoot, "fetch", "--quiet").Run()
		if out, err := exec.Command("git", "-C", storeRoot, "rev-list", "--count", "HEAD..@{upstream}").Output(); err == nil {
			if n := strings.TrimSpace(string(out)); n != "" && n != "0" {
				d.BehindRemote = true
				d.Problems = append(d.Problems, fmt.Sprintf("store is %s commit(s) behind its remote — run: git -C %s pull", n, storeRoot))
			}
		}
	}

	changes, err := specsync.LoadChanges(root.Dir)
	if err != nil {
		fail(err)
	}
	for _, c := range changes {
		if c.Archived {
			continue
		}
		d.Changes++
		if root.IsStore() && len(c.Targets) == 0 {
			d.MissingTargets = append(d.MissingTargets, c.Slug)
		}
		if len(c.Deltas) == 0 {
			d.Uncontracted = append(d.Uncontracted, c.Slug)
		}
	}
	if len(d.MissingTargets) > 0 {
		d.Problems = append(d.Problems, fmt.Sprintf(
			"%d change(s) in the store declare no targets — they will be filed wherever the shell happens to be standing",
			len(d.MissingTargets)))
	}
	emitStoreDiagnostic(d, asJSON)
}

func emitStoreDiagnostic(d StoreDiagnostic, asJSON bool) {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(d); err != nil {
			fail(err)
		}
		if len(d.Problems) > 0 {
			os.Exit(1)
		}
		return
	}
	fmt.Printf("spec root: %s\n", d.Root)
	fmt.Printf("  resolved via: %s\n", d.Rule)
	if d.IsStore {
		fmt.Printf("  store: %s (registered: %v, dirty: %v, behind remote: %v)\n", d.StoreID, d.Registered, d.Dirty, d.BehindRemote)
	} else {
		fmt.Printf("  store: none (repo-local)\n")
	}
	fmt.Printf("  active changes: %d\n", d.Changes)
	if len(d.MissingTargets) > 0 {
		fmt.Printf("  without targets: %s\n", strings.Join(d.MissingTargets, ", "))
	}
	if len(d.Uncontracted) > 0 {
		fmt.Printf("  without a specs/ delta: %s\n", strings.Join(d.Uncontracted, ", "))
	}
	if len(d.Problems) == 0 {
		fmt.Println("\nno problems found")
		return
	}
	fmt.Println()
	for _, p := range d.Problems {
		fmt.Printf("  ✗ %s\n", p)
	}
	os.Exit(1)
}
