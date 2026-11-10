package main

import (
	"fmt"
	"os"
	"path/filepath"

	specsync "github.com/androidand/specsync"
)

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
