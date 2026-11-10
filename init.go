package specsync

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// InitResult reports what InitScratch did (or, on a dry run, would do).
type InitResult struct {
	Dir            string // absolute path to the openspec/ directory
	CreatedDir     bool   // openspec/changes/ did not exist and was created
	Excluded       bool   // an ignore line was added to the git exclude file
	AlreadyIgnored bool   // openspec/ was already ignored, nothing added
	ExcludeFile    string // exclude file consulted or edited
	Pattern        string // exclude pattern added
}

// InitScratch creates a scratch openspec/changes/ at <repoDir>/openspec and
// keeps it out of git through the clone's info/exclude file (shared by all its
// worktrees), never a tracked .gitignore. It is idempotent and refuses where openspec/ is tracked,
// because hiding tracked files would be a silent repo-structure change.
func InitScratch(repoDir string, dryRun bool) (InitResult, error) {
	abs, err := filepath.Abs(repoDir)
	if err != nil {
		return InitResult{}, err
	}
	res := InitResult{Dir: filepath.Join(abs, "openspec")}

	top, err := gitOut(abs, "rev-parse", "--show-toplevel")
	if err != nil {
		return InitResult{}, fmt.Errorf("specsync init needs a git repository at %s: %w", abs, err)
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	rel, err := filepath.Rel(top, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return InitResult{}, fmt.Errorf("%s is outside the git work tree %s", abs, top)
	}
	rel = filepath.ToSlash(rel)
	spec := "openspec"
	if rel != "." {
		spec = rel + "/openspec"
	}

	if tracked, _ := gitOut(abs, "ls-files", "--", "openspec"); tracked != "" {
		return InitResult{}, fmt.Errorf("openspec/ is tracked in this repo; it already holds specs, so init has nothing to do (use -openspec or -store to point elsewhere)")
	}

	changes := filepath.Join(res.Dir, "changes")
	if info, err := os.Stat(changes); err != nil || !info.IsDir() {
		res.CreatedDir = true
	}

	ignored, err := gitCheckIgnored(abs, "openspec/changes")
	if err != nil {
		return InitResult{}, err
	}
	if ignored {
		res.AlreadyIgnored = true
	} else {
		exclude, err := gitOut(abs, "rev-parse", "--git-path", "info/exclude")
		if err != nil {
			return InitResult{}, err
		}
		if !filepath.IsAbs(exclude) {
			exclude = filepath.Join(abs, exclude)
		}
		res.ExcludeFile = exclude
		res.Pattern = "/" + spec + "/"
		res.Excluded = true
	}

	if dryRun {
		return res, nil
	}
	if err := os.MkdirAll(changes, 0o755); err != nil {
		return InitResult{}, fmt.Errorf("create %s: %w", changes, err)
	}
	if res.Excluded {
		if err := appendLine(res.ExcludeFile, res.Pattern); err != nil {
			return InitResult{}, err
		}
	}
	return res, nil
}

// appendLine adds line to path unless an identical line is already there.
func appendLine(path, line string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, l := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(l) == line {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	text := string(data)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return os.WriteFile(path, []byte(text+line+"\n"), 0o644)
}

// gitCheckIgnored reports whether path is ignored. git exits 1 for "not
// ignored"; any other failure is a real error and is returned.
func gitCheckIgnored(dir, path string) (bool, error) {
	cmd := exec.Command("git", "check-ignore", "-q", "--", path)
	cmd.Dir = dir
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git check-ignore: %w", err)
}

func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
