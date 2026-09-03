package specsync

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// PRState holds the state of a single pull request.
type PRState struct {
	Number      int    `json:"number"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	HeadRefName string `json:"headRefName"`
	Body        string `json:"body"`
	Merged      bool   `json:"merged"`
}

// GitHubProvider projects changes onto GitHub Issues using the `gh` CLI. It
// holds no GitHub SDK dependency; everything is shelled out, which keeps this
// package free of network/auth code and easy to fake in tests by swapping run.
type GitHubProvider struct {
	resolved ResolvedRepo // resolved repo and rule; zero = not yet resolved
	explicit string       // -repo flag; empty if not set
	// run executes gh and returns trimmed stdout. Overridable in tests.
	run func(ctx context.Context, args ...string) (string, error)

	// nameOnce memoizes the canonical cache key so Name() resolves the concrete
	// repo (auto-detect included) exactly once instead of on every call.
	nameOnce sync.Once
	name     string

	// boardMu guards boardCache, which memoizes a target board's resolved node id
	// and Status schema per run so repeated projections don't re-query the schema.
	boardMu    sync.Mutex
	boardCache map[string]*boardSchema
}

// NewGitHubProvider returns a provider that drives the real `gh` binary,
// resolving the target repo explicitly (explicit flag → gh-set-default → origin).
func NewGitHubProvider() *GitHubProvider {
	return &GitHubProvider{run: runGH}
}

// NewGitHubProviderWithRepo returns a provider targeting an explicit repo
// ("owner/name") instead of resolving it. The ref cache key becomes
// "github:owner/name" so cross-repo refs coexist in one refs.json.
func NewGitHubProviderWithRepo(repo string) *GitHubProvider {
	return &GitHubProvider{explicit: repo, run: runGH}
}

// NewGitHubProviderFunc returns a provider driven by the given runner instead of
// the real `gh` binary. Used for dry-runs and tests.
func NewGitHubProviderFunc(run func(ctx context.Context, args ...string) (string, error)) *GitHubProvider {
	return &GitHubProvider{run: run}
}

func NewGitHubProviderFuncWithRepo(repo string, run func(ctx context.Context, args ...string) (string, error)) *GitHubProvider {
	return &GitHubProvider{explicit: repo, run: run}
}

func runGH(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "gh", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

// Name is the ref-cache key. It resolves the concrete target repo — from -repo
// or, failing that, the git-remote auto-detect gh itself would use — and keys
// canonically as "github:owner/repo". Two providers aimed at the same repo thus
// share one key, whichever way the repo was supplied, so a ref cached by `pull`
// is found by a later auto-detected `sync`. Resolution is memoized because it
// shells out; when the repo can't be resolved it degrades to the bare "github".
func (p *GitHubProvider) Name() string {
	p.nameOnce.Do(func() {
		p.name = p.resolveKey(context.Background())
	})
	return p.name
}

func (p *GitHubProvider) resolveKey(ctx context.Context) string {
	repo, _ := p.resolveRepo(ctx)
	if repo == "" {
		return "github"
	}
	return "github:" + repo
}

// resolveRepo returns the repo string, resolving explicitly if needed.
func (p *GitHubProvider) resolveRepo(ctx context.Context) (string, RepoRule) {
	if p.resolved.Repo != "" {
		return p.resolved.Repo, p.resolved.Rule
	}
	resolver := NewRepoResolverFunc(p.explicit, p.run)
	resolved, err := resolver.Resolve(ctx)
	if err != nil {
		// Degrade: use gh auto-detect as fallback.
		if out, err2 := p.run(ctx, "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner"); err2 == nil {
			resolved = ResolvedRepo{Repo: strings.TrimSpace(out), Rule: RuleDefault}
		}
	}
	p.resolved = resolved
	return resolved.Repo, resolved.Rule
}

// Resolve resolves the target repo and returns it with the rule that selected it.
// Call this before any write to verify the target.
func (p *GitHubProvider) Resolve(ctx context.Context) (ResolvedRepo, error) {
	repo, rule := p.resolveRepo(ctx)
	return ResolvedRepo{Repo: repo, Rule: rule}, nil
}

// CheckForkDivergence returns (divergent, upstreamRepo) if origin and upstream
// name different repos. Returns false when the user explicitly named the repo
// via -repo (their choice) or when there's no upstream.
func (p *GitHubProvider) CheckForkDivergence(ctx context.Context) (bool, string, error) {
	repo, rule := p.resolveRepo(ctx)
	return IsForkDivergence(ctx, ResolvedRepo{Repo: repo, Rule: rule})
}

// repoFlag returns ["--repo", "owner/name"] with the resolved repo.
// Unlike the old behavior, this always returns a flag — gh never auto-detects.
func (p *GitHubProvider) repoFlag() []string {
	repo, _ := p.resolveRepo(context.Background())
	if repo == "" {
		return nil
	}
	return []string{"--repo", repo}
}

// Get reads an existing issue so it can be pulled into a local change. It
// satisfies the IssueReader capability, enabling the issue-first flow.
func (p *GitHubProvider) Get(ctx context.Context, id string) (FetchedItem, error) {
	args := append([]string{"issue", "view", id}, p.repoFlag()...)
	args = append(args, "--json", "number,url,title,body,state,labels")
	out, err := p.run(ctx, args...)
	if err != nil {
		return FetchedItem{}, err
	}
	var v struct {
		Number int    `json:"number"`
		URL    string `json:"url"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		State  string `json:"state"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return FetchedItem{}, fmt.Errorf("parse gh issue view: %w", err)
	}
	item := FetchedItem{
		ID:     fmt.Sprintf("%d", v.Number),
		URL:    v.URL,
		Title:  v.Title,
		Body:   v.Body,
		Closed: strings.EqualFold(v.State, "closed"),
	}
	for _, l := range v.Labels {
		item.Labels = append(item.Labels, l.Name)
	}
	return item, nil
}

// marker is the durable identity anchor embedded in the issue body. The ref
// cache is only an optimization; this marker lets Find rebuild it from scratch.
func marker(slug string) string { return fmt.Sprintf("<!-- specsync:change=%s -->", slug) }

func (p *GitHubProvider) renderBody(item WorkItem) string {
	return marker(item.Slug) + "\n\n" + item.Body
}

// GitHubBodyLimit is GitHub's issue/PR body size limit, in characters.
const GitHubBodyLimit = 65536

// designCommentMarker is the identity anchor for design.md's overflow
// comment, distinct from the issue-body marker so an idempotent upsert can
// find the same comment again instead of duplicating it.
func designCommentMarker(slug string) string {
	return fmt.Sprintf("<!-- specsync:change=%s:design -->", slug)
}

// designNotesSection is the exact substring WorkItemFor inlines into Body
// when design.md is non-empty, so Push's overflow check is a plain replace.
// Delegates to wrapSection (sync.go) rather than duplicating its format, so
// the two can never drift out of byte-for-byte sync with each other.
func designNotesSection(designNotes string) string {
	return "\n\n" + wrapSection("Design notes", "design-notes", designNotes)
}

// designNotesStub replaces designNotesSection when it overflows to a
// comment. url is "" until the comment exists (the create-new-issue path
// renders it once without a link, then edits the issue again once known).
func designNotesStub(url string) string {
	if url == "" {
		return "\n\n## Design notes\n\n_Too large to inline — design notes comment pending._"
	}
	return "\n\n## Design notes\n\n_Too large to inline — see [design notes comment](" + url + ")._"
}

// designCommentStaleBody is what an overflow comment is rewritten to once
// design.md moves back inline — marked stale, not deleted.
func designCommentStaleBody(slug string) string {
	return designCommentMarker(slug) + "\n\n_Design notes moved back into the issue body; this comment is no longer current._"
}

const designCommentStaleNote = "moved back into the issue body"

const designNotesStubMarker = "Too large to inline"

func isDesignNotesStub(text string) bool {
	return strings.Contains(text, designNotesStubMarker)
}

// ReadDesignNotesComment reads back design.md's overflow comment for
// issueID, satisfying DesignNotesCommentReader. found is false when no such
// comment exists, or it's marked stale (content already moved into the body).
func (p *GitHubProvider) ReadDesignNotesComment(ctx context.Context, issueID, slug string) (string, bool, error) {
	_, _, body, found, err := p.findDesignComment(ctx, issueID, slug)
	if err != nil || !found {
		return "", false, err
	}
	if strings.Contains(body, designCommentStaleNote) {
		return "", false, nil
	}
	content := strings.TrimSpace(strings.TrimPrefix(body, designCommentMarker(slug)))
	if content != "" {
		content += "\n"
	}
	return content, true, nil
}

// findDesignComment locates the marked design-notes comment on issue num,
// returning its GraphQL node id (for in-place edits), URL, and body.
func (p *GitHubProvider) findDesignComment(ctx context.Context, num, slug string) (id, url, body string, found bool, err error) {
	args := append([]string{"issue", "view", num}, p.repoFlag()...)
	args = append(args, "--json", "comments")
	out, err := p.run(ctx, args...)
	if err != nil {
		return "", "", "", false, err
	}
	var v struct {
		Comments []struct {
			ID   string `json:"id"`
			URL  string `json:"url"`
			Body string `json:"body"`
		} `json:"comments"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return "", "", "", false, fmt.Errorf("parse issue comments: %w", err)
	}
	want := designCommentMarker(slug)
	for _, c := range v.Comments {
		if strings.Contains(c.Body, want) {
			return c.ID, c.URL, c.Body, true, nil
		}
	}
	return "", "", "", false, nil
}

