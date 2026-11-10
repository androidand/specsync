package specsync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TrustFileEnv overrides where trust records are kept (used by tests).
const TrustFileEnv = "SPECSYNC_TRUST_FILE"

// TrustFile is the per-user file mapping a repository to the hash of the
// config the user trusted.
func TrustFile() string {
	if p := os.Getenv(TrustFileEnv); p != "" {
		return p
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "specsync", "trust.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "specsync", "trust.json")
}

func repoKey(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		return root
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real
	}
	return abs
}

// ConfigHash is the hash of the repository's config file, or "" when it has none.
func ConfigHash(root string) string {
	data, err := os.ReadFile(filepath.Join(root, SpecSyncConfigPath))
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func readTrust() map[string]string {
	records := map[string]string{}
	if data, err := os.ReadFile(TrustFile()); err == nil {
		_ = json.Unmarshal(data, &records)
	}
	return records
}

func writeTrust(records map[string]string) error {
	path := TrustFile()
	if path == "" {
		return fmt.Errorf("cannot locate the trust file: set %s", TrustFileEnv)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// IsTrusted reports whether the current config content is the one the user
// trusted for this repository.
func IsTrusted(root string) bool {
	h := ConfigHash(root)
	return h != "" && readTrust()[repoKey(root)] == h
}

// Trust records the current config content as trusted and returns its hash.
func Trust(root string) (string, error) {
	h := ConfigHash(root)
	if h == "" {
		return "", fmt.Errorf("no %s to trust", SpecSyncConfigPath)
	}
	records := readTrust()
	records[repoKey(root)] = h
	return h, writeTrust(records)
}

// Untrust removes the record for the repository.
func Untrust(root string) error {
	records := readTrust()
	delete(records, repoKey(root))
	return writeTrust(records)
}

// RequireTrust returns nil when commands is empty or the config is trusted,
// and otherwise an error that lists the commands and how to trust them. Any
// code that runs a command defined in the config must call it first.
func RequireTrust(root string, commands []string) error {
	if len(commands) == 0 || IsTrusted(root) {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s has not been trusted (or changed since it was), so nothing was run. It would run:\n", SpecSyncConfigPath)
	for _, c := range commands {
		fmt.Fprintf(&b, "  $ %s\n", c)
	}
	b.WriteString("Read them, then run: specsync trust -yes")
	return fmt.Errorf("%s", b.String())
}
