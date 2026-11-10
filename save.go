package specsync

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// SaveOptions describes one `specsync save`.
type SaveOptions struct {
	Dir     string
	Message string
	// Add names untracked files to include. Untracked files are never staged
	// otherwise.
	Add  []string
	Ways Ways
	// Push pushes the branch after committing; it never forces.
	Push bool
	// PushToRequest allows pushing a branch that has an open request.
	PushToRequest bool
	// OpenRequest reports the number of an open request for the branch, 0 when
	// there is none. nil means the provider could not be asked.
	OpenRequest func(ctx context.Context, branch string) (int, error)
	DryRun      bool
}

// SaveResult reports what a save did, or on a dry run would do.
type SaveResult struct {
	Files     []string `json:"files"`
	Branch    string   `json:"branch"`
	Committed bool     `json:"committed"`
	Pushed    bool     `json:"pushed"`
	Commit    string   `json:"commit,omitempty"`
}

var secretNames = []string{".env", ".env.*", "*.pem", "*.key", "*.p12", "*.pfx", "*.keystore", "*.jks",
	"id_rsa", "id_rsa.*", "id_dsa", "id_ecdsa", "id_ed25519", ".netrc", ".npmrc", ".pgpass", "*.secret", "*.secrets", "secrets.*", "credentials.json"}

var secretNameExceptions = []string{".env.example", ".env.sample", ".env.template", ".env.dist", "id_rsa.pub", "id_ed25519.pub"}

var secretContent = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----|\bgh[pousr]_[A-Za-z0-9]{36,}\b|\bgithub_pat_[A-Za-z0-9_]{20,}\b|\bAKIA[0-9A-Z]{16}\b|\bsk-ant-[A-Za-z0-9_-]{20,}\b`)

func secretReason(name string, extra []string) string {
	base := strings.ToLower(path.Base(filepath.ToSlash(name)))
	for _, ok := range secretNameExceptions {
		if base == ok {
			return ""
		}
	}
	for _, p := range secretNames {
		if m, _ := path.Match(p, base); m {
			return "looks like a secret by name"
		}
	}
	for _, p := range extra {
		if m, _ := path.Match(p, base); m {
			return "matches save.forbid_files " + p
		}
		if m, _ := path.Match(p, filepath.ToSlash(name)); m {
			return "matches save.forbid_files " + p
		}
	}
	return ""
}

func forbiddenTrailer(message string, forbidden []string) string {
	if len(forbidden) == 0 {
		return ""
	}
	for _, line := range strings.Split(message, "\n") {
		key, _, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		for _, f := range forbidden {
			if strings.EqualFold(key, f) {
				return key
			}
		}
	}
	return ""
}

func checkAddPath(dir, p string) (string, error) {
	clean := filepath.Clean(p)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is outside the worktree", p)
	}
	info, err := os.Stat(filepath.Join(dir, clean))
	if err != nil {
		return "", fmt.Errorf("%s: %v", p, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a folder: name the files to add", p)
	}
	return filepath.ToSlash(clean), nil
}

// Save commits the tracked changes (and the untracked files named in Add),
// optionally pushes, and refuses what looks unsafe: secret-looking files,
// trailers the config forbids, and a push onto a branch with an open request.
func Save(ctx context.Context, o SaveOptions) (SaveResult, error) {
	var res SaveResult
	if strings.TrimSpace(o.Message) == "" {
		return res, fmt.Errorf("save needs a message: -m <message>")
	}
	if t := forbiddenTrailer(o.Message, o.Ways.Save.ForbidTrailers); t != "" {
		return res, fmt.Errorf("the commit message has a %q trailer, which %s forbids (save.forbid_trailers)", t, SpecSyncConfigPath)
	}
	branch, err := gitIn(ctx, o.Dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return res, err
	}
	res.Branch = branch

	tracked, err := gitIn(ctx, o.Dir, "diff", "--name-only", "-z", "HEAD")
	if err != nil {
		return res, err
	}
	var files []string
	seen := map[string]bool{}
	for _, f := range strings.Split(tracked, "\x00") {
		if f != "" && !seen[f] {
			seen[f] = true
			files = append(files, f)
		}
	}
	var adds []string
	for _, a := range o.Add {
		clean, err := checkAddPath(o.Dir, a)
		if err != nil {
			return res, err
		}
		adds = append(adds, clean)
		if !seen[clean] {
			seen[clean] = true
			files = append(files, clean)
		}
	}
	res.Files = files
	if len(files) == 0 {
		return res, fmt.Errorf("nothing to commit (untracked files are only included when named with -add)")
	}

	var refused []string
	for _, f := range files {
		if why := secretReason(f, o.Ways.Save.ForbidFiles); why != "" {
			refused = append(refused, f+" ("+why+")")
			continue
		}
		if data, err := os.ReadFile(filepath.Join(o.Dir, f)); err == nil && len(data) <= 1<<20 && secretContent.Match(data) {
			refused = append(refused, f+" (contains what looks like a key or token)")
		}
	}
	if len(refused) > 0 {
		return res, fmt.Errorf("refusing to commit: %s", strings.Join(refused, "; "))
	}

	if o.DryRun {
		return res, nil
	}
	if _, err := gitIn(ctx, o.Dir, "add", "-u"); err != nil {
		return res, err
	}
	if len(adds) > 0 {
		if _, err := gitIn(ctx, o.Dir, append([]string{"add", "--"}, adds...)...); err != nil {
			return res, err
		}
	}
	if _, err := gitIn(ctx, o.Dir, "commit", "-q", "-m", o.Message); err != nil {
		return res, err
	}
	res.Committed = true
	res.Commit, _ = gitIn(ctx, o.Dir, "rev-parse", "HEAD")

	if !o.Push {
		return res, nil
	}
	return res, pushBranch(ctx, o, branch, &res)
}

func pushBranch(ctx context.Context, o SaveOptions, branch string, res *SaveResult) error {
	if branch == "HEAD" {
		return fmt.Errorf("not pushed: HEAD is detached; check out a branch")
	}
	if !o.PushToRequest {
		if o.OpenRequest == nil {
			return fmt.Errorf("not pushed: cannot tell whether a pull request is open for %s; pass -push-to-request to push anyway", branch)
		}
		n, err := o.OpenRequest(ctx, branch)
		if err != nil {
			return fmt.Errorf("not pushed: cannot tell whether a pull request is open for %s (%v); pass -push-to-request to push anyway", branch, err)
		}
		if n > 0 {
			return fmt.Errorf("not pushed: %s has an open pull request (#%d), and a push notifies its reviewers; pass -push-to-request to push anyway", branch, n)
		}
	}
	remote, _ := gitIn(ctx, o.Dir, "config", "--get", "branch."+branch+".remote")
	if remote != "" {
		if _, err := gitIn(ctx, o.Dir, "push"); err != nil {
			return err
		}
		res.Pushed = true
		return nil
	}
	remotes, err := gitIn(ctx, o.Dir, "remote")
	if err != nil {
		return err
	}
	var name string
	for _, r := range strings.Fields(remotes) {
		if r == "origin" {
			name = r
		}
	}
	if name == "" {
		if f := strings.Fields(remotes); len(f) == 1 {
			name = f[0]
		}
	}
	if name == "" {
		return fmt.Errorf("not pushed: no remote to push %s to", branch)
	}
	if _, err := gitIn(ctx, o.Dir, "push", "-u", name, branch); err != nil {
		return err
	}
	res.Pushed = true
	return nil
}