// upsertDesignComment creates or updates the marked design-notes comment on
// issue num, returning its URL. Idempotent: a re-sync with the same content
// finds and edits the same comment via its marker rather than creating a
// duplicate.
func (p *GitHubProvider) upsertDesignComment(ctx context.Context, num, slug, content string) (string, error) {
	body := designCommentMarker(slug) + "\n\n" + content
	id, url, _, found, err := p.findDesignComment(ctx, num, slug)
	if err != nil {
		return "", err
	}
	if found {
		if err := p.updateIssueComment(ctx, id, body); err != nil {
			return "", err
		}
		return url, nil
	}
	args := append([]string{"issue", "comment", num}, p.repoFlag()...)
	args = append(args, "--body", body)
	return p.run(ctx, args...)
}

// markDesignCommentStale rewrites an overflow comment once to note that
// design.md moved back inline; a no-op if already marked.
func (p *GitHubProvider) markDesignCommentStale(ctx context.Context, num, slug string) error {
	id, _, body, found, err := p.findDesignComment(ctx, num, slug)
	if err != nil || !found {
		return err
	}
	if strings.Contains(body, designCommentStaleNote) {
		return nil
	}
	return p.updateIssueComment(ctx, id, designCommentStaleBody(slug))
}

// updateIssueComment edits an existing issue comment in place by its GraphQL
// node id.
func (p *GitHubProvider) updateIssueComment(ctx context.Context, nodeID, body string) error {
	mutation := `
		mutation($id: ID!, $body: String!) {
			updateIssueComment(input: {id: $id, body: $body}) {
				clientMutationId
			}
		}
	`
	return p.graphql(ctx, "updateIssueComment", mutation, nil, "-f", "id="+nodeID, "-f", "body="+body)
}

