package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/androidand/specsync"
)

type verifyRefsOptions struct {
	OpenSpecDir string
	RepoDir     string
	Paths       []string
	Provider    specsync.WorkProvider
	DefaultKey  string
}

type refRow struct {
	file string
	line int
	text string
}

// verifyRefs scans change documents (and Paths) for commit and request
// references, prints what no longer resolves, and returns how many commit
// references were stale or orphaned. It reads documents, git and the provider
// and writes nothing.
func verifyRefs(ctx context.Context, w io.Writer, opts verifyRefsOptions) (int, error) {
	type source struct {
		slug  string
		files []string
	}
	var sources []source
	changes, err := specsync.LoadChanges(opts.OpenSpecDir)
	if err != nil {
		return 0, err
	}
	for _, c := range changes {
		if c.Archived {
			continue
		}
		sources = append(sources, source{slug: c.Slug, files: markdownFiles(c.Dir)})
	}
	var given []string
	for _, p := range opts.Paths {
		if _, err := os.Stat(p); err != nil {
			return 0, fmt.Errorf("verify -refs: %w", err)
		}
		given = append(given, markdownFiles(p)...)
	}
	if len(given) > 0 {
		sources = append(sources, source{files: given})
	}
	scanned := map[string]bool{}
	for i := range sources {
		var files []string
		for _, f := range sources[i].files {
			abs, err := filepath.Abs(f)
			if err != nil {
				abs = f
			}
			if !scanned[abs] {
				scanned[abs] = true
				files = append(files, f)
			}
		}
		sources[i].files = files
	}

	var rows []refRow
	var all []specsync.DocRef
	stale := 0
	checker := specsync.NewDocRefChecker(ctx, opts.RepoDir)
	for _, src := range sources {
		var refs []specsync.DocRef
		for _, f := range src.files {
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			refs = append(refs, specsync.ScanDocRefs(displayPath(opts.RepoDir, f), string(data))...)
		}
		all = append(all, refs...)
		for _, c := range checker.Commits(refs, checker.Reachable(src.slug)) {
			switch c.Status {
			case "unknown":
				rows = append(rows, refRow{c.Ref.File, c.Ref.Line, "unknown  " + c.Ref.Text + "  (not fetched in this shallow clone)"})
			case "stale":
				stale++
				rows = append(rows, refRow{c.Ref.File, c.Ref.Line, "stale  " + c.Ref.Text})
			case "orphaned":
				stale++
				rows = append(rows, refRow{c.Ref.File, c.Ref.Line, fmt.Sprintf("orphaned  %s  %q", c.Ref.Text, c.Subject)})
			}
		}
	}

	checks, status := specsync.CheckRequests(ctx, opts.Provider, opts.DefaultKey, all)
	for _, c := range checks {
		switch c.State {
		case "missing":
			rows = append(rows, refRow{c.Ref.File, c.Ref.Line, c.Ref.Text + "  missing"})
		case "closed", "merged":
			state := c.State
			if c.Reason != "" {
				state += " (" + c.Reason + ")"
			}
			rows = append(rows, refRow{c.Ref.File, c.Ref.Line, c.Ref.Text + "  " + state})
		}
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].file != rows[j].file {
			return rows[i].file < rows[j].file
		}
		return rows[i].line < rows[j].line
	})
	for _, r := range rows {
		fmt.Fprintf(w, "%s:%d  %s\n", r.file, r.line, r.text)
	}
	if status != "" {
		fmt.Fprintf(w, "note: %s\n", status)
	}
	if len(rows) == 0 && status == "" {
		fmt.Fprintln(w, "verify -refs: every commit and request reference resolves")
	} else if len(rows) == 0 {
		fmt.Fprintln(w, "verify -refs: no findings among the references that could be checked")
	} else {
		fmt.Fprintf(w, "verify -refs: %d finding(s), %d stale or orphaned commit(s)\n", len(rows), stale)
	}
	return stale, nil
}

func markdownFiles(path string) []string {
	var out []string
	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".md") {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func displayPath(repoDir, p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		if rel, err := filepath.Rel(repoDir, abs); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(p)
}
