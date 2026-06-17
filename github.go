package specsync

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// GitHubProvider projects changes onto GitHub Issues using the `gh` CLI. It
// holds no GitHub SDK dependency; everything is shelled out, which keeps this
// package free of network/auth code and easy to fake in tests by swapping run.
type GitHubProvider struct {
	// run executes gh and returns trimmed stdout. Overridable in tests.
	run func(ctx context.Context, args ...string) (string, error)
}

// NewGitHubProvider returns a provider that drives the real `gh` binary.
func NewGitHubProvider() *GitHubProvider {
	return &GitHubProvider{run: runGH}
}

// NewGitHubProviderFunc returns a provider driven by the given runner instead of
// the real `gh` binary. Used for dry-runs and tests.
func NewGitHubProviderFunc(run func(ctx context.Context, args ...string) (string, error)) *GitHubProvider {
	return &GitHubProvider{run: run}
}

func runGH(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "gh", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func (p *GitHubProvider) Name() string { return "github" }

// Get reads an existing issue so it can be pulled into a local change. It
// satisfies the IssueReader capability, enabling the issue-first flow.
func (p *GitHubProvider) Get(ctx context.Context, id string) (FetchedItem, error) {
	out, err := p.run(ctx, "issue", "view", id, "--json", "number,url,title,body,state,labels")
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

func (p *GitHubProvider) Push(ctx context.Context, item WorkItem, existing *Ref) (Ref, error) {
	labels := desiredLabels(item)
	if err := p.ensureLabels(ctx, labels); err != nil {
		return Ref{}, err
	}
	body := p.renderBody(item)

	// Defend against duplicates: if we have no cached ref, look one up by marker.
	if existing == nil {
		found, err := p.Find(ctx, item.Slug)
		if err != nil {
			return Ref{}, err
		}
		existing = found
	}

	if existing == nil {
		args := []string{"issue", "create", "--title", item.Title, "--body", body}
		for _, l := range labels {
			args = append(args, "--label", l)
		}
		url, err := p.run(ctx, args...)
		if err != nil {
			return Ref{}, err
		}
		ref := Ref{Provider: p.Name(), ID: numberFromURL(url), URL: url}
		if item.Closed {
			return ref, p.close(ctx, ref.ID)
		}
		return ref, nil
	}

	num := existing.ID
	args := []string{"issue", "edit", num, "--title", item.Title, "--body", body}
	add, remove, err := p.labelDelta(ctx, num, labels)
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
	if item.Closed {
		return *existing, p.close(ctx, num)
	}
	return *existing, nil
}

func (p *GitHubProvider) Find(ctx context.Context, slug string) (*Ref, error) {
	// Search the inner token (not the full HTML comment) for friendlier indexing.
	search := fmt.Sprintf("specsync:change=%s in:body", slug)
	out, err := p.run(ctx, "issue", "list", "--state", "all",
		"--search", search, "--json", "number,url,body", "--limit", "30")
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

func (p *GitHubProvider) close(ctx context.Context, num string) error {
	_, err := p.run(ctx, "issue", "close", num)
	return err
}

// ensureLabels makes every desired label exist. --force is idempotent: it
// creates the label or updates it if present.
func (p *GitHubProvider) ensureLabels(ctx context.Context, labels []string) error {
	for _, l := range labels {
		if _, err := p.run(ctx, "label", "create", l, "--force"); err != nil {
			return err
		}
	}
	return nil
}

// labelDelta computes which managed labels to add/remove so the issue ends up
// with exactly the desired set. Labels outside our namespace are left alone.
func (p *GitHubProvider) labelDelta(ctx context.Context, num string, desired []string) (add, remove []string, err error) {
	out, err := p.run(ctx, "issue", "view", num, "--json", "labels")
	if err != nil {
		return nil, nil, err
	}
	var v struct {
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return nil, nil, fmt.Errorf("parse labels: %w", err)
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
	return add, remove, nil
}

func desiredLabels(item WorkItem) []string {
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