// syncDesignComment upserts the design-notes overflow comment on issue num
// and patches the placeholder stub in *body with the comment's real link,
// now that its URL is known.
func (p *GitHubProvider) syncDesignComment(ctx context.Context, num string, item WorkItem, body *string) error {
	url, err := p.upsertDesignComment(ctx, num, item.Slug, item.DesignNotes)
	if err != nil {
		return err
	}
	*body = strings.Replace(*body, designNotesStub(""), designNotesStub(url), 1)
	return nil
}

// EnsureMarker upserts the identity marker into issue id's body so the link
// survives loss of the local ref cache: a later sync rediscovers the issue via
// Find. Idempotent — a body already carrying the marker is left untouched and no
// gh write happens. It satisfies the IssueMarkerWriter capability used by pull.
func (p *GitHubProvider) EnsureMarker(ctx context.Context, id, slug, body string) (bool, error) {
	if strings.Contains(body, marker(slug)) {
		return false, nil
	}
	args := append([]string{"issue", "edit", id}, p.repoFlag()...)
	args = append(args, "--body", marker(slug)+"\n\n"+body)
	if _, err := p.run(ctx, args...); err != nil {
		return false, err
	}
	return true, nil
}

func (p *GitHubProvider) Push(ctx context.Context, item WorkItem, existing *Ref) (Ref, error) {
	labels := desiredLabels(item)
	if err := p.EnsureLabels(ctx, labels); err != nil {
		return Ref{}, err
	}
	body := p.renderBody(item)

	// Oversized design.md moves to a linked comment instead of the body.
	designOverflow := strings.TrimSpace(item.DesignNotes) != "" && len(body) > GitHubBodyLimit
	if designOverflow {
		body = strings.Replace(body, designNotesSection(item.DesignNotes), designNotesStub(""), 1)
	}

	// Defend against duplicates: if we have no cached ref, look one up by
	// marker, retrying briefly in case a very-recently-created issue (by
	// another cache-less run, e.g. a prior CI run for the same push burst)
	// hasn't reached GitHub's search index yet.
	if existing == nil {
		found, err := findWithRetry(ctx, func(ctx context.Context) (*Ref, error) { return p.Find(ctx, item.Slug) })
		if err != nil {
			return Ref{}, err
		}
		existing = found
	}

	if existing == nil {
		args := append([]string{"issue", "create"}, p.repoFlag()...)
		args = append(args, "--title", item.Title, "--body", body)
		for _, l := range labels {
			args = append(args, "--label", l)
		}
		url, err := p.run(ctx, args...)
		if err != nil {
			return Ref{}, err
		}
		ref := Ref{Provider: p.Name(), ID: numberFromURL(url), URL: url}
		if designOverflow {
			if err := p.syncDesignComment(ctx, ref.ID, item, &body); err != nil {
				return Ref{}, err
			}
			editArgs := append([]string{"issue", "edit", ref.ID}, p.repoFlag()...)
			editArgs = append(editArgs, "--body", body)
			if _, err := p.run(ctx, editArgs...); err != nil {
				return Ref{}, err
			}
		}
		if item.Closed {
			ref.BaseClosed = boolPtr(true)
			return ref, p.close(ctx, ref.ID)
		}
		if item.ManageClosed {
			ref.BaseClosed = boolPtr(false)
		}
		return ref, nil
	}

	num := existing.ID
	if designOverflow {
		if err := p.syncDesignComment(ctx, num, item, &body); err != nil {
			return Ref{}, err
		}
	} else if strings.TrimSpace(item.DesignNotes) != "" {
		// Fits inline again; mark any leftover overflow comment stale.
		if err := p.markDesignCommentStale(ctx, num, item.Slug); err != nil {
			return Ref{}, err
		}
	}
	args := append([]string{"issue", "edit", num}, p.repoFlag()...)
	args = append(args, "--title", item.Title, "--body", body)
	add, remove, currentlyClosed, err := p.labelDelta(ctx, num, labels)
	if err != nil {
		return Ref{}, err
	}
	for _, l := range add {
		args = append(args, "--add-label", l)
	}
	for _, l := range remove {
		args = append(args, "--remove-label", l)
	}
	if _, err := p.run(ctx, args...); err != nil {
		return Ref{}, err
	}
	ref := *existing
	if !item.ManageClosed {
		return ref, nil
	}
	switch {
	case item.Closed && !currentlyClosed:
		ref.BaseClosed = boolPtr(true)
		return ref, p.close(ctx, num)

	case !item.Closed && currentlyClosed:
		// Three-way merge on open/closed state, mirroring the board's human-move
		// detection. specsync may only undo a close it made itself: with a base of
		// true, "closed" is specsync's own last assertion and local work has since
		// reappeared, which is what reopen is for. With a base of false or nil the
		// close came from outside — a merged PR, a human, a reviewing agent — and
		// specsync is glue, not a second authority on it. Reopening there is the
		// clobber: it would undo a deliberate close on the next unrelated spec push.
		//
		// Note the asymmetry: an external close is never adopted as the new base, so
		// specsync stays deferential from then on rather than re-arming itself to
		// reopen later. Whoever took over the state keeps it until specsync closes
		// the item again on its own.
		if existing.BaseClosed == nil || !*existing.BaseClosed {
			reason := "never asserted"
			if existing.BaseClosed != nil && !*existing.BaseClosed {
				reason = "externally closed"
			}
			ref.ClosedDeferred = reason
			return ref, nil
		}
		ref.BaseClosed = boolPtr(false)
		return ref, p.reopen(ctx, num)

	default:
		// Remote already matches the desired state; record it as the base so a
		// later divergence is measured against something.
		ref.BaseClosed = boolPtr(item.Closed)
		return ref, nil
	}
}

