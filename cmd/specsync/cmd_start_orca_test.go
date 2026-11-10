package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestStartLauncher(t *testing.T) {
	path := func(have map[string]string) func(string) (string, error) {
		return func(n string) (string, error) {
			if p, ok := have[n]; ok {
				return p, nil
			}
			return "", exec.ErrNotFound
		}
	}
	noEnv := func(string) string { return "" }
	tests := []struct {
		name, flag, config, goos string
		have                     map[string]string
		env                      func(string) string
		want, err                string
	}{
		{"default is git", "", "", "darwin", map[string]string{"orca": "/bin/orca"}, noEnv, "", ""},
		{"config orca", "", "orca", "darwin", map[string]string{"orca": "/bin/orca"}, noEnv, "/bin/orca", ""},
		{"flag beats config", "git", "orca", "darwin", map[string]string{"orca": "/bin/orca"}, noEnv, "", ""},
		{"auto without orca", "auto", "", "darwin", nil, noEnv, "", ""},
		{"auto on linux with only the screen reader", "auto", "", "linux", map[string]string{"orca": "/usr/bin/orca"}, noEnv, "", ""},
		{"linux orca-ide", "orca", "", "linux", map[string]string{"orca-ide": "/bin/orca-ide", "orca": "/usr/bin/orca"}, noEnv, "/bin/orca-ide", ""},
		{"env wins", "orca", "", "linux", nil, func(k string) string {
			if k == "ORCA_CLI_COMMAND" {
				return "/opt/orca"
			}
			return ""
		}, "/opt/orca", ""},
		{"orca requested but missing", "orca", "", "linux", map[string]string{"orca": "/usr/bin/orca"}, noEnv, "", "screen reader"},
		{"unknown value", "tmux", "", "darwin", nil, noEnv, "", "must be git, orca or auto"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := startLauncher(tt.flag, tt.config, tt.env, path(tt.have), tt.goos)
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("err = %v, want %q", err, tt.err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}
