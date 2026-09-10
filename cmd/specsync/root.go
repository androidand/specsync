package main

import (
	"flag"
	"fmt"
	"os"

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
	if err := root.Validate(); err != nil {
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
