package specsync

import "testing"

func TestStripParentheticals(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"no parens", "no parens"},
		{"has (one) paren", "has  paren"},
		{"two (a) and (b) here", "two  and  here"},
		{"(leading paren)", ""},
		{"trailing paren) (wait no", "trailing paren "},
		{"nested (a (b) c) end", "nested  end"},
		{"empty () here", "empty  here"},
		{"design: pluggable notification channel (email → sms → push)", "design: pluggable notification channel "},
	}
	for _, tt := range tests {
		got := stripParentheticals(tt.in)
		if got != tt.want {
			t.Errorf("stripParentheticals(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStripBackticks(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"no backticks", "no backticks"},
		{"use `widget-client` here", "use  here"},
		{"`widget-client` generator", " generator"},
		{"Migrate to Widget SDK 7 `widget-client` generator", "Migrate to Widget SDK 7  generator"},
		{"two `a` and `b` here", "two  and  here"},
		{"`nested` `backticks`", " "},
	}
	for _, tt := range tests {
		got := stripBackticks(tt.in)
		if got != tt.want {
			t.Errorf("stripBackticks(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTrimDetailWords(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Migrate to Widget SDK 7 generator", "Migrate to Widget SDK 7"},
		{"Migrate to Widget SDK 7", "Migrate to Widget SDK 7"},
		{"fix the generator function", "fix the generator"},
		{"add a new component", "add a new"},
		{"update the module", "update the"},
		{"use the adapter", "use the"},
		{"use the client", "use the"},
		{"Migrate to Widget SDK 7 widget-client generator", "Migrate to Widget SDK 7 widget-client"},
	}
	for _, tt := range tests {
		got := trimDetailWords(tt.in)
		if got != tt.want {
			t.Errorf("trimDetailWords(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestShortenTitle(t *testing.T) {
	tests := []struct {
		maxLen   int
		in, want string
	}{
		// Simple case: no parens, fits
		{80, "Migrate to Widget SDK 7", "Migrate to Widget SDK 7"},
		// Strip parentheticals + backticks + detail words
		{80, "Migrate to Widget SDK 7 `widget-client` generator (rewrite ~450 imports)", "Migrate to Widget SDK 7"},
		{80, "Design: pluggable notification channel (email → sms → push)", "Design: pluggable notification channel"},
		// (core) prefix is in parens, so it gets stripped — that's correct
		{80, "refactor(core): derive WidgetApiLayer from a runtime WIDGET_RESOURCE_NAMES (invert source of truth)", "refactor: derive WidgetApiLayer from a runtime WIDGET_RESOURCE_NAMES"},
		// ci: title is 91 runes, exceeds 80, so it gets truncated
		{80, "ci: explore path-based / affected-only test selection to avoid running full suite on every commit", "ci: explore path-based / affected-only test selection to avoid running full"},
		// Truncation at word boundary
		{20, "this is a very long title that needs truncation", "this is a very"},
		// Already short after strip
		{40, "fix: handle edge case (minor cleanup)", "fix: handle edge case"},
		// Empty after strip
		{80, "(just a paren)", ""},
		// Leading paren
		{80, "(prefix) actual title", "actual title"},
		// Backtick + detail word combo — "client" isn't trailing, so it stays
		{80, "Use `acmepay` client for API calls", "Use  client for API calls"},
	}
	for _, tt := range tests {
		got := shortenTitle(tt.in, tt.maxLen)
		if got != tt.want {
			t.Errorf("shortenTitle(%q, %d) = %q, want %q", tt.in, tt.maxLen, got, tt.want)
		}
		// Ensure result never exceeds maxLen
		if len(got) > tt.maxLen {
			t.Errorf("shortenTitle(%q, %d) = %q (%d chars) exceeds max %d", tt.in, tt.maxLen, got, len(got), tt.maxLen)
		}
	}
}
