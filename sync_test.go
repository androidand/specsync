package specsync

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// boardCapturingProvider is a fake WorkProvider that also implements
// BoardProjector, so Sync's board-projection call can be observed without a
// real GitHub round-trip.
type boardCapturingProvider struct {
	ref      Ref
	plan     BoardPlan
	err      error
	calls    int
	lastItem WorkItem
}

func (p *boardCapturingProvider) Name() string { return "github" }
func (p *boardCapturingProvider) Push(context.Context, WorkItem, *Ref) (Ref, error) {
	return p.ref, nil
}
func (p *boardCapturingProvider) Find(context.Context, string) (*Ref, error) { return nil, nil }

func (p *boardCapturingProvider) ProjectOntoBoard(_ context.Context, _ BoardTarget, _ Ref, item WorkItem, _ bool, _ string) (BoardPlan, error) {
	p.calls++
	p.lastItem = item
	return p.plan, p.err
}

func configuredTarget() BoardTarget {
	return BoardTarget{Owner: "acme", Number: 1}
}

func TestSyncProjectsOntoBoardWhenConfigured(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "changes", "c1", "proposal.md"), "# C1\n")
	prov := &boardCapturingProvider{
		ref:  Ref{Provider: "github", ID: "1", URL: "https://example.test/1"},
		plan: BoardPlan{ProjectID: "PVT_1", AddedToBoard: true},
	}

	res, err := Sync(context.Background(), Options{OpenSpecDir: root, Provider: prov, Project: configuredTarget()})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if prov.calls != 1 {
		t.Fatalf("expected exactly 1 ProjectOntoBoard call, got %d", prov.calls)
	}
	if len(res.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Items))
	}
	item := res.Items[0]
	if !item.BoardConfigured {
		t.Error("expected BoardConfigured true")
	}
	if item.Board == (BoardPlan{}) {
		t.Error("expected ItemResult.Board to carry the projected plan, got zero value")
	}
	if item.Board.ProjectID != "PVT_1" {
		t.Errorf("Board.ProjectID = %q, want PVT_1", item.Board.ProjectID)
	}
}

func TestSyncSkipsBoardProjectionWhenUnconfigured(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "changes", "c1", "proposal.md"), "# C1\n")
	prov := &boardCapturingProvider{ref: Ref{Provider: "github", ID: "1"}}

	res, err := Sync(context.Background(), Options{OpenSpecDir: root, Provider: prov})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if prov.calls != 0 {
		t.Fatalf("expected no ProjectOntoBoard calls when Project is unconfigured, got %d", prov.calls)
	}
	if res.Items[0].BoardConfigured {
		t.Error("expected BoardConfigured false")
	}
}

func TestSyncBoardProjectionErrorIsNonFatal(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "changes", "c1", "proposal.md"), "# C1\n")
	prov := &boardCapturingProvider{
		ref: Ref{Provider: "github", ID: "1", URL: "https://example.test/1"},
		err: os.ErrInvalid,
	}

	var stderr bytes.Buffer
	restore := redirectStderr(t, &stderr)
	defer restore()

	res, err := Sync(context.Background(), Options{OpenSpecDir: root, Provider: prov, Project: configuredTarget()})
	if err != nil {
		t.Fatalf("Sync must not fail when board projection errors: %v", err)
	}
	if res.Updated != 1 && res.Created != 1 {
		t.Fatalf("expected the issue push itself to still count as created/updated, got %+v", res)
	}
	if len(res.Items) != 1 || res.Items[0].URL == "" {
		t.Fatalf("expected the change to still be pushed despite the board error: %+v", res.Items)
	}
	restore()
	if !bytes.Contains(stderr.Bytes(), []byte("board projection for c1 failed")) {
		t.Errorf("expected a warning naming the change on stderr, got: %s", stderr.String())
	}
}

func TestSyncDryRunStillProjectsOntoBoard(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "changes", "c1", "proposal.md"), "# C1\n")
	prov := &boardCapturingProvider{
		ref:  Ref{Provider: "github", ID: "0"},
		plan: BoardPlan{ProjectID: "PVT_1", AddedToBoard: true},
	}

	res, err := Sync(context.Background(), Options{OpenSpecDir: root, Provider: prov, Project: configuredTarget(), DryRun: true})
	if err != nil {
		t.Fatalf("dry-run Sync: %v", err)
	}
	if prov.calls != 1 {
		t.Fatalf("expected ProjectOntoBoard to run under -dry-run (it handles dry-run internally), got %d calls", prov.calls)
	}
	if res.Items[0].Board == (BoardPlan{}) {
		t.Error("expected -dry-run to preview a non-zero board plan")
	}
}

// redirectStderr temporarily replaces os.Stderr so a test can capture warnings
// printed via fmt.Fprintf(os.Stderr, ...). Returns a restore func; safe to
// call more than once.
func redirectStderr(t *testing.T, into *bytes.Buffer) func() {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			n, rerr := r.Read(buf)
			if n > 0 {
				into.Write(buf[:n])
			}
			if rerr != nil {
				close(done)
				return
			}
		}
	}()
	restored := false
	return func() {
		if restored {
			return
		}
		restored = true
		os.Stderr = orig
		w.Close()
		<-done
		r.Close()
	}
}