func boolPtr(b bool) *bool { return &b }

func (p *GitHubProvider) Find(ctx context.Context, slug string) (*Ref, error) {
	// Search the inner token (not the full HTML comment) for friendlier indexing.
	search := fmt.Sprintf("specsync:change=%s in:body", slug)
	args := append([]string{"issue", "list"}, p.repoFlag()...)
	args = append(args, "--state", "all", "--search", search, "--json", "number,url,body", "--limit", "30")
	out, err := p.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	if out == "" || out == "[]" {
		return nil, nil
	}
	var items []struct {
		Number int    `json:"number"`
		URL    string `json:"url"`
		Body   string `json:"body"`
	}
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		return nil, fmt.Errorf("parse gh issue list: %w", err)
	}
	want := marker(slug)
	for _, it := range items {
		if strings.Contains(it.Body, want) {
			return &Ref{Provider: p.Name(), ID: fmt.Sprintf("%d", it.Number), URL: it.URL}, nil
		}
	}
	return nil, nil
}

// SearchOpenIssues finds open issues matching a free-text query, satisfying the
// IssueSearcher capability used by `scan`.
func (p *GitHubProvider) SearchOpenIssues(ctx context.Context, query string) ([]FetchedItem, error) {
	args := append([]string{"issue", "list"}, p.repoFlag()...)
	args = append(args, "--state", "open", "--search", query, "--json", "number,title,url,body", "--limit", "50")
	out, err := p.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	if out == "" || out == "[]" {
		return nil, nil
	}
	var items []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		URL    string `json:"url"`
		Body   string `json:"body"`
	}
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		return nil, fmt.Errorf("parse gh issue list: %w", err)
	}
	var out2 []FetchedItem
	for _, it := range items {
		out2 = append(out2, FetchedItem{
			ID:    fmt.Sprintf("%d", it.Number),
			Title: it.Title,
			URL:   it.URL,
			Body:  it.Body,
		})
	}
	return out2, nil
}

