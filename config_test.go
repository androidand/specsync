package specsync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Config
	}{
		{
			name:  "retain move",
			input: "retain=move\n",
			want:  Config{RetentionPolicy: RetentionPolicyMove},
		},
		{
			name:  "retain prune",
			input: "retain=prune\n",
			want:  Config{RetentionPolicy: RetentionPolicyPrune},
		},
		{
			name:  "empty",
			input: "",
			want:  Config{},
		},
		{
			name:  "comments and blank lines",
			input: "# retention policy\n\nretain=move\n",
			want:  Config{RetentionPolicy: RetentionPolicyMove},
		},
		{
			name:  "unknown key ignored",
			input: "retain=move\nunknown=value\n",
			want:  Config{RetentionPolicy: RetentionPolicyMove},
		},
		{
			name:  "invalid retain value ignored",
			input: "retain=invalid\n",
			want:  Config{},
		},
		{
			name:  "no equals sign ignored",
			input: "retain move\n",
			want:  Config{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseConfig([]byte(tt.input))
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestReadConfig(t *testing.T) {
	dir := t.TempDir()

	// No config file
	got := ReadConfig(dir)
	if got != (Config{}) {
		t.Errorf("absent config: got %+v, want empty", got)
	}

	// With config file
	configPath := filepath.Join(dir, ConfigPath)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("retain=move\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got = ReadConfig(dir)
	want := Config{RetentionPolicy: RetentionPolicyMove}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestResolveRetentionPolicy(t *testing.T) {
	dir := t.TempDir()

	// Flag wins over config.
	if got := ResolveRetentionPolicy(RetentionPolicyMove, dir); got != RetentionPolicyMove {
		t.Errorf("flag move: got %q, want move", got)
	}

	// Config is used when flag is empty.
	configPath := filepath.Join(dir, ConfigPath)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("retain=prune\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := ResolveRetentionPolicy("", dir); got != RetentionPolicyPrune {
		t.Errorf("config prune: got %q, want prune", got)
	}

	// Heuristic default (prune) when no flag and no config.
	emptyDir := t.TempDir()
	if got := ResolveRetentionPolicy("", emptyDir); got != RetentionPolicyPrune {
		t.Errorf("heuristic default: got %q, want prune", got)
	}
}
