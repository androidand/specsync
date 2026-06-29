package specsync

import (
	"os"
	"path/filepath"
	"strings"
)

// ReleaseTool names the release tool a project uses and the responsibilities it
// owns. Detection is filesystem-only and never invokes the tool — specsync
// reports it and defers, so it visibly stays in its lane. The recommendation
// specsync prints is advisory; the named tool owns the actual bump/tag/publish.
type ReleaseTool struct {
	Name     string   // "goreleaser", "changesets", "custom", "none", ...
	Evidence []string // the marker paths that matched
	Owns     []string // responsibilities: "bump", "tag", "changelog", "publish"
}

// releaseToolProbe is one detector: a name, the responsibilities it owns, and a
// predicate over the repo root.
type releaseToolProbe struct {
	name  string
	owns  []string
	match func(root string) []string
}

// detectionOrder lists probes in priority order; the first to match wins.
var detectionOrder = []releaseToolProbe{
	{"release-please", []string{"bump", "tag", "changelog", "publish"}, anyExist("release-please-config.json", ".release-please-manifest.json")},
	{"changesets", []string{"bump", "changelog", "publish"}, anyExist(".changeset/config.json")},
	{"release-it", []string{"bump", "tag", "changelog", "publish"}, anyExistOrPkgKey([]string{".release-it.json", ".release-it.js", ".release-it.cjs", ".release-it.yaml"}, "release-it")},
	{"semantic-release", []string{"bump", "tag", "changelog", "publish"}, anyExistOrPkgKey([]string{".releaserc", ".releaserc.json", ".releaserc.yaml", ".releaserc.js", "release.config.js", "release.config.cjs"}, "\"release\"")},
	{"standard-version", []string{"bump", "tag", "changelog"}, anyExist(".versionrc", ".versionrc.json", ".versionrc.js")},
	{"goreleaser", []string{"bump", "publish"}, anyExist(".goreleaser.yaml", ".goreleaser.yml")},
}

// DetectReleaseTool probes root for the common release tools, returning the
// first match. With no marker it reports a custom flow (a release script) or
// "none"; either way the bump stays advisory.
func DetectReleaseTool(root string) ReleaseTool {
	for _, p := range detectionOrder {
		if ev := p.match(root); len(ev) > 0 {
			return ReleaseTool{Name: p.name, Evidence: ev, Owns: p.owns}
		}
	}
	if ev := customRelease(root); len(ev) > 0 {
		return ReleaseTool{Name: "custom", Evidence: ev, Owns: nil}
	}
	return ReleaseTool{Name: "none"}
}

func anyExist(paths ...string) func(root string) []string {
	return func(root string) []string {
		var found []string
		for _, p := range paths {
			if exists(filepath.Join(root, p)) {
				found = append(found, p)
			}
		}
		return found
	}
}

// anyExistOrPkgKey matches a marker file, or a key present in package.json.
func anyExistOrPkgKey(paths []string, pkgKey string) func(root string) []string {
	return func(root string) []string {
		if found := anyExist(paths...)(root); len(found) > 0 {
			return found
		}
		if b, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
			if strings.Contains(string(b), pkgKey) {
				return []string{"package.json:" + pkgKey}
			}
		}
		return nil
	}
}

// customRelease detects an ad-hoc release flow: a release script in package.json
// or a release target in a Makefile.
func customRelease(root string) []string {
	var ev []string
	if b, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		if strings.Contains(string(b), "\"release\"") {
			ev = append(ev, "package.json:scripts.release")
		}
	}
	if b, err := os.ReadFile(filepath.Join(root, "Makefile")); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "release:") {
				ev = append(ev, "Makefile:release")
				break
			}
		}
	}
	return ev
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
