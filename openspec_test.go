package specsync

import (
	"context"
	"testing"
)

// fakeOpenSpec answers --version, list, and show from canned strings, recording
// how many times the CLI was invoked so caching can be asserted.
func fakeOpenSpec(version, list, show string, count *int) func(context.Context, ...string) (string, error) {
	return func(_ context.Context, args ...string) (string, error) {
		*count++
		switch {
		case len(args) >= 1 && args[0] == "--version":
			return version, nil
		case len(args) >= 2 && args[0] == "list" && args[1] == "--specs":
			return `{"specs":[]}`, nil
		case len(args) >= 1 && args[0] == "list":
			return list, nil
		case len(args) >= 1 && args[0] == "show":
			return show, nil
		default:
			return "", nil
		}
	}
}

const listJSON = `{"changes":[{"name":"add-planning-scan","completedTasks":0,"totalTasks":12,"status":"in-progress","extraFutureField":"ignored"}]}`

const showJSON = `Warning: ignoring flags
{"id":"x","deltaCount":2,"deltas":[
  {"spec":"release-impact","operation":"ADDED","requirement":{"text":"R1"}},
  {"spec":"release-impact","operation":"REMOVED","requirement":{"text":"R2"}}
]}`

func TestOpenSpecChangesTolerateUnknownFields(t *testing.T) {
	var n int
	o := NewOpenSpecCLIFunc(fakeOpenSpec("1.4.1", listJSON, showJSON, &n))
	changes, err := o.Changes(context.Background())
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	if len(changes) != 1 || changes[0].Name != "add-planning-scan" || changes[0].Status != "in-progress" {
		t.Fatalf("parsed wrong: %+v", changes)
	}
}

func TestOpenSpecDeltasParsedAndCached(t *testing.T) {
	var n int
	o := NewOpenSpecCLIFunc(fakeOpenSpec("1.4.1", listJSON, showJSON, &n))
	d1, err := o.Deltas(context.Background(), "x")
	if err != nil {
		t.Fatalf("Deltas: %v", err)
	}
	if len(d1) != 2 || d1[0].Operation != "ADDED" || d1[1].Operation != "REMOVED" {
		t.Fatalf("deltas parsed wrong: %+v", d1)
	}
	callsAfterFirst := n
	if _, err := o.Deltas(context.Background(), "x"); err != nil {
		t.Fatalf("Deltas (cached): %v", err)
	}
	if n != callsAfterFirst {
		t.Fatalf("expected cached deltas to spawn no further calls, went %d -> %d", callsAfterFirst, n)
	}
}

func TestOpenSpecVersionGuard(t *testing.T) {
	var n int
	o := NewOpenSpecCLIFunc(fakeOpenSpec("1.3.9", listJSON, showJSON, &n))
	if _, err := o.Changes(context.Background()); err == nil {
		t.Fatalf("expected version error for openspec below minimum")
	}
}

func TestOpenSpecHasBaselineFalseWhenEmpty(t *testing.T) {
	var n int
	o := NewOpenSpecCLIFunc(fakeOpenSpec("1.4.1", listJSON, showJSON, &n))
	has, err := o.HasBaseline(context.Background())
	if err != nil {
		t.Fatalf("HasBaseline: %v", err)
	}
	if has {
		t.Fatalf("expected no baseline for empty specs")
	}
}

func TestVersionAtLeast(t *testing.T) {
	cases := []struct {
		got, want string
		ok        bool
	}{
		{"1.4.1", "1.4.0", true},
		{"1.4.0", "1.4.0", true},
		{"1.3.9", "1.4.0", false},
		{"2.0.0", "1.4.0", true},
		{"v1.4.1-beta", "1.4.0", true},
	}
	for _, c := range cases {
		if got := versionAtLeast(c.got, c.want); got != c.ok {
			t.Errorf("versionAtLeast(%q,%q) = %v, want %v", c.got, c.want, got, c.ok)
		}
	}
}
