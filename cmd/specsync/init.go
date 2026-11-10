package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	specsync "github.com/androidand/specsync"
)

// runInit creates a scratch openspec/ in the current worktree and keeps it out
// of git. It is the only way (besides `pull -init`) specsync creates one.
func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "show what would be created without touching disk")
	if err := deprecatedSlugFlag(args); err != nil {
		fail(err)
	}
	_ = fs.Parse(args)
	checkArgs(fs, args, 0)

	cwd, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	res, err := specsync.InitScratch(cwd, *dryRun)
	if err != nil {
		fail(err)
	}
	printInit(res, *dryRun, true)
}

func printInit(res specsync.InitResult, dryRun, hint bool) {
	prefix, verb := "specsync: ", ""
	if dryRun {
		prefix, verb = "DRY RUN — ", "would "
	}
	rel := func(p string) string {
		if cwd, err := os.Getwd(); err == nil {
			if r, err := filepath.Rel(cwd, p); err == nil {
				return r
			}
		}
		return p
	}
	if res.CreatedDir {
		fmt.Printf("%s%screate %s\n", prefix, verb, filepath.Join(rel(res.Dir), "changes"))
	} else {
		fmt.Printf("%s%s already exists\n", prefix, rel(res.Dir))
	}
	switch {
	case res.Excluded:
		fmt.Printf("%s%sadd %s to %s (local only; .gitignore is untouched)\n", prefix, verb, res.Pattern, res.ExcludeFile)
	case res.AlreadyIgnored:
		fmt.Printf("%sopenspec/ is already ignored by git; nothing to add\n", prefix)
	}
	if hint && res.CreatedDir && !dryRun {
		fmt.Println("next: specsync pull -issue <n>, work, specsync -change <slug>. Never commit openspec/.")
	}
}
