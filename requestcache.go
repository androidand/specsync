package specsync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// DefaultRequestCacheTTL is how long a repo's request listing is reused.
const DefaultRequestCacheTTL = time.Minute

// RequestCache holds a repo's change-request listing for a short time so
// repeated status runs do not repeat the provider call. A cache is only ever a
// shortcut: a miss, a corrupt entry or a failed write must never be an error.
type RequestCache interface {
	Get(repo string) (ChangeRequestSet, bool)
	Put(repo string, set ChangeRequestSet)
}

// FileRequestCache stores listings as small files in a directory outside any
// repository (see DefaultRequestCache), so nothing is ever committed.
type FileRequestCache struct {
	Dir string
	TTL time.Duration

	now func() time.Time // test seam
}

// DefaultRequestCache returns a cache under the user's cache directory, or nil
// when there is none.
func DefaultRequestCache() *FileRequestCache {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		return nil
	}
	return &FileRequestCache{Dir: filepath.Join(dir, "specsync", "requests"), TTL: DefaultRequestCacheTTL}
}

type requestCacheFile struct {
	FetchedAt time.Time       `json:"fetchedAt"`
	Truncated bool            `json:"truncated,omitempty"`
	Requests  []ChangeRequest `json:"requests"`
}

func (c *FileRequestCache) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// path maps a repo to a file name that cannot leave Dir.
func (c *FileRequestCache) path(repo string) string {
	sum := sha256.Sum256([]byte(repo))
	return filepath.Join(c.Dir, "requests-"+hex.EncodeToString(sum[:8])+".json")
}

// Get returns the cached listing when it is younger than the TTL.
func (c *FileRequestCache) Get(repo string) (ChangeRequestSet, bool) {
	b, err := os.ReadFile(c.path(repo))
	if err != nil {
		return ChangeRequestSet{}, false
	}
	var f requestCacheFile
	if json.Unmarshal(b, &f) != nil || f.FetchedAt.IsZero() {
		return ChangeRequestSet{}, false
	}
	if age := c.clock().Sub(f.FetchedAt); age < 0 || age > c.TTL {
		return ChangeRequestSet{}, false
	}
	return ChangeRequestSet{Requests: f.Requests, Truncated: f.Truncated}, true
}

// Put stores the listing. Failures are ignored.
func (c *FileRequestCache) Put(repo string, set ChangeRequestSet) {
	b, err := json.Marshal(requestCacheFile{FetchedAt: c.clock(), Truncated: set.Truncated, Requests: set.Requests})
	if err != nil || os.MkdirAll(c.Dir, 0o700) != nil {
		return
	}
	tmp, err := os.CreateTemp(c.Dir, ".tmp-*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(b)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Chmod(tmp.Name(), 0o600) != nil || os.Rename(tmp.Name(), c.path(repo)) != nil {
		os.Remove(tmp.Name())
	}
}
