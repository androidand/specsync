// Package specsync projects OpenSpec changes into external work trackers
// (GitHub Issues today; other providers later).
//
// It is a standalone, dependency-light tool. Its only inputs are OpenSpec
// directory conventions and an optional per-change ".status" file; its only
// outputs are work-tracker API calls (shelled out via the host CLI) and a
// gitignored ref cache under each change's .specsync/ directory. The package
// depends only on the Go standard library, an invariant enforced by
// TestStdlibOnly in boundary_test.go.
//
// Layering:
//
//	OpenSpec change folder  ->  Change   (change.go)
//	Change                  ->  WorkItem (sync.go)
//	WorkItem                ->  provider projection (provider.go, github.go)
//
// Stage model: OpenSpec has no native lifecycle beyond active/archived, which
// this package derives from the folder location. A richer stage may be supplied
// by writing its name into <change>/.status; orchestrators that track a funnel
// can populate it, while vanilla OpenSpec projects simply omit it.
package specsync
