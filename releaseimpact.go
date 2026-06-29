package specsync

import "fmt"

// ReleaseImpact is an advisory SemVer bump level. specsync recommends it; the
// project's release tool performs the actual bump.
type ReleaseImpact int

const (
	ImpactNone ReleaseImpact = iota
	ImpactPatch
	ImpactMinor
	ImpactMajor
)

func (r ReleaseImpact) String() string {
	switch r {
	case ImpactMajor:
		return "major"
	case ImpactMinor:
		return "minor"
	case ImpactPatch:
		return "patch"
	default:
		return "none"
	}
}

// ImpactResult is the inferred bump plus the human reasons behind it. The reasons
// matter as much as the bump — they are shown verbatim in the report.
type ImpactResult struct {
	Impact  ReleaseImpact
	Reasons []string
}

// InferImpact combines commit signals with OpenSpec requirement deltas into an
// advisory bump, taking the maximum across all signals so any breaking signal
// wins. When hasBaseline is false (no accepted baseline yet), every delta is
// necessarily ADDED, so the spec-delta signal is capped at minor and a major can
// come only from a commit breaking marker.
//
// commitTypeImpact maps a conventional type to its impact; pass nil for the
// fixed default (feat→minor, fix→patch, everything else→none).
func InferImpact(commits []Commit, deltas []OpenSpecDelta, hasBaseline bool, commitTypeImpact map[string]ReleaseImpact) ImpactResult {
	if commitTypeImpact == nil {
		commitTypeImpact = defaultTypeImpact
	}
	res := ImpactResult{Impact: ImpactNone}

	for _, c := range commits {
		if c.Breaking {
			res.raise(ImpactMajor, fmt.Sprintf("breaking: %s", commitOneLine(c)))
			continue
		}
		if !c.ConventionalOK {
			continue
		}
		if imp, ok := commitTypeImpact[c.Type]; ok && imp > ImpactNone {
			res.raise(imp, fmt.Sprintf("%s: %s", c.Type, c.Description))
		}
	}

	for _, d := range deltas {
		imp, reason := deltaImpact(d, hasBaseline)
		if imp > ImpactNone {
			res.raise(imp, reason)
		}
	}
	return res
}

var defaultTypeImpact = map[string]ReleaseImpact{
	"feat": ImpactMinor,
	"fix":  ImpactPatch,
}

// deltaImpact maps an OpenSpec requirement delta to its impact. Before the first
// baseline everything is ADDED, so REMOVED/MODIFIED cannot occur and the signal
// caps at minor.
func deltaImpact(d OpenSpecDelta, hasBaseline bool) (ReleaseImpact, string) {
	switch d.Operation {
	case "REMOVED":
		if !hasBaseline {
			return ImpactNone, ""
		}
		return ImpactMajor, fmt.Sprintf("removed requirement in %s", d.Spec)
	case "MODIFIED":
		if !hasBaseline {
			return ImpactNone, ""
		}
		return ImpactPatch, fmt.Sprintf("modified requirement in %s", d.Spec)
	case "ADDED":
		return ImpactMinor, fmt.Sprintf("added requirement in %s", d.Spec)
	default:
		return ImpactNone, ""
	}
}

func (r *ImpactResult) raise(to ReleaseImpact, reason string) {
	if to > r.Impact {
		r.Impact = to
	}
	if reason == "" {
		return
	}
	for _, existing := range r.Reasons { // dedup identical reasons (e.g. N added requirements in one spec)
		if existing == reason {
			return
		}
	}
	r.Reasons = append(r.Reasons, reason)
}

func commitOneLine(c Commit) string {
	if c.ConventionalOK && c.Type != "" {
		if c.Scope != "" {
			return fmt.Sprintf("%s(%s): %s", c.Type, c.Scope, c.Description)
		}
		return fmt.Sprintf("%s: %s", c.Type, c.Description)
	}
	return c.Description
}
