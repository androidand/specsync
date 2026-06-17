package specsync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
