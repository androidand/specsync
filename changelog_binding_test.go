package specsync

import "testing"

// Pins how BuildChangelog binds a commit to a change, because the two cases
// below are indistinguishable in the gate's error message and are not the same
// situation:
//
//   - "(#42)" in a squash-merge header, where 42 IS the change's issue id.
//     extractRefs files a header "(#N)" under PRRefs, and BuildChangelog
//     consults append(IssueRefs, PRRefs...) against issueToChange, so this DOES
//     bind. The form the gate recommends is the form that works.
//   - "(#131)" where 131 is a PR number and not any change's issue. Nothing
//     binds, and the commit is correctly reported as loose.
//
// A reader of the error message cannot tell these apart: both print a "(#N)"
// and both are rejected or accepted on grounds the message never states. That
// ambiguity is what makes 57c7e49 look self-contradictory rather than merely
// unlinked.
func TestBindingDistinguishesHeaderRefFromPRRef(t *testing.T) {
	t.Run("header ref binds when the number is the change's issue", func(t *testing.T) {
		in := TraceInput{
			Commits: []Commit{{
				Hash: "abc1234", Type: "feat", ConventionalOK: true,
				Description: "something", PRRefs: []string{"#42"},
			}},
			Changes: []ChangeRefs{
				{Change: Change{Slug: "my-change"}, IssueIDs: []string{"42"}},
			},
		}
		cl := BuildChangelog(in, nil)
		if !hasBoundEntry(cl, "my-change") {
			t.Fatalf("header (#42) should bind to the change whose issue is 42; entries=%+v", cl.Entries)
		}
	})

	t.Run("header ref does not bind when the number is only a PR", func(t *testing.T) {
		in := TraceInput{
			Commits: []Commit{{
				Hash: "57c7e49", Type: "feat", ConventionalOK: true,
				Description: "something", PRRefs: []string{"#131"},
			}},
			Changes: []ChangeRefs{
				{Change: Change{Slug: "my-change"}, IssueIDs: []string{"42"}},
			},
		}
		cl := BuildChangelog(in, nil)
		if hasBoundEntry(cl, "my-change") {
			t.Fatal("(#131) is not this change's issue and must not bind")
		}
		if !hasLooseEntry(cl, "57c7e49") {
			t.Fatalf("expected a loose entry for the unbound commit; entries=%+v", cl.Entries)
		}
	})
}

func hasBoundEntry(cl Changelog, slug string) bool {
	for _, e := range cl.Entries {
		if e.Slug == slug && e.Hash == "" {
			return true
		}
	}
	return false
}

func hasLooseEntry(cl Changelog, hash string) bool {
	for _, e := range cl.Entries {
		if e.Hash == hash && e.Slug == "" {
			return true
		}
	}
	return false
}
