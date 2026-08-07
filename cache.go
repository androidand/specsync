package specsync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The ref cache lives under <change>/.specsync/ which is gitignored, satisfying
// the rule that provider ids never enter git. It maps provider name -> Ref and
// is purely an optimization: a missing or stale cache is rebuilt via the
// provider's Find (identity marker).

func refCachePath(changeDir string) string {
	return filepath.Join(changeDir, ".specsync", "refs.json")
}

func LoadRefs(changeDir string) (map[string]Ref, error) {
	b, err := os.ReadFile(refCachePath(changeDir))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Ref{}, nil
		}
		return nil, fmt.Errorf("read ref cache: %w", err)
	}
	refs := map[string]Ref{}
	if err := json.Unmarshal(b, &refs); err != nil {
		return nil, fmt.Errorf("parse ref cache: %w", err)
	}
	return refs, nil
}

func saveRef(changeDir, provider string, ref Ref) error {
	refs, err := LoadRefs(changeDir)
	if err != nil {
		return err
	}
	refs[provider] = ref
	// Migrating a ref to a repo-qualified key retires the legacy bare "github"
	// entry; leaving it behind would keep a stale duplicate around forever. The
	// only legacy entry worth keeping is one that verifiably points at a
	// *different* repo — it is still that repo's only link until a sync
	// targeting it migrates it in turn. Same-repo entries are superseded by the
	// canonical ref being written; unparseable ones can't belong to any repo
	// (sync's guarded fallback will never use them) and are dropped as garbage.
	if strings.HasPrefix(provider, "github:") {
		if legacy, ok := refs["github"]; ok {
			repo, parsed := ghIssueRepo(legacy.URL)
			if !parsed || strings.EqualFold(repo, strings.TrimPrefix(provider, "github:")) {
				delete(refs, "github")
			}
		}
	}

	if err := os.MkdirAll(filepath.Join(changeDir, ".specsync"), 0o755); err != nil {
		return fmt.Errorf("create .specsync: %w", err)
	}
	b, err := json.MarshalIndent(refs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ref cache: %w", err)
	}
	if err := os.WriteFile(refCachePath(changeDir), append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write ref cache: %w", err)
	}
	return nil
}

// saveLinksToMD records refs in links.md in the change dir root. Each ref not
// already recorded becomes a "- owner/repo#N" line (or a bare URL when the
// shorthand can't be derived), appended to whatever the file already contains.
//
// The append is the point. links.md is the human- and agent-readable source of
// relationship truth, and people write more into it than link entries: prose,
// dependency order, sequencing notes, the `## Blocked by` / `## Blocks` sections
// the dependency sync maintains. Rewriting the file to a bare list of URLs — as
// this used to — silently destroyed all of it, on every `link`, `pull`, and
// `spinoff`. So specsync only ever adds lines here; removing one is the author's
// call.
//
// Refs are deduplicated against the file's *resolved* entries, so a full URL and
// its "owner/repo#N" shorthand count as the same link, as does a sibling slug
// that resolves to it. A ref set that is already fully recorded writes nothing at
// all, keeping repeat runs byte-for-byte idempotent. openspecDir is needed to
// resolve slug entries; pass "" to compare only the literal reference forms.
func saveLinksToMD(changeDir, openspecDir string, refs []Ref) error {
	path := filepath.Join(changeDir, "links.md")
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read links.md: %w", err)
	}

	recorded := map[string]bool{}
	links, blockedBy, blocks := parseLinksMD(changeDir, openspecDir)
	for _, r := range append(links, append(blockedBy, blocks...)...) {
		recorded[linkKey(r)] = true
	}

	var sb strings.Builder
	for _, r := range refs {
		entry := ghShortEntry(r.URL)
		if entry == "" {
			continue // nothing to point at; a bare "- " is noise, not a link
		}
		key := linkKey(r)
		if recorded[key] {
			continue
		}
		recorded[key] = true
		sb.WriteString("- ")
		sb.WriteString(entry)
		sb.WriteByte('\n')
	}
	if sb.Len() == 0 {
		return nil
	}

	// Keep the appended entries on their own lines without collapsing whatever
	// the last existing line was.
	out := string(existing)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return os.WriteFile(path, []byte(out+sb.String()), 0o644)
}

// linkKey identifies the issue a Ref points at, for deduplicating links.md
// entries. It keys on the normalized URL rather than provider+id because the
// provider string is not canonical: the same issue is "github" in a legacy ref
// and "github:owner/repo" in a current one (see saveRef's migration), and
// spinoff builds bare-"github" refs while parseLinksMD resolves the identical
// URL to the qualified form. Keying on the URL makes those all one link.
func linkKey(r Ref) string {
	if r.URL != "" {
		return ghShortEntry(r.URL)
	}
	return r.Provider + "#" + r.ID
}

// ghShortEntry converts a GitHub issue URL to "owner/repo#N" shorthand.
// Falls back to the original URL for non-GitHub or unexpected shapes.
func ghShortEntry(url string) string {
	const prefix = "https://github.com/"
	if !strings.HasPrefix(url, prefix) {
		return url
	}
	rest := url[len(prefix):]
	if i := strings.Index(rest, "/issues/"); i >= 0 {
		return rest[:i] + "#" + rest[i+8:]
	}
	return url
}
