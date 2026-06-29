package specsync

import (
	"fmt"
	"strings"
)

// LinkOptions configures a link run: establishing cross-references between two
// or more local specs so their GitHub issues reference each other.
type LinkOptions struct {
	OpenSpecDir string   // path to the openspec/ directory
	Slugs       []string // at least 2 slugs to link together
	DryRun      bool
}

// LinkedPair describes one spec after linking — the caller uses Repo to
// construct the right provider and sync the updated body to GitHub.
type LinkedPair struct {
	Slug string
	Dir  string
	Repo string // "owner/name" from the ref key, or "" for auto-detect
	Ref  Ref    // the issue ref to update
}

// Link records cross-references between specs in their .specsync/links.json
// files and returns the pairs so the caller can sync each with the correct
// provider. It never writes to GitHub itself — syncing is left to the caller
// so provider selection (repo override, dry-runner) stays in one place.
func Link(opts LinkOptions) ([]LinkedPair, error) {
	if len(opts.Slugs) < 2 {
		return nil, fmt.Errorf("link: at least 2 slugs required")
	}

	type entry struct {
		change Change
		refKey string
		ref    Ref
	}
	entries := make([]entry, len(opts.Slugs))
	for i, slug := range opts.Slugs {
		c, err := loadChangeBySlug(opts.OpenSpecDir, slug)
		if err != nil {
			return nil, err
		}
		refs, err := loadRefs(c.Dir)
		if err != nil {
			return nil, err
		}
		if len(refs) == 0 {
			return nil, fmt.Errorf("slug %q has no synced ref; run specsync -slug %s first", slug, slug)
		}
		// Prefer the plain "github" key (same-repo); fall back to first available.
		key, ref := firstRef(refs)
		entries[i] = entry{change: *c, refKey: key, ref: ref}
	}

	pairs := make([]LinkedPair, len(entries))
	for i, e := range entries {
		// Collect the refs of all other slugs as links for this one.
		links := make([]Ref, 0, len(entries)-1)
		for j, other := range entries {
			if j != i {
				links = append(links, other.ref)
			}
		}

		if !opts.DryRun {
			if err := saveLinks(e.change.Dir, links); err != nil {
				return nil, fmt.Errorf("save links for %s: %w", e.change.Slug, err)
			}
		}

		pairs[i] = LinkedPair{
			Slug: e.change.Slug,
			Dir:  e.change.Dir,
			Repo: repoFromKey(e.refKey),
			Ref:  e.ref,
		}
	}
	return pairs, nil
}

// firstRef returns the first ref from the map, preferring the plain "github"
// key over namespaced "github:owner/repo" entries.
func firstRef(refs map[string]Ref) (string, Ref) {
	if r, ok := refs["github"]; ok {
		return "github", r
	}
	for k, r := range refs {
		return k, r
	}
	return "", Ref{}
}

// repoFromKey extracts "owner/name" from "github:owner/name", returning ""
// for the plain "github" key (meaning auto-detect from git remote).
func repoFromKey(key string) string {
	const prefix = "github:"
	if strings.HasPrefix(key, prefix) {
		return key[len(prefix):]
	}
	return ""
}
