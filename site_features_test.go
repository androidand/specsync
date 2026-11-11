package specsync

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestSiteFeaturesAllRender pins the site's feature list: site/build.sh only
// renders cards whose group is one of the known themes, so a card with a
// missing or misspelled group parses fine and silently never reaches the page.
func TestSiteFeaturesAllRender(t *testing.T) {
	raw, err := os.ReadFile("site/features.json")
	if err != nil {
		t.Fatal(err)
	}
	var features []struct {
		Group string `json:"group"`
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.Unmarshal(raw, &features); err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{"plan": true, "collaborate": true, "review": true, "ship": true}
	seen := map[string]bool{}
	for i, f := range features {
		if !known[f.Group] {
			t.Errorf("feature %d %q: group %q is not rendered by site/build.sh", i, f.Title, f.Group)
		}
		if strings.TrimSpace(f.Title) == "" || strings.TrimSpace(f.Body) == "" {
			t.Errorf("feature %d: empty title or body", i)
		}
		if seen[f.Title] {
			t.Errorf("feature %d: duplicate title %q", i, f.Title)
		}
		seen[f.Title] = true
	}
}
