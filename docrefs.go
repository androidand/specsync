package specsync

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// DocRef is a commit hash or request reference found in a markdown document.
type DocRef struct {
	File   string
	Line   int
	Kind   string // "commit" or "request"
	Text   string // the reference as written
	Repo   string // owner/name for a qualified request reference
	Number int    // request or issue number
}

var (
	codeSpan     = regexp.MustCompile("`([^`\n]+)`")
	commitWord   = regexp.MustCompile("(?i)\\b(?:commits?|sha|fixed in)\\s+`?([0-9a-f]{7,40})\\b")
	hexHash      = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)
	urlText      = regexp.MustCompile(`https?://\S+`)
	qualifiedReq = regexp.MustCompile(`(?:^|[^\w./-])([A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9._-]+)#([0-9]+)\b`)
	bareReq      = regexp.MustCompile(`(?:^|[\s(\[,;:])#([0-9]+)\b`)
	hasDigit     = regexp.MustCompile(`[0-9]`)
	hasHexLetter = regexp.MustCompile(`[a-fA-F]`)
	fileName     = regexp.MustCompile(`(?i)\.(md|txt|go|json|ya?ml|html?|js|ts|py|sh|toml|css)$`)
)

func looksLikeHash(h string) bool {
	return hasDigit.MatchString(h) && hasHexLetter.MatchString(h)
}

