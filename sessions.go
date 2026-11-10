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
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
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
	lw := &limitedWriter{w: &out, max: maxSessionsOutput}
	cmd.Stdout = lw
	err := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Errorf("session provider timed out after %s", timeout)
	}
	if lw.over {
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
			e.ID, e.Name, e.Status = SanitizeText(e.ID, maxShownText), SanitizeText(e.Name, maxShownText), SanitizeText(e.Status, maxShownText)
			e.UpdatedAt, e.Resume = SanitizeText(e.UpdatedAt, maxShownText), SanitizeText(e.Resume, maxShownText)
			kept = append(kept, e)
		}
	}
	return kept, nil
}

// limitedWriter fails once more than max bytes have been written, which stops
// the command's copy and lets the caller report an oversized output.
type limitedWriter struct {
	w    io.Writer
	max  int
	n    int
	over bool
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.n+len(p) > l.max {
		l.over = true
		return 0, errors.New("output too large")
	}
	l.n += len(p)
	return l.w.Write(p)
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
		if !filepath.IsAbs(e.CWD) {
			unmatched++
			continue
		}
		cwd := resolved(e.CWD)
		var best *target
		for _, t := range targets {
			if insideOrSame(cwd, t.path) {
				if best == nil || len(t.path) > len(best.path) {
					best = t
				}
			}
		}
		if best == nil {
			unmatched++
			continue
		}
		i := best.rows[0]
		for _, j := range best.rows {
			if rows[j].Slug != "" {
				i = j
				break
			}
		}
		rows[i].Sessions = append(rows[i].Sessions, e)
	}

	for i := range rows {
		if rows[i].Worktree == "" {
			continue
		}
		sessions := rows[i].Sessions
		sort.SliceStable(sessions, func(a, b int) bool {
			return sessionTime(sessions[a]).After(sessionTime(sessions[b]))
		})
		rows[i].Resume = "cd " + shellQuote(rows[i].Worktree)
		if len(sessions) > 0 && sessions[0].Resume != "" {
			rows[i].Resume = sessions[0].Resume
		}
	}
	return unmatched
}

var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// shellQuote makes p safe to paste into a shell: bare when it has only safe
// characters, else single-quoted with embedded quotes escaped.
func shellQuote(p string) string {
	if shellSafe.MatchString(p) {
		return p
	}
	return "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
}

// SessionsSkippedOffline is the status line when -state=off keeps the provider from running.
const SessionsSkippedOffline = "session provider skipped with -state=off (offline)"

// maxShownText bounds any provider or repository text printed to a terminal.
const maxShownText = 120

// SanitizeText makes text from a provider or a repository file safe to print:
// control and escape characters are dropped, whitespace runs collapse to one
// space, and the result is cut to max runes.
func SanitizeText(s string, max int) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			space = true
		case unicode.IsControl(r):
		default:
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
		}
	}
	out := b.String()
	if runes := []rune(out); len(runes) > max {
		out = string(runes[:max-1]) + "…"
	}
	return out
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
		if s := SanitizeText(lines[i], maxShownText); s != "" {
			return s
		}
	}
	return ""
}

// ApplySessions loads the provider (when cmdline is set), attaches its sessions
// to rows and returns the status lines to show. A failing provider adds one line
// and changes nothing else, so sessions can never make a command fail.
// reportUnmatched says whether the rows cover everything the provider may know
// about; a view of one worktree or store must not claim sessions matched no work.
func ApplySessions(ctx context.Context, rows []ChangeTopology, cmdline string, timeout time.Duration, reportUnmatched bool) []string {
	entries, err := LoadSessions(ctx, cmdline, timeout)
	var status []string
	if err != nil {
		status = append(status, err.Error())
	}
	n := AttachSessions(rows, entries)
	if !reportUnmatched {
		return status
	}
	switch n {
	case 0:
	case 1:
		status = append(status, "1 session matched no work")
	default:
		status = append(status, fmt.Sprintf("%d sessions matched no work", n))
	}
	return status
}