// ListOpenPRs returns open pull requests from the target repo.
func (p *GitHubProvider) ListOpenPRs(ctx context.Context) ([]PRState, error) {
	args := append([]string{"pr", "list"}, p.repoFlag()...)
	args = append(args, "--state", "open", "--json", "number,url,title,headRefName,body")
	out, err := p.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	if out == "" || out == "[]" {
		return nil, nil
	}
	var items []PRState
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		return nil, fmt.Errorf("parse gh pr list: %w", err)
	}
	return items, nil
}

// ListRecentMergedPRs returns recently merged pull requests from the target repo.
func (p *GitHubProvider) ListRecentMergedPRs(ctx context.Context) ([]PRState, error) {
	args := append([]string{"pr", "list"}, p.repoFlag()...)
	args = append(args, "--state", "merged", "--limit", "50", "--json", "number,url,title,headRefName,body")
	out, err := p.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	if out == "" || out == "[]" {
		return nil, nil
	}
	var items []PRState
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		return nil, fmt.Errorf("parse gh pr list: %w", err)
	}
	for i := range items {
		items[i].Merged = true
	}
	return items, nil
}

func (p *GitHubProvider) close(ctx context.Context, num string) error {
	args := append([]string{"issue", "close", num}, p.repoFlag()...)
	_, err := p.run(ctx, args...)
	return err
}

func (p *GitHubProvider) reopen(ctx context.Context, num string) error {
	args := append([]string{"issue", "reopen", num}, p.repoFlag()...)
	_, err := p.run(ctx, args...)
	return err
}

// EnsureLabels makes every desired label exist. --force is idempotent: it
// creates the label or updates it if present.
func (p *GitHubProvider) EnsureLabels(ctx context.Context, labels []string) error {
	for _, l := range labels {
		args := append([]string{"label", "create", l, "--force"}, p.repoFlag()...)
		if _, err := p.run(ctx, args...); err != nil {
			return err
		}
	}
	return nil
}

