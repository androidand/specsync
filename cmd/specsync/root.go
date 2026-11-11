package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	specsync "github.com/androidand/specsync"
)

// addRootFlags registers the spec-root flags shared by every command that
// reads changes. -store is accepted everywhere -openspec is.
func addRootFlags(fs *flag.FlagSet) (openspec, store *string) {
	openspec = fs.String("openspec", "openspec", "path to the openspec/ directory")
	store = fs.String("store", "", "registered OpenSpec store id to operate on (default: openspec/config.yaml `store:`, else ./openspec)")
	return openspec, store
}

// resolveRoot applies the spec-root resolution order and fails loudly rather
// than letting a command print an empty result against a root that isn't there.
func resolveRoot(fs *flag.FlagSet, openspec, store *string) specsync.ResolvedRoot {
	root := resolveRootUnchecked(fs, openspec, store)
	if err := root.Validate(); err != nil {
		fail(err)
	}
	return root
}

// resolveRootInit is resolveRoot for commands that accept -init: when the
// repo-local spec root is missing it creates a scratch one (specsync init),
// and only when initFlag is set. A dry run reports the plan and creates nothing.
func resolveRootInit(fs *flag.FlagSet, openspec, store *string, initFlag, dryRun bool) specsync.ResolvedRoot {
	if initFlag && !flagVisited(fs, "openspec") && *store == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fail(err)
		}
		if info, err := os.Stat(filepath.Join(cwd, "openspec")); err != nil || !info.IsDir() {
			res, err := specsync.InitScratch(cwd, dryRun)
			if err != nil {
				fail(err)
			}
			printInit(res, dryRun, false)
			return specsync.ResolvedRoot{Dir: res.Dir, Rule: specsync.RootRuleLocal}
		}
	}
	return resolveRoot(fs, openspec, store)
}

func flagVisited(fs *flag.FlagSet, name string) bool {
	seen := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			seen = true
		}
	})
	return seen
}

func resolveRootUnchecked(fs *flag.FlagSet, openspec, store *string) specsync.ResolvedRoot {
	flagSet := flagVisited(fs, "openspec")
	cwd, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	root, err := specsync.ResolveSpecRoot(*openspec, flagSet, *store, cwd)
	if err != nil {
		fail(err)
	}
	return root
}

// guardStoreScope refuses an unscoped write against a store. A store spans
// repositories, so "every change" is an org-wide blast radius rather than
// one repo's backlog.
func guardStoreScope(root specsync.ResolvedRoot, slug string, all bool) {
	if !root.IsStore() || slug != "" || all {
		return
	}
	fmt.Fprintf(os.Stderr,
		"specsync: refusing an unscoped sync against store %q — a store spans repositories.\n"+
			"Pass -change <slug> for one change, or -all to sync every change in the store.\n",
		root.StoreID)
	os.Exit(2)
}

// guardUnscopedSync refuses a sync that would write to the tracker without
// naming its scope: one change (-change) or every change (-all). A dry run
// writes nothing and needs neither. An agent that ran `specsync` to see its
// commands once created an issue for every change in a repository.
func guardUnscopedSync(slug string, all, dryRun bool) {
	if err := unscopedSyncErr(slug, all, dryRun); err != nil {
		fmt.Fprintln(os.Stderr, "specsync:", err)
		os.Exit(2)
	}
}

func unscopedSyncErr(slug string, all, dryRun bool) error {
	if slug != "" || all || dryRun {
		return nil
	}
	return fmt.Errorf("refusing to sync without a scope: pass -change <slug> for one change, -all for every change, or -dry-run to preview")
}

// scopedTargetRepos returns the GitHub repos declared by a single scoped
// change. It yields nothing when the run is unscoped or when -repo was given,
// because an explicit flag always wins.
func scopedTargetRepos(openspecDir, slug, explicitRepo string) []string {
	if slug == "" || explicitRepo != "" {
		return nil
	}
	changes, err := specsync.LoadChanges(openspecDir)
	if err != nil {
		return nil
	}
	for _, c := range changes {
		if c.Slug == slug {
			return specsync.GitHubTargetRepos(c.Dir)
		}
	}
	return nil
}

// repoTop is the top of the git work tree containing dir, or dir itself when
// it is not in one.
func repoTop(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return dir
	}
	return strings.TrimSpace(string(out))
}

// textRepoRoot is the repository whose openspec/specsync.yml holds the text
// standard: the one containing a change's own spec root, and for a store (which
// spans repositories) the one the command runs in.
func textRepoRoot(resolved *specsync.ResolvedRoot) string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	if resolved != nil && !resolved.IsStore() {
		return repoTop(filepath.Dir(resolved.Dir))
	}
	return repoTop(cwd)
}
