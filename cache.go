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

func loadRefs(changeDir string) (map[string]Ref, error) {
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
	refs, err := loadRefs(changeDir)
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
// shorthand can't be derived), appended to the correct section of whatever the
// file already contains.
//
// The append is the point. links.md is the human- and agent-readable source of
// relationship truth, and people write more into it than link entries: prose,
// dependency order, sequencing notes. Rewriting the file to a bare list of URLs
// — as this used to — silently destroyed all of it. So specsync only ever adds
// lines here; removing one is the author's call.
//
// Refs are deduplicated against the file's *resolved* entries (via
// parseLinksMD), so a full URL and its "owner/repo#N" shorthand count as the
// same link. A ref set that is already fully recorded writes nothing at all.
// The three ref lists (links, blockedBy, blocks) are appended under their
// respective section headers.
func saveLinksToMD(changeDir, openspecDir string, links, blockedBy, blocks []Ref) error {
	path := filepath.Join(changeDir, "links.md")

	// Collect all already-recorded refs for dedup.
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read links.md: %w", err)
	}
	exLinks, exBlockedBy, exBlocks := parseLinksMD(changeDir, openspecDir)
	recorded := map[string]bool{}
	for _, r := range exLinks {
		recorded[r.Provider+"#"+r.ID] = true
	}
	for _, r := range exBlockedBy {
		recorded[r.Provider+"#"+r.ID] = true
	}
	for _, r := range exBlocks {
		recorded[r.Provider+"#"+r.ID] = true
	}

	// Build new entries per section.
	newLinks := newEntriesStr(links, recorded)
	newBlockedBy := newEntriesStr(blockedBy, recorded)
	newBlocks := newEntriesStr(blocks, recorded)

	if newLinks == "" && newBlockedBy == "" && newBlocks == "" {
		return nil
	}

	var sb strings.Builder
	out := string(existing)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
		sb.WriteString("\n")
	}

	if newLinks != "" {
		sb.WriteString("## Related\n")
		sb.WriteString(newLinks)
	}

	if newBlockedBy != "" {
		sb.WriteString("## Blocked by\n")
		sb.WriteString(newBlockedBy)
	}

	if newBlocks != "" {
		sb.WriteString("## Blocks\n")
		sb.WriteString(newBlocks)
	}

	return os.WriteFile(path, []byte(out+sb.String()), 0o644)
}

func newEntriesStr(refs []Ref, recorded map[string]bool) string {
	var sb strings.Builder
	for _, r := range refs {
		key := r.Provider + "#" + r.ID
		if recorded[key] {
			continue
		}
		entry := ghShortEntry(r.URL)
		if entry == "" {
			continue
		}
		sb.WriteString("- ")
		sb.WriteString(entry)
		sb.WriteByte('\n')
		recorded[key] = true
	}
	return sb.String()
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
