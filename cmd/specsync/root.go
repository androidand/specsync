package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

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
	root := resolveRootUnchecked(fs, openspec, store)
	verr := root.Validate()
	if verr == nil {
		return root
	}
	if !initFlag || root.Rule != specsync.RootRuleLocal || filepath.Base(root.Dir) != "openspec" {
		fail(verr)
	}
	res, err := specsync.InitScratch(filepath.Dir(root.Dir), dryRun)
	if err != nil {
		fail(err)
	}
	printInit(res, dryRun)
	return root
}

func resolveRootUnchecked(fs *flag.FlagSet, openspec, store *string) specsync.ResolvedRoot {
	flagSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "openspec" {
			flagSet = true
		}
	})
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
