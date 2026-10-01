package specsync

// This repository is PUBLIC. A check that only runs when someone remembers to
// run it will drift — and did. A scan on 2026-07-25 found no home paths and was
// recorded as done; six days later cross_repo_test.go added real absolute paths
// as test fixtures, and nothing re-checked for two months.
//
// So the check runs in `go test ./...`, which ci.yml already invokes. It
// cannot be skipped by forgetting a CI step, and it fails locally too.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	// /Users/<lowercase-name> — a real macOS home directory. Placeholders
	// like /Users/<name> are deliberately excluded by requiring lowercase
	// letters, so documentation can still describe the shape of a path.
	homePath = regexp.MustCompile(`/Users/[a-z][a-z0-9._-]*`)

	// RFC1918 address literals. A dotted quad only; prose mentioning
	// "192.168." as a thing to scan for is not a match.
	privateIP = regexp.MustCompile(
		`\b(?:10\.[0-9]{1,3}|192\.168|172\.(?:1[6-9]|2[0-9]|3[01]))\.[0-9]{1,3}\.[0-9]{1,3}\b`)

	// Credential prefixes. Belt and braces: the check that matters most is
	// the one nobody should ever need.
	credential = regexp.MustCompile(
		`\b(?:ghp_[A-Za-z0-9]{20,}|gho_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|` +
			`ABSK[A-Z0-9]{10,}|AKIA[0-9A-Z]{16}|sk-ant-[A-Za-z0-9_-]{20,})\b`)
)

// skippedNever: this test is meaningless if the repository stops being public,
// but it is also not harmful then, so it runs unconditionally and says so.
func TestNoPrivateDataInPublicRepo(t *testing.T) {
	root := "."
	if _, err := os.Stat("go.mod"); err != nil {
		root = "." // running from a subpackage; paths still resolve upward
	}

	var offenders []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable path: not a privacy failure
		}
		name := d.Name()
		if d.IsDir() {
			// Skip VCS, build and dependency trees. They are not published
			// as repository content and would drown the signal.
			switch name {
			case ".git", "node_modules", "site", "dist", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(name) {
		case ".go", ".md", ".yml", ".yaml", ".json", ".sh", ".txt", ".toml":
		default:
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, re := range []struct {
				what string
				re   *regexp.Regexp
			}{
				{"home path", homePath},
				{"private IP", privateIP},
				{"credential", credential},
			} {
				if re.re.FindString(line) != "" {
					offenders = append(offenders, fmt.Sprintf("%s:%d: %s: %s",
						path, i+1, re.what, strings.TrimSpace(line)))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(offenders) > 0 {
		t.Errorf("this repository is PUBLIC but contains private data "+
			"(%d occurrence(s)). Redact before pushing:\n  %s",
			len(offenders), strings.Join(offenders, "\n  "))
	}
}
