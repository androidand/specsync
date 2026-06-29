package specsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// BeadsProvider projects changes onto a Beads (`bd`) dependency graph: one epic
// bead per change, one child bead per task. Beads is a tracker like GitHub — a
// projection target some projects use — and specsync targets it on the same
// tracker-agnostic footing, nothing more. It carries no library dependency;
// everything shells out to `bd`, keeping the package std-lib-only and letting
// tests swap run with a fake.
//
// This provider syncs TASKS only. Beads' long-term memory (`bd remember` /
// `bd prime`) is the tool's own concern, handled by its session hook outside
// specsync — never read or written here.
//
// Identity mirrors GitHub's exactly: the durable `<!-- specsync:change=<slug> -->`
// marker (the shared marker func) is written into every bead's description —
// epic and children alike — so the whole family is findable with
// `bd list --desc-contains` and the local ref cache stays a disposable
// optimization. Tasks map to child beads by normalized title text, the same key
// reconcile merges on, so an agent closing a child bead flips the matching
// tasks.md checkbox on the next sync.
//
// OpenSpec is the single source of truth: it owns task existence and wording
// (Push creates beads, never re-titles them). Done-state read from the beads
// merges back into tasks.md by the same monotonic union every provider uses.
// specsync only synchronizes the correspondence — it is glue, not a control
// plane, and not a second authority.
type BeadsProvider struct {
	// run executes bd and returns trimmed stdout. Overridable in tests.
	run func(ctx context.Context, args ...string) (string, error)
}

// NewBeadsProvider returns a provider driving the real `bd` binary.
func NewBeadsProvider() *BeadsProvider { return &BeadsProvider{run: runBD} }

// NewBeadsProviderFunc returns a provider driven by the given runner instead of
// the real `bd` binary. Used for dry-runs and tests.
func NewBeadsProviderFunc(run func(ctx context.Context, args ...string) (string, error)) *BeadsProvider {
	return &BeadsProvider{run: run}
}

func runBD(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "bd", args...).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("bd %s: %w\n%s", strings.Join(args, " "), err, ee.Stderr)
		}
		return "", fmt.Errorf("bd %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (p *BeadsProvider) Name() string { return "beads" }

// bead is the subset of `bd ... --json` output specsync reads.
type bead struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	IssueType   string `json:"issue_type"`
}

func (b bead) closed() bool { return strings.EqualFold(b.Status, "closed") }
func (b bead) isEpic() bool { return strings.EqualFold(b.IssueType, "epic") }

// beadURL is the human-facing reference for a bead. Beads is local-first with no
// web URL, so we use a bd:// scheme over the id; it reads clearly in sync output
// and never collides with a GitHub URL.
func beadURL(id string) string { return "bd://" + id }

// family lists every bead carrying this change's identity marker. The
// server-side --desc-contains filter narrows by the inner token; the exact
// marker match in Go then rejects prefix collisions (slug vs slug-v2), mirroring
// the GitHub Find guard.
func (p *BeadsProvider) family(ctx context.Context, slug string) ([]bead, error) {
	out, err := p.run(ctx, "list", "--all", "--json", "--no-pager", "--desc-contains", "specsync:change="+slug)
	if err != nil {
		return nil, err
	}
	if out == "" || out == "[]" {
		return nil, nil
	}
	var beads []bead
	if err := json.Unmarshal([]byte(out), &beads); err != nil {
		return nil, fmt.Errorf("parse bd list: %w", err)
	}
	want := marker(slug)
	var matched []bead
	for _, b := range beads {
		if strings.Contains(b.Description, want) {
			matched = append(matched, b)
		}
	}
	return matched, nil
}

// Find locates the change's epic bead (the ref anchor) via the identity marker.
func (p *BeadsProvider) Find(ctx context.Context, slug string) (*Ref, error) {
	fam, err := p.family(ctx, slug)
	if err != nil {
		return nil, err
	}
	for _, b := range fam {
		if b.isEpic() {
			return &Ref{Provider: p.Name(), ID: b.ID, URL: beadURL(b.ID)}, nil
		}
	}
	return nil, nil
}

// Get reads a bead so it can be projected into a local change (IssueReader). bd
// show returns a one-element array even for a single id.
func (p *BeadsProvider) Get(ctx context.Context, id string) (FetchedItem, error) {
	out, err := p.run(ctx, "show", id, "--json")
	if err != nil {
		return FetchedItem{}, err
	}
	var beads []bead
	if err := json.Unmarshal([]byte(out), &beads); err != nil {
		return FetchedItem{}, fmt.Errorf("parse bd show: %w", err)
	}
	if len(beads) == 0 {
		return FetchedItem{}, fmt.Errorf("bd show %s: no such bead", id)
	}
	b := beads[0]
	return FetchedItem{ID: b.ID, URL: beadURL(b.ID), Title: b.Title, Body: b.Description, Closed: b.closed()}, nil
}

