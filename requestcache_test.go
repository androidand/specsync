package specsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestCache(t *testing.T, now *time.Time) *FileRequestCache {
	t.Helper()
	return &FileRequestCache{Dir: t.TempDir(), TTL: time.Minute, now: func() time.Time { return *now }}
}

func TestFileRequestCache_RoundTripAndExpiry(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	c := newTestCache(t, &now)
	set := ChangeRequestSet{Truncated: true, Requests: []ChangeRequest{{Number: 7, Branch: "feat/x", State: "open"}}}
	if _, ok := c.Get("acme/portal"); ok {
		t.Fatal("empty cache must miss")
	}
	c.Put("acme/portal", set)

	got, ok := c.Get("acme/portal")
	if !ok || !got.Truncated || len(got.Requests) != 1 || got.Requests[0].Number != 7 {
		t.Fatalf("round trip = %+v, %v", got, ok)
	}
	if _, ok := c.Get("acme/other"); ok {
		t.Fatal("repos are cached separately")
	}

	now = now.Add(59 * time.Second)
	if _, ok := c.Get("acme/portal"); !ok {
		t.Fatal("still fresh inside the TTL")
	}
	now = now.Add(2 * time.Second)
	if _, ok := c.Get("acme/portal"); ok {
		t.Fatal("expired after the TTL")
	}
}

func TestFileRequestCache_CorruptOrUnwritableNeverFails(t *testing.T) {
	now := time.Now()
	c := newTestCache(t, &now)
	c.Put("acme/portal", ChangeRequestSet{})
	files, _ := filepath.Glob(filepath.Join(c.Dir, "*"))
	if len(files) != 1 {
		t.Fatalf("expected one cache file, got %v", files)
	}
	if err := os.WriteFile(files[0], []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get("acme/portal"); ok {
		t.Fatal("a corrupt file is a miss")
	}

	// A directory that cannot be created: Put silently does nothing.
	blocked := &FileRequestCache{Dir: filepath.Join(files[0], "sub"), TTL: time.Minute}
	blocked.Put("acme/portal", ChangeRequestSet{})
}

func TestFileRequestCache_RepoNameIsSafeAsAFileName(t *testing.T) {
	now := time.Now()
	c := newTestCache(t, &now)
	c.Put("../../etc/evil", ChangeRequestSet{})
	files, _ := filepath.Glob(filepath.Join(c.Dir, "*"))
	if len(files) != 1 || strings.Contains(filepath.Base(files[0]), "/") || !strings.HasPrefix(files[0], c.Dir) {
		t.Fatalf("cache file escaped its directory: %v", files)
	}
}

func TestBuildTopology_RequestCacheServesTheSecondRun(t *testing.T) {
	now := time.Now()
	cache := newTestCache(t, &now)
	var asked []string
	opts := requestFixture(t, &asked)
	opts.Cache = cache

	for i := 0; i < 2; i++ {
		topo, err := BuildTopology(context.Background(), opts)
		if err != nil {
			t.Fatal(err)
		}
		if topo.Changes[0].PR == nil || topo.Changes[0].PR.Number != 7 {
			t.Fatalf("run %d: pr = %+v", i, topo.Changes[0].PR)
		}
	}
	if len(asked) != 1 {
		t.Fatalf("second run inside the TTL must not ask the provider, asked %v", asked)
	}

	opts.RefreshRequests = true
	if _, err := BuildTopology(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if len(asked) != 2 {
		t.Fatalf("refresh must bypass the cache, asked %v", asked)
	}

	now = now.Add(2 * time.Minute)
	opts.RefreshRequests = false
	if _, err := BuildTopology(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if len(asked) != 3 {
		t.Fatalf("an expired entry is refetched, asked %v", asked)
	}
}

func TestBuildTopology_FailedListingIsNotCached(t *testing.T) {
	now := time.Now()
	cache := newTestCache(t, &now)
	var asked []string
	opts := requestFixture(t, &asked)
	opts.Cache = cache
	fail := true
	good := opts.changeRequests
	opts.changeRequests = func(ctx context.Context, repo string) (*ChangeRequestSet, error) {
		if fail {
			return nil, context.DeadlineExceeded
		}
		return good(ctx, repo)
	}
	if _, err := BuildTopology(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	fail = false
	topo, _ := BuildTopology(context.Background(), opts)
	if topo.Changes[0].PR == nil {
		t.Fatal("a failure must not poison the cache")
	}
}

func TestBuildTopology_LocalOnlyNeverTouchesTheCache(t *testing.T) {
	now := time.Now()
	cache := newTestCache(t, &now)
	cache.Put("acme/portal", ChangeRequestSet{Requests: []ChangeRequest{{Number: 7, Branch: "feat/4231-add-field-forms", State: "open"}}})
	var asked []string
	opts := requestFixture(t, &asked)
	opts.Cache = cache
	opts.LocalOnly = true
	topo, _ := BuildTopology(context.Background(), opts)
	if topo.Changes[0].PR != nil {
		t.Fatal("-state=off shows no request state, not even a cached one")
	}
}
