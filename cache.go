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

// saveLinksToMD writes links.md in the change dir root. Each ref becomes a
// "- owner/repo#N" line (or bare URL when the shorthand can't be derived).
// links.md is the human- and agent-readable source of relationship truth;
// it is loaded by LoadChange on every sync so the Related section stays current.
func saveLinksToMD(changeDir string, refs []Ref) error {
	var sb strings.Builder
	for _, r := range refs {
		sb.WriteString("- ")
		sb.WriteString(ghShortEntry(r.URL))
		sb.WriteByte('\n')
	}
	return os.WriteFile(filepath.Join(changeDir, "links.md"), []byte(sb.String()), 0o644)
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
