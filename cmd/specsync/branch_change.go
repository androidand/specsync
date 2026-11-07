package main

import (
	"fmt"
	"os"

	specsync "github.com/androidand/specsync"
)

// changeFromBranchFn resolves the change named by the current branch. A
// variable so tests can stub git.
var changeFromBranchFn = specsync.ChangeFromCurrentBranch

// splitChangeArgs splits the positional args of a command shaped
// `<cmd> <change> <rest...>`. Given len(rest) args only, the change is taken
// from the current branch, and the pick is reported on stderr.
func splitChangeArgs(cmd, openspecDir string, args []string, rest int) (string, []string, error) {
	switch len(args) {
	case rest + 1:
		return args[0], args[1:], nil
	case rest:
		slug, err := inferChange(cmd, openspecDir)
		if err != nil {
			return "", nil, err
		}
		return slug, args, nil
	default:
		return "", nil, fmt.Errorf("wrong number of arguments")
	}
}

// inferChange returns the change named by the current branch, reporting the
// pick on stderr, or an error naming the branch when it names no change.
func inferChange(cmd, openspecDir string) (string, error) {
	slug, branch := changeFromBranchFn(openspecDir)
	if slug == "" {
		if branch == "" {
			return "", fmt.Errorf("%s: no change given, and not on a branch", cmd)
		}
		return "", fmt.Errorf("%s: no change given, and branch %q names no active change", cmd, branch)
	}
	fmt.Fprintf(os.Stderr, "%s: using change %s (from branch %s)\n", cmd, slug, branch)
	return slug, nil
}
