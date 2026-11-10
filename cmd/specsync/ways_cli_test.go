package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func waysRepo(t *testing.T, cfg string) (repo string, env []string) {
	t.Helper()
	repo, env = firstUseEnv(t)
	if err := os.MkdirAll(filepath.Join(repo, "openspec", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "openspec", "specsync.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	env = append(env, "SPECSYNC_TRUST_FILE="+filepath.Join(t.TempDir(), "trust.json"))
	return repo, env
}

func TestDoctorConfigReportsTypoWithLine(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "board: acme/1\ngate:\n  - {name: a, run: b}\n")
	out, err := runIn(t, bin, repo, env, "doctor", "config")
	if err == nil {
		t.Fatalf("a config with problems must exit non-zero:\n%s", out)
	}
	if !strings.Contains(out, "line 2") || !strings.Contains(out, `did you mean "gates"`) {
		t.Fatalf("want the typo with its line:\n%s", out)
	}
}

func TestDoctorConfigValidConfigAndDefaults(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "flow: strict\ngates:\n  - {name: t, run: make test}\n")
	out, err := runIn(t, bin, repo, env, "doctor", "config")
	if err != nil || !strings.Contains(out, "config is valid") || !strings.Contains(out, "not trusted") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	bare, benv := firstUseEnv(t)
	out, err = runIn(t, bin, bare, benv, "doctor", "config")
	if err != nil || !strings.Contains(out, "defaults apply") {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

func TestTrustDryRunWritesNothingThenRecords(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "gates:\n  - {name: t, run: make test}\n")
	trustFile := ""
	for _, e := range env {
		if strings.HasPrefix(e, "SPECSYNC_TRUST_FILE=") {
			trustFile = strings.TrimPrefix(e, "SPECSYNC_TRUST_FILE=")
		}
	}
	out, err := runIn(t, bin, repo, env, "trust", "-dry-run")
	if err != nil || !strings.Contains(out, "make test") || !strings.Contains(out, "DRY RUN") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	if _, statErr := os.Stat(trustFile); statErr == nil {
		t.Fatal("a dry run must not record trust")
	}
	if out, err := runIn(t, bin, repo, env, "trust"); err != nil {
		t.Fatalf("trust: %v\n%s", err, out)
	}
	if _, statErr := os.Stat(trustFile); statErr != nil {
		t.Fatalf("trust must record: %v", statErr)
	}
	out, err = runIn(t, bin, repo, env, "doctor", "config")
	if err != nil || strings.Contains(out, "not trusted") {
		t.Fatalf("after trust: err=%v\n%s", err, out)
	}
	if out, err := runIn(t, bin, repo, env, "trust", "-revoke"); err != nil {
		t.Fatalf("revoke: %v\n%s", err, out)
	}
	out, _ = runIn(t, bin, repo, env, "doctor", "config")
	if !strings.Contains(out, "not trusted") {
		t.Fatalf("after revoke:\n%s", out)
	}
}

func TestTrustRefusesABrokenConfig(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	repo, env := waysRepo(t, "gates: make\n")
	out, err := runIn(t, bin, repo, env, "trust")
	if err == nil || !strings.Contains(out, "fix the config first") {
		t.Fatalf("err=%v\n%s", err, out)
	}
}
