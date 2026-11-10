package specsync

import (
	"context"
	"regexp"
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
	codeSpan       = regexp.MustCompile("`([^`\n]+)`")
	commitWord     = regexp.MustCompile(`(?i)\bcommits?\s+([0-9a-f]{7,40})\b`)
	hexHash        = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)
	urlText        = regexp.MustCompile(`https?://\S+`)
	qualifiedReq   = regexp.MustCompile(`(?:^|[^\w./-])([A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9._-]+)#([0-9]+)\b`)
	bareReq        = regexp.MustCompile(`(?:^|[\s(\[,;:])#([0-9]+)\b`)
	hasDigitRegexp = regexp.MustCompile(`[0-9]`)
)

// ScanDocRefs finds the commit hashes and request references in text. It
// favours precision: a hash must be 7 to 40 hex characters with at least one
// digit and be written as inline code or after the word "commit"; fenced code
// blocks and URLs are ignored.
func ScanDocRefs(file, text string) []DocRef {
	var refs []DocRef
	inFence := false
	for i, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		n := i + 1
		seen := map[string]bool{}
		addCommit := func(h string) {
			if !hasDigitRegexp.MatchString(h) || seen[h] {
				return
			}
			seen[h] = true
			refs = append(refs, DocRef{File: file, Line: n, Kind: "commit", Text: h})
		}
		type found struct {
			pos  int
			hash string
		}
		var commits []found
		for _, m := range codeSpan.FindAllStringSubmatchIndex(line, -1) {
			if h := line[m[2]:m[3]]; hexHash.MatchString(h) {
				commits = append(commits, found{m[0], h})
			}
		}
		for _, m := range commitWord.FindAllStringSubmatchIndex(line, -1) {
			commits = append(commits, found{m[0], line[m[2]:m[3]]})
		}
		for len(commits) > 0 {
			best := 0
			for j, c := range commits {
				if c.pos < commits[best].pos {
					best = j
				}
			}
			addCommit(commits[best].hash)
			commits = append(commits[:best], commits[best+1:]...)
		}

		plain := urlText.ReplaceAllString(line, " ")
		type reqAt struct {
			pos int
			ref DocRef
		}
		var reqs []reqAt
		for _, m := range qualifiedReq.FindAllStringSubmatchIndex(plain, -1) {
			num, _ := strconv.Atoi(plain[m[4]:m[5]])
			repo := plain[m[2]:m[3]]
			reqs = append(reqs, reqAt{m[2], DocRef{File: file, Line: n, Kind: "request", Text: repo + "#" + plain[m[4]:m[5]], Repo: repo, Number: num}})
		}
		for _, m := range bareReq.FindAllStringSubmatchIndex(plain, -1) {
			num, _ := strconv.Atoi(plain[m[2]:m[3]])
			reqs = append(reqs, reqAt{m[2], DocRef{File: file, Line: n, Kind: "request", Text: "#" + plain[m[2]:m[3]], Number: num}})
		}
		for len(reqs) > 0 {
			best := 0
			for j, r := range reqs {
				if r.pos < reqs[best].pos {
					best = j
				}
			}
			refs = append(refs, reqs[best].ref)
			reqs = append(reqs[:best], reqs[best+1:]...)
		}
	}
	return refs
}

// CommitCheck is the outcome of resolving one commit reference.
type CommitCheck struct {
	Ref     DocRef
	Status  string // "ok", "stale" (does not resolve) or "orphaned" (resolves, on no checked ref)
	Subject string // the commit subject, for an orphaned hash
}

// CheckCommits resolves every commit reference in dir with git only. A hash that
// does not resolve (or is ambiguous) is stale. One that resolves but is an
// ancestor of none of reachable is orphaned. With no reachable refs, every
// resolving hash is ok. Other kinds of reference are skipped.
func CheckCommits(ctx context.Context, dir string, refs []DocRef, reachable []string) []CommitCheck {
	var out []CommitCheck
	for _, r := range refs {
		if r.Kind != "commit" {
			continue
		}
		c := CommitCheck{Ref: r, Status: "stale"}
		sha, err := gitIn(ctx, dir, "rev-parse", "--verify", "--quiet", r.Text+"^{commit}")
		if err == nil && sha != "" {
			c.Status = "ok"
			if len(reachable) > 0 {
				c.Status = "orphaned"
				for _, ref := range reachable {
					if _, err := gitIn(ctx, dir, "merge-base", "--is-ancestor", sha, ref); err == nil {
						c.Status = "ok"
						break
					}
				}
			}
			if c.Status == "orphaned" {
				c.Subject, _ = gitIn(ctx, dir, "show", "-s", "--format=%s", sha)
			}
		}
		out = append(out, c)
	}
	return out
}

// DocRefReachableRefs lists the refs a cited commit may legitimately be on:
// HEAD, the default branch, and any branch named for the change.
func DocRefReachableRefs(ctx context.Context, dir, slug string) []string {
	var refs []string
	add := func(ref string) {
		if _, err := gitIn(ctx, dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err == nil {
			refs = append(refs, ref)
		}
	}
	add("HEAD")
	if out, err := gitIn(ctx, dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		add(strings.TrimSpace(out))
	}
	for _, b := range []string{"main", "master"} {
		add(b)
	}
	if slug != "" {
		out, err := gitIn(ctx, dir, "for-each-ref", "--format=%(refname:short)", "refs/heads", "refs/remotes")
		if err == nil {
			for _, name := range strings.Split(out, "\n") {
				if name == slug || strings.HasSuffix(name, "/"+slug) || strings.HasSuffix(name, "-"+slug) {
					add(name)
				}
			}
		}
	}
	return refs
}

// RequestCheck is the outcome of looking up one request or issue reference.
type RequestCheck struct {
	Ref    DocRef
	State  string // "open", "closed", "merged" or "missing"
	Reason string
}

// CheckRequests looks up request and issue references through the provider.
// defaultKey is the provider key a bare #n refers to. When the provider cannot
// answer it returns nil and a status line saying the check was skipped.
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
	keyOf := func(r DocRef) Ref {
		key := defaultKey
		if r.Repo != "" {
			key = "github:" + r.Repo
		}
		return Ref{Provider: key, ID: strconv.Itoa(r.Number)}
	}
	var asks []Ref
	for _, r := range reqs {
		asks = append(asks, keyOf(r))
	}
	states, err := reader.IssueStates(ctx, asks)
	if err != nil {
		return nil, "request checks skipped: " + err.Error()
	}
	out := make([]RequestCheck, 0, len(reqs))
	for _, r := range reqs {
		st, found := states[IssueStateKey(keyOf(r))]
		c := RequestCheck{Ref: r, State: st.State, Reason: st.Reason}
		if !found || st.State == "unknown" || st.State == "" {
			c.State, c.Reason = "missing", ""
		}
		out = append(out, c)
	}
	return out, ""
}
