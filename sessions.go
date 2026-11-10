package specsync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DefaultSessionsTimeout bounds one run of the session provider.
const DefaultSessionsTimeout = 5 * time.Second

const maxSessionsOutput = 1 << 20

// SessionEntry is one session reported by the provider. Only CWD is required;
// Resume is opaque text that specsync prints and never interprets.
type SessionEntry struct {
	CWD       string `json:"cwd"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Status    string `json:"status,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	Resume    string `json:"resume,omitempty"`
}

// LoadSessions runs the provider command line once through the shell and
// returns its entries that carry a cwd. The command comes from the caller, never
// from a repository. A failure returns an error and no entries.
func LoadSessions(ctx context.Context, cmdline string, timeout time.Duration) ([]SessionEntry, error) {
	if strings.TrimSpace(cmdline) == "" {
		return nil, nil
	}
	if timeout <= 0 {
		timeout = DefaultSessionsTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdline)
	cmd.WaitDelay = 200 * time.Millisecond
	var out bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &out, max: maxSessionsOutput}
	err := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Errorf("session provider timed out after %s", timeout)
	}
	var tooBig *outputTooLargeError
	if errors.As(err, &tooBig) || out.Len() >= maxSessionsOutput {
		return nil, fmt.Errorf("session provider output is larger than %d bytes", maxSessionsOutput)
	}
	if err != nil {
		return nil, fmt.Errorf("session provider failed: %s", errReason(err))
	}
	var all []SessionEntry
	if err := json.Unmarshal(out.Bytes(), &all); err != nil {
		return nil, fmt.Errorf("session provider did not print a JSON array: %v", err)
	}
	var kept []SessionEntry
	for _, e := range all {
		if strings.TrimSpace(e.CWD) != "" {
			kept = append(kept, e)
		}
	}
	return kept, nil
}

type outputTooLargeError struct{}

func (*outputTooLargeError) Error() string { return "output too large" }

// limitedWriter stops accepting bytes once max is reached, which fails the
// command's copy and lets the caller report an oversized output.
type limitedWriter struct {
	w   io.Writer
	max int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if room := l.max - sizeOf(l.w); len(p) > room {
		if room > 0 {
			l.w.Write(p[:room])
		}
		return 0, &outputTooLargeError{}
	}
	return l.w.Write(p)
}

func sizeOf(w io.Writer) int {
	if b, ok := w.(*bytes.Buffer); ok {
		return b.Len()
	}
	return 0
}

// AttachSessions joins entries to rows by the longest worktree path that
// prefixes the entry's cwd, and returns how many entries matched no row. Every
// row with a worktree gets a resume hint: the most recently updated matching
// session's own string when it has one, else `cd <worktree>`. Nothing is stored.
func AttachSessions(rows []ChangeTopology, entries []SessionEntry) (unmatched int) {
	type target struct {
		path string
		rows []int
	}
	var targets []*target
	byPath := map[string]*target{}
	for i, r := range rows {
		if r.Worktree == "" {
			continue
		}
		key := resolved(absPath(r.Worktree))
		t := byPath[key]
		if t == nil {
			t = &target{path: key}
			byPath[key] = t
			targets = append(targets, t)
		}
		t.rows = append(t.rows, i)
	}

	for _, e := range entries {
		cwd := resolved(absPath(e.CWD))
		var best *target
		for _, t := range targets {
			if cwd == t.path || strings.HasPrefix(cwd, t.path+string(filepath.Separator)) {
				if best == nil || len(t.path) > len(best.path) {
					best = t
				}
			}
		}
		if best == nil {
			unmatched++
			continue
		}
		for _, i := range best.rows {
			rows[i].Sessions = append(rows[i].Sessions, e)
		}
	}

	for i := range rows {
		if rows[i].Worktree == "" {
			continue
		}
		sessions := rows[i].Sessions
		sort.SliceStable(sessions, func(a, b int) bool {
			return sessionTime(sessions[a]).After(sessionTime(sessions[b]))
		})
		rows[i].Resume = "cd " + rows[i].Worktree
		if len(sessions) > 0 && sessions[0].Resume != "" {
			rows[i].Resume = sessions[0].Resume
		}
	}
	return unmatched
}

func absPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func sessionTime(e SessionEntry) time.Time {
	t, err := time.Parse(time.RFC3339, e.UpdatedAt)
	if err != nil {
		return time.Time{}
	}
	return t
}

// LastNote returns the last non-empty line of the change's discoveries.md, the
// file `specsync note` appends to, or "" when it is absent or empty.
func LastNote(changeDir string) string {
	data, err := os.ReadFile(filepath.Join(changeDir, "discoveries.md"))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if s := strings.TrimSpace(lines[i]); s != "" {
			return s
		}
	}
	return ""
}

// ApplySessions loads the provider (when cmdline is set), attaches its sessions
// to rows and returns the status lines to show. A failing provider adds one line
// and changes nothing else, so sessions can never make a command fail.
func ApplySessions(ctx context.Context, rows []ChangeTopology, cmdline string, timeout time.Duration) []string {
	entries, err := LoadSessions(ctx, cmdline, timeout)
	var status []string
	if err != nil {
		status = append(status, err.Error())
	}
	switch n := AttachSessions(rows, entries); n {
	case 0:
	case 1:
		status = append(status, "1 session matched no work")
	default:
		status = append(status, fmt.Sprintf("%d sessions matched no work", n))
	}
	return status
}