// labelDelta computes which managed labels to add/remove so the issue ends up
// with exactly the desired set. Labels outside our namespace are left alone.
func (p *GitHubProvider) labelDelta(ctx context.Context, num string, desired []string) (add, remove []string, closed bool, err error) {
	args := append([]string{"issue", "view", num}, p.repoFlag()...)
	args = append(args, "--json", "labels,state")
	out, err := p.run(ctx, args...)
	if err != nil {
		return nil, nil, false, err
	}
	var v struct {
		State  string `json:"state"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return nil, nil, false, fmt.Errorf("parse labels: %w", err)
	}
	current := map[string]bool{}
	for _, l := range v.Labels {
		current[l.Name] = true
	}
	want := map[string]bool{}
	for _, l := range desired {
		want[l] = true
		if !current[l] {
			add = append(add, l)
		}
	}
	for name := range current {
		if !want[name] && managedLabel(name) {
			remove = append(remove, name)
		}
	}
	return add, remove, strings.EqualFold(v.State, "closed"), nil
}

// ApplyLabelDelta adds and removes labels on an existing issue using gh CLI.
func (p *GitHubProvider) ApplyLabelDelta(ctx context.Context, num string, add, remove []string) error {
	for _, l := range remove {
		args := append([]string{"issue", "edit", num}, p.repoFlag()...)
		args = append(args, "--remove-label", l)
		if _, err := p.run(ctx, args...); err != nil {
			return fmt.Errorf("remove label %s: %w", l, err)
		}
	}
	for _, l := range add {
		args := append([]string{"issue", "edit", num}, p.repoFlag()...)
		args = append(args, "--add-label", l)
		if _, err := p.run(ctx, args...); err != nil {
			return fmt.Errorf("add label %s: %w", l, err)
		}
	}
	return nil
}

func desiredLabels(item WorkItem) []string {
	if item.Labels != nil {
		return item.Labels
	}
	labels := []string{"specsync", "stage:" + string(item.Stage)}
	if item.Priority > 0 {
		labels = append(labels, fmt.Sprintf("priority:%d", item.Priority))
	}
	return labels
}

// managedLabel reports whether a label is owned by specsync and therefore safe
// to reconcile (add/remove) on updates.
func managedLabel(name string) bool {
	return name == "specsync" ||
		strings.HasPrefix(name, "stage:") ||
		strings.HasPrefix(name, "priority:")
}

func numberFromURL(url string) string {
	if i := strings.LastIndex(url, "/"); i >= 0 {
		return url[i+1:]
	}
	return url
}

// ReferenceLine returns the PR-body reference line for the change's tracker item.
// When allTasksComplete is true the line upgrades to "Closes #N"; otherwise it
// stays "Part of #N". The number comes from ref.ID (the issue/PR number).
func (p *GitHubProvider) ReferenceLine(ref Ref, allTasksComplete bool) string {
	if ref.ID == "" {
		return ""
	}
	if allTasksComplete {
		return fmt.Sprintf("Closes #%s", ref.ID)
	}
	return fmt.Sprintf("Part of #%s", ref.ID)
}

// ReadDependencies queries GitHub for the blockedBy and blocking edges on the
// issue identified by ref. It returns the list of edges. This is the source of
// truth for dependency reconciliation on GitHub.
func (p *GitHubProvider) ReadDependencies(ctx context.Context, ref Ref) ([]DependencyEdge, error) {
	owner, repo, number, err := parseIssueURL(ref.URL)
	if err != nil {
		return nil, err
	}

	query := `
		query($owner: String!, $repo: String!, $number: Int!) {
			repository(owner: $owner, name: $repo) {
				issue(number: $number) {
					issueDependenciesSummary {
						blockedBy {
							edges {
								node {
									... on Issue {
										id
										databaseId
										url
									}
								}
							}
						}
						blocking {
							edges {
								node {
									... on Issue {
										id
										databaseId
										url
									}
								}
							}
						}
					}
				}
			}
		}
	`

	var out struct {
		Repository struct {
			Issue struct {
				IssueDependenciesSummary struct {
					BlockedBy struct {
						Edges []struct {
							Node struct {
								ID         string `json:"id"`
								DatabaseID int    `json:"databaseId"`
								URL        string `json:"url"`
							} `json:"node"`
						} `json:"edges"`
					} `json:"blockedBy"`
					Blocking struct {
						Edges []struct {
							Node struct {
								ID         string `json:"id"`
								DatabaseID int    `json:"databaseId"`
								URL        string `json:"url"`
							} `json:"node"`
						} `json:"edges"`
					} `json:"blocking"`
				} `json:"issueDependenciesSummary"`
			} `json:"issue"`
		} `json:"repository"`
	}

	if err := p.graphql(ctx, "readDependencies", query, &out,
		"-f", "owner="+owner,
		"-f", "repo="+repo,
		"-F", "number="+fmt.Sprintf("%d", number),
	); err != nil {
		return nil, err
	}

	var edges []DependencyEdge
	for _, e := range out.Repository.Issue.IssueDependenciesSummary.BlockedBy.Edges {
		r := *refFromURL(e.Node.URL)
		edges = append(edges, DependencyEdge{
			Ref:    r,
			NodeID: e.Node.ID,
		})
	}
	for _, e := range out.Repository.Issue.IssueDependenciesSummary.Blocking.Edges {
		r := *refFromURL(e.Node.URL)
		edges = append(edges, DependencyEdge{
			Ref:      r,
			NodeID:   e.Node.ID,
			IsBlocks: true,
		})
	}
	return edges, nil
}

// ReadDependenciesForRef reads the blockedBy edges for a specific ref
// (used to check the inverse edge for "## Blocks").
func (p *GitHubProvider) ReadDependenciesForRef(ctx context.Context, ref Ref) ([]DependencyEdge, error) {
	edges, err := p.ReadDependencies(ctx, ref)
	if err != nil {
		return nil, err
	}
	// Only return blockedBy edges (not blocks).
	var blockedBy []DependencyEdge
	for _, e := range edges {
		if !e.IsBlocks {
			blockedBy = append(blockedBy, e)
		}
	}
	return blockedBy, nil
}

// ResolveNodeID returns the GitHub node id for the issue at url.
// The node id is required for addBlockedBy / removeBlockedBy mutations.
func (p *GitHubProvider) ResolveNodeID(ctx context.Context, url string) (string, error) {
	owner, repo, number, err := parseIssueURL(url)
	if err != nil {
		return "", err
	}

	query := `
		query($owner: String!, $repo: String!, $number: Int!) {
			repository(owner: $owner, name: $repo) {
				issue(number: $number) {
					id
				}
			}
		}
	`

	var out struct {
		Repository struct {
			Issue struct {
				ID string `json:"id"`
			} `json:"issue"`
		} `json:"repository"`
	}

	if err := p.graphql(ctx, "resolveNodeID", query, &out,
		"-f", "owner="+owner,
		"-f", "repo="+repo,
		"-F", "number="+fmt.Sprintf("%d", number),
	); err != nil {
		return "", err
	}
	return out.Repository.Issue.ID, nil
}

// AddBlockedBy adds a dependency edge: the issue with number issueNum is
// blocked by the issue with nodeID.
func (p *GitHubProvider) AddBlockedBy(ctx context.Context, issueNum, blockedByNodeID string) error {
	mutation := `
		mutation($issueId: ID!, $blockedById: ID!) {
			addBlockedBy(input: {issueId: $issueId, blockedById: $blockedById}) {
				clientMutationId
			}
		}
	`
	return p.graphql(ctx, "addBlockedBy", mutation, nil,
		"-f", "issueId="+issueNum,
		"-f", "blockedById="+blockedByNodeID,
	)
}

// RemoveBlockedBy removes a dependency edge: the issue with number issueNum
// is no longer blocked by the issue with nodeID.
func (p *GitHubProvider) RemoveBlockedBy(ctx context.Context, issueNum, blockedByNodeID string) error {
	mutation := `
		mutation($issueId: ID!, $blockedById: ID!) {
			removeBlockedBy(input: {issueId: $issueId, blockedById: $blockedById}) {
				clientMutationId
			}
		}
	`
	return p.graphql(ctx, "removeBlockedBy", mutation, nil,
		"-f", "issueId="+issueNum,
		"-f", "blockedById="+blockedByNodeID,
	)
}
