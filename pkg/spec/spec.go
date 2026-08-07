// Package spec provides SpecSource implementations for external use.
// The SpecSource interface is defined in the root package (specsync.SpecSource).
//
// To use:
//
//	import "github.com/androidand/specsync/pkg/spec"
//
//	source := spec.NewFileSpecSource() // or spec.NewBeadsSource()
//	changes, err := source.LoadChanges(openspecDir)
package spec

// NewFileSpecSource returns a FileSpecSource that loads OpenSpec changes from disk.
// It implements specsync.SpecSource (defined in the root package).
func NewFileSpecSource() FileSpecSource {
	return FileSpecSource{}
}

// NewBeadsSource returns a BeadsSource placeholder.
// It implements specsync.SpecSource (defined in the root package).
func NewBeadsSource() BeadsSource {
	return BeadsSource{}
}