// TaskStates reports per-task done-state from the change's child beads, keyed by
// normalized title — the same key reconcile merges on (TaskStateReader). The
// epic is skipped: it carries the change, not a task.
func (p *BeadsProvider) TaskStates(ctx context.Context, slug string, _ *Ref) (map[string]bool, error) {
	fam, err := p.family(ctx, slug)
	if err != nil {
		return nil, err
	}
	states := map[string]bool{}
	for _, b := range fam {
		if b.isEpic() {
			continue
		}
		states[normalizeTaskText(b.Title)] = b.closed()
	}
	return states, nil
}

// Push ensures the change's epic bead exists, that every task has a child bead
// under it, and that each child's open/closed status reflects its task's
// checkbox — projecting done-state outward the way the GitHub provider renders
// [x] into an issue body. Existing beads are matched by marker (epic) and
// normalized title (children), so re-running never duplicates the graph; titles
// are never rewritten (OpenSpec owns wording). Projection is monotonic, matching
// the inbound union: a checked task closes its open bead, but Push never reopens
// a closed one.
func (p *BeadsProvider) Push(ctx context.Context, item WorkItem, existing *Ref) (Ref, error) {
	fam, err := p.family(ctx, item.Slug)
	if err != nil {
		return Ref{}, err
	}

	// Locate or create the epic — the ref anchor for the change.
	var epic *bead
	for i := range fam {
		if fam[i].isEpic() {
			epic = &fam[i]
			break
		}
	}
	var ref Ref
	switch {
	case epic != nil:
		ref = Ref{Provider: p.Name(), ID: epic.ID, URL: beadURL(epic.ID)}
	case existing != nil:
		ref = *existing
	default:
		id, err := p.create(ctx, item.Title, p.epicDescription(item), "")
		if err != nil {
			return Ref{}, err
		}
		ref = Ref{Provider: p.Name(), ID: id, URL: beadURL(id)}
	}

	// Index existing child beads by normalized title.
	children := map[string]*bead{}
	for i := range fam {
		if !fam[i].isEpic() {
			children[normalizeTaskText(fam[i].Title)] = &fam[i]
		}
	}

	// Ensure a child bead per task, then project done-state: a checked task
	// closes its open bead. Newly created beads start open, so an already-checked
	// task is created and then closed.
	for _, t := range itemTasks(item) {
		child := children[t.Text]
		if child == nil {
			id, err := p.create(ctx, t.Text, marker(item.Slug), ref.ID)
			if err != nil {
				return Ref{}, err
			}
			child = &bead{ID: id, Title: t.Text, Status: "open"}
			children[t.Text] = child
		}
		if t.Checked && !child.closed() {
			if _, err := p.run(ctx, "close", child.ID, "-r", "completed in spec"); err != nil {
				return Ref{}, err
			}
		}
	}

	// An archived change closes its epic.
	if item.Closed && (epic == nil || !epic.closed()) {
		if _, err := p.run(ctx, "close", ref.ID, "-r", "change archived"); err != nil {
			return Ref{}, err
		}
	}
	return ref, nil
}

// create makes one bead and returns its id. parent is the epic id for children
// (empty for the epic itself). --silent makes bd emit only the id.
func (p *BeadsProvider) create(ctx context.Context, title, description, parent string) (string, error) {
	args := []string{"create", title, "-d", description, "--silent"}
	if parent == "" {
		args = append(args, "--type", "epic")
	} else {
		args = append(args, "--parent", parent)
	}
	id, err := p.run(ctx, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(id), nil
}

// epicDescription is the proposal body with the identity marker appended — the
// inverse of what splitBody strips on the way back in.
func (p *BeadsProvider) epicDescription(item WorkItem) string {
	proposal, _, _ := splitBody(item.Body, item.Title)
	return strings.TrimSpace(proposal) + "\n\n" + marker(item.Slug)
}

// taskState is one task line parsed from a rendered WorkItem body: its
// normalized text (the child-bead match key) and checkbox state.
type taskState struct {
	Text    string
	Checked bool
}

// itemTasks extracts the task lines (text + checkbox state) from a rendered
// WorkItem body, reusing the same body splitter and task-line parser the
// reconcile path uses so child-bead titles match exactly what reconcile keys on.
func itemTasks(item WorkItem) []taskState {
	_, tasks, _ := splitBody(item.Body, item.Title)
	var out []taskState
	for _, line := range strings.Split(tasks, "\n") {
		if text, checked, ok := parseTaskLine(line); ok {
			out = append(out, taskState{Text: text, Checked: checked})
		}
	}
	return out
}