// ScanDocRefs finds the commit hashes and request references in text. It
// favours precision. A commit hash is 7 to 40 hex characters with at least one
// digit and one letter, written as inline code or after "commit", "sha" or
// "fixed in". A request reference is #n or owner/repo#n. Fenced code blocks,
// URLs, markdown anchors such as (#2-output) and file paths such as
// docs/guide.md#4 are ignored.
func ScanDocRefs(file, text string) []DocRef {
	type found struct {
		pos int
		ref DocRef
	}
	var refs []DocRef
	fence := ""
	for i, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if marker := fenceMarker(trimmed); marker != "" {
			if fence == "" {
				fence = marker
			} else if marker == fence {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		n := i + 1
		var cands []found
		for _, m := range codeSpan.FindAllStringSubmatchIndex(line, -1) {
			if h := line[m[2]:m[3]]; hexHash.MatchString(h) && looksLikeHash(h) {
				cands = append(cands, found{m[0], DocRef{File: file, Line: n, Kind: "commit", Text: h}})
			}
		}
		for _, m := range commitWord.FindAllStringSubmatchIndex(line, -1) {
			if h := line[m[2]:m[3]]; looksLikeHash(h) {
				cands = append(cands, found{m[2], DocRef{File: file, Line: n, Kind: "commit", Text: h}})
			}
		}

		plain := urlText.ReplaceAllString(line, " ")
		for _, m := range qualifiedReq.FindAllStringSubmatchIndex(plain, -1) {
			repo := plain[m[2]:m[3]]
			if fileName.MatchString(repo) || anchorFollows(plain, m[5]) {
				continue
			}
			num, _ := strconv.Atoi(plain[m[4]:m[5]])
			cands = append(cands, found{m[2], DocRef{File: file, Line: n, Kind: "request", Text: repo + "#" + plain[m[4]:m[5]], Repo: repo, Number: num}})
		}
		for _, m := range bareReq.FindAllStringSubmatchIndex(plain, -1) {
			if anchorFollows(plain, m[3]) {
				continue
			}
			num, _ := strconv.Atoi(plain[m[2]:m[3]])
			cands = append(cands, found{m[2], DocRef{File: file, Line: n, Kind: "request", Text: "#" + plain[m[2]:m[3]], Number: num}})
		}
		sort.SliceStable(cands, func(a, b int) bool { return cands[a].pos < cands[b].pos })
		seen := map[string]bool{}
		for _, c := range cands {
			key := c.ref.Kind + c.ref.Text
			if seen[key] {
				continue
			}
			seen[key] = true
			refs = append(refs, c.ref)
		}
	}
	return refs
}

func fenceMarker(trimmed string) string {
	for _, m := range []string{"```", "~~~"} {
		if strings.HasPrefix(trimmed, m) {
			return m
		}
	}
	return ""
}

// anchorFollows reports whether a "-" follows the digits ending at end, as in
// the markdown anchor (#2-output).
func anchorFollows(s string, end int) bool {
	return end < len(s) && s[end] == '-'
}

// CommitCheck is the outcome of resolving one commit reference.
type CommitCheck struct {
	Ref     DocRef
	Status  string // "ok", "stale" (does not resolve), "orphaned" (on no branch) or "unknown" (not fetched in a shallow clone)
	Subject string // the commit subject, for an orphaned hash
}

// DocRefChecker resolves commit references in one repository with git only,
// caching each hash and the branch lists so a run asks git once per fact.
type DocRefChecker struct {
	ctx      context.Context
	dir      string
	resolved map[string]docSHA
	base     []string
	branches []string
	listed   bool
	shallow  *bool
}

type docSHA struct {
	sha       string
	notCommit bool
}

// NewDocRefChecker returns a checker for the repository at dir.
func NewDocRefChecker(ctx context.Context, dir string) *DocRefChecker {
	c := &DocRefChecker{ctx: ctx, dir: dir, resolved: map[string]docSHA{}}
	add := func(ref string) {
		if _, err := gitIn(ctx, dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err == nil {
			c.base = append(c.base, ref)
		}
	}
	add("HEAD")
	if out, err := gitIn(ctx, dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		add(strings.TrimSpace(out))
	}
	add("main")
	add("master")
	return c
}

// Reachable lists the refs a cited commit may be on: HEAD, the default branch
// and any branch named for the change.
func (c *DocRefChecker) Reachable(slug string) []string {
	refs := append([]string(nil), c.base...)
	if slug == "" {
		return refs
	}
	if !c.listed {
		c.listed = true
		if out, err := gitIn(c.ctx, c.dir, "for-each-ref", "--format=%(refname:short)", "refs/heads", "refs/remotes"); err == nil {
			c.branches = strings.Split(out, "\n")
		}
	}
	for _, name := range c.branches {
		if name == slug || strings.HasSuffix(name, "/"+slug) || strings.HasSuffix(name, "-"+slug) {
			refs = append(refs, name)
		}
	}
	return refs
}

func (c *DocRefChecker) isShallow() bool {
	if c.shallow == nil {
		out, err := gitIn(c.ctx, c.dir, "rev-parse", "--is-shallow-repository")
		v := err == nil && strings.TrimSpace(out) == "true"
		c.shallow = &v
	}
	return *c.shallow
}

func (c *DocRefChecker) resolve(hash string) (docSHA, bool) {
	if r, ok := c.resolved[hash]; ok {
		return r, r.sha != "" || r.notCommit
	}
	var r docSHA
	if sha, err := gitIn(c.ctx, c.dir, "rev-parse", "--verify", "--quiet", hash+"^{commit}"); err == nil && sha != "" {
		r.sha = sha
	} else if _, err := gitIn(c.ctx, c.dir, "cat-file", "-e", hash); err == nil {
		r.notCommit = true
	}
	c.resolved[hash] = r
	return r, r.sha != "" || r.notCommit
}

// Commits resolves every commit reference. A hash that does not resolve (or is
// ambiguous) is stale, or unknown in a shallow clone. One that resolves but is
// on no branch and an ancestor of none of reachable is orphaned. A hash that
// names an object that is not a commit is not reported. Other kinds of
// reference are skipped.
func (c *DocRefChecker) Commits(refs []DocRef, reachable []string) []CommitCheck {
	var out []CommitCheck
	for _, r := range refs {
		if r.Kind != "commit" {
			continue
		}
		res, found := c.resolve(r.Text)
		switch {
		case res.notCommit:
			continue
		case !found:
			status := "stale"
			if c.isShallow() {
				status = "unknown"
			}
			out = append(out, CommitCheck{Ref: r, Status: status})
			continue
		}
		chk := CommitCheck{Ref: r, Status: "ok"}
		if !c.onABranch(res.sha, reachable) {
			chk.Status = "orphaned"
			chk.Subject, _ = gitIn(c.ctx, c.dir, "show", "-s", "--format=%s", res.sha)
		}
		out = append(out, chk)
	}
	return out
}

func (c *DocRefChecker) onABranch(sha string, reachable []string) bool {
	if out, err := gitIn(c.ctx, c.dir, "branch", "-a", "--contains", sha); err == nil && strings.TrimSpace(out) != "" {
		return true
	}
	for _, ref := range reachable {
		if _, err := gitIn(c.ctx, c.dir, "merge-base", "--is-ancestor", sha, ref); err == nil {
			return true
		}
	}
	return false
}

// CheckCommits resolves the commit references in refs against the repository at dir.
func CheckCommits(ctx context.Context, dir string, refs []DocRef, reachable []string) []CommitCheck {
	return NewDocRefChecker(ctx, dir).Commits(refs, reachable)
}

// DocRefReachableRefs lists the refs a cited commit may legitimately be on.
func DocRefReachableRefs(ctx context.Context, dir, slug string) []string {
	return NewDocRefChecker(ctx, dir).Reachable(slug)
}

// RequestCheck is the outcome of looking up one request or issue reference.
type RequestCheck struct {
	Ref    DocRef
	State  string // "open", "closed", "merged" or "missing"
	Reason string
}

// CheckRequests looks up request and issue references through the provider.
// defaultKey is the provider key a bare #n refers to; when it names no repo
// (a bare "github") bare references are not sent. A reference the provider
// cannot be asked about (number 0, an invalid repo, another provider) is
// skipped, never reported as missing; only a queried reference the provider
// does not know is "missing". The status line says what was skipped, or why
// nothing could be checked.
func CheckRequests(ctx context.Context, p WorkProvider, defaultKey string, refs []DocRef) ([]RequestCheck, string) {
	var reqs []DocRef
	for _, r := range refs {
		if r.Kind == "request" {
			reqs = append(reqs, r)
		}
	}
	if len(reqs) == 0 {
		return nil, ""
	}
	reader, ok := p.(IssueStateReader)
	if !ok {
		return nil, "request checks skipped: this provider cannot look up requests"
	}
	var asked []DocRef
	var asks []Ref
	skipped := 0
	for _, r := range reqs {
		key := defaultKey
		if r.Repo != "" {
			key = "github:" + r.Repo
		}
		owner, name, okRepo := strings.Cut(RepoFromProviderKey(key), "/")
		if r.Number <= 0 || !okRepo || !repoPart.MatchString(owner) || !repoPart.MatchString(name) {
			skipped++
			continue
		}
		asked = append(asked, r)
		asks = append(asks, Ref{Provider: key, ID: strconv.Itoa(r.Number)})
	}
	note := ""
	if skipped > 0 {
		note = strconv.Itoa(skipped) + " reference(s) skipped: repository not resolved or not a valid issue number"
	}
	if len(asks) == 0 {
		return nil, "request checks skipped: " + note
	}
	states, err := reader.IssueStates(ctx, asks)
	if err != nil {
		return nil, "request checks skipped: " + err.Error()
	}
	out := make([]RequestCheck, 0, len(asked))
	for i, r := range asked {
		st := states[IssueStateKey(asks[i])]
		c := RequestCheck{Ref: r, State: st.State, Reason: st.Reason}
		if st.State == "unknown" || st.State == "" {
			c.State, c.Reason = "missing", ""
		}
		out = append(out, c)
	}
	if note != "" {
		note = "request checks partly skipped: " + note
	}
	return out, note
}
