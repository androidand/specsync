package specsync

import "testing"

func TestSlugify(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{
			name:  "normal short title unchanged",
			title: "Add worktree support",
			want:  "add-worktree-support",
		},
		{
			name:  "long title truncated on a word boundary",
			title: "Multi-entity integrations: Create all has no selection UI and company name display is inconsistent across pages",
			want:  "multi-entity-integrations-create-all-has-no",
		},
		{
			name:  "regression case from the report (portal#4331, 103 chars)",
			title: `Multi-entity integrations: "Create all" has no selection UI, and company-name display is inconsistent across pages`,
			want:  "multi-entity-integrations-create-all-has-no",
		},
		{
			name:  "first word alone exceeds the cap: hard-truncate rather than empty",
			title: "Supercalifragilisticexpialidocioussupercalifragilisticexpialidocious even more trailing words",
			want:  "supercalifragilisticexpialidocioussupercalifragi",
		},
		{
			name:  "only punctuation returns empty",
			title: "!!! --- ???",
			want:  "",
		},
		{
			name:  "leading and trailing punctuation trimmed",
			title: "--Fix login--",
			want:  "fix-login",
		},
		{
			name:  "consecutive punctuation runs collapse to one hyphen",
			title: "foo   ---___   bar",
			want:  "foo-bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slugify(tt.title)
			if got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.title, got, tt.want)
			}
			if len(got) > maxSlugLen {
				t.Errorf("slugify(%q) = %q, length %d exceeds maxSlugLen %d", tt.title, got, len(got), maxSlugLen)
			}
		})
	}
}

func TestSlugifyNeverEmitsTrailingOrLeadingHyphen(t *testing.T) {
	titles := []string{
		"a title exactly at the boundary of forty eight characters here",
		"---leading and trailing---",
		"word-boundary-cutoff-should-not-leave-a-trailing-hyphen-behind",
	}
	for _, title := range titles {
		got := slugify(title)
		if got == "" {
			continue
		}
		if got[0] == '-' || got[len(got)-1] == '-' {
			t.Errorf("slugify(%q) = %q has a leading or trailing hyphen", title, got)
		}
	}
}

func TestSlugifyDistinctLongTitlesCanCollideOnTruncation(t *testing.T) {
	a := slugify("Add field forms for the portal onboarding wizard step one only")
	b := slugify("Add field forms for the portal onboarding wizard step one plus extra")
	if a != b {
		t.Skip("these two examples happen not to collide; collision is a known, accepted risk of truncation, not a guaranteed property")
	}
}

func TestCapSlug(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"under cap unchanged", "short-slug", 48, "short-slug"},
		{"exact cap unchanged", "exactly-forty-eight-characters-long-slug-abcdef", 48, "exactly-forty-eight-characters-long-slug-abcdef"},
		{"truncate at word boundary", "one-two-three-four-five-six-seven-eight-nine-ten-eleven", 20, "one-two-three-four"},
		{"single word exceeding cap hard-truncates", "supercalifragilisticexpialidocious", 10, "supercalif"},
		{"truncation landing on a hyphen trims it", "abcde-fghij-klmno", 11, "abcde"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := capSlug(tt.in, tt.max)
			if got != tt.want {
				t.Errorf("capSlug(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
			}
		})
	}
}
