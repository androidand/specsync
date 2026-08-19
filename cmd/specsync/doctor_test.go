package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

// writeFakeOpenspec creates a fake "openspec" executable in dir that prints
// script to stdout and exits with exitCode, then points $PATH at dir (and
// nowhere else) for the duration of the test.
func writeFakeOpenspec(t *testing.T, script string, exitCode int) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake openspec script uses a POSIX shebang")
	}

	dir := t.TempDir()
	binPath := filepath.Join(dir, "openspec")
	content := "#!/bin/sh\n"
	if script != "" {
		content += "printf '%s' \"" + script + "\"\n"
	}
	content += "exit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(binPath, []byte(content), 0o755); err != nil {
		t.Fatalf("write fake openspec: %v", err)
	}
	t.Setenv("PATH", dir)
}

func TestCheckOpenspecBinary_Present_ParseableVersion(t *testing.T) {
	writeFakeOpenspec(t, "1.5.0", 0)

	dep := checkOpenspecBinary()

	if !dep.Found {
		t.Fatalf("Found = false, want true")
	}
	if dep.Name != "openspec" {
		t.Errorf("Name = %q, want %q", dep.Name, "openspec")
	}
	if dep.Path == "" {
		t.Errorf("Path is empty, want resolved binary path")
	}
	if dep.Version != "1.5.0" {
		t.Errorf("Version = %q, want %q", dep.Version, "1.5.0")
	}
}

func TestCheckOpenspecBinary_Present_UnparseableVersion(t *testing.T) {
	writeFakeOpenspec(t, "openspec version 1.5.0 (build abc)", 0)

	dep := checkOpenspecBinary()

	if !dep.Found {
		t.Fatalf("Found = false, want true (binary is on PATH even if version output is unparseable)")
	}
	if dep.Version != "" {
		t.Errorf("Version = %q, want empty for unparseable output", dep.Version)
	}
}

func TestCheckOpenspecBinary_Present_VersionCommandFails(t *testing.T) {
	writeFakeOpenspec(t, "", 1)

	dep := checkOpenspecBinary()

	if !dep.Found {
		t.Fatalf("Found = false, want true (binary is on PATH even if --version fails)")
	}
	if dep.Version != "" {
		t.Errorf("Version = %q, want empty when --version fails", dep.Version)
	}
}

func TestCheckOpenspecBinary_Absent(t *testing.T) {
	dir := t.TempDir() // empty dir, no openspec binary
	t.Setenv("PATH", dir)

	dep := checkOpenspecBinary()

	if dep.Found {
		t.Fatalf("Found = true, want false when openspec is not on PATH")
	}
	if dep.Path != "" {
		t.Errorf("Path = %q, want empty when not found", dep.Path)
	}
	if dep.Version != "" {
		t.Errorf("Version = %q, want empty when not found", dep.Version)
	}
}

// TestDoctorClaudeJSON_AdditiveFields guards against the dependency-check
// addition breaking existing `doctor claude --json` consumers: every
// previously-existing top-level field must still be present with its
// original meaning, alongside the new "dependencies" field.
func TestDoctorClaudeJSON_AdditiveFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir) // openspec absent, so Dependencies[0].Found = false
	t.Setenv("HOME", t.TempDir())

	out := captureStdout(t, func() { doctorClaude(true) })

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("unmarshal doctor claude --json output: %v\noutput: %s", err, out)
	}

	for _, field := range []string{"status", "installation", "token_analysis", "recommendations"} {
		if _, ok := parsed[field]; !ok {
			// omitempty fields (installation/token_analysis/recommendations) may be
			// absent when empty; status must always be present.
			if field == "status" {
				t.Errorf("missing pre-existing field %q in doctor claude --json output", field)
			}
		}
	}

	depsRaw, ok := parsed["dependencies"]
	if !ok {
		t.Fatalf("missing new field %q in doctor claude --json output", "dependencies")
	}
	var deps []DependencyInfo
	if err := json.Unmarshal(depsRaw, &deps); err != nil {
		t.Fatalf("unmarshal dependencies: %v", err)
	}
	if len(deps) != 1 || deps[0].Name != "openspec" {
		t.Errorf("dependencies = %+v, want one entry named %q", deps, "openspec")
	}
	if deps[0].Found {
		t.Errorf("dependencies[0].Found = true, want false (openspec not on PATH in this test)")
	}
}

// TestDoctorInstallJSON_EmitsStructuredOutput pins the pre-existing gap this
// change closes: `doctor install --json` previously ignored --json entirely
// and always printed prose. It must now emit structured JSON, including the
// dependency-check fields.
func TestDoctorInstallJSON_EmitsStructuredOutput(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir) // openspec absent
	t.Setenv("HOME", t.TempDir())

	out := captureStdout(t, func() { doctorInstall(true) })

	var result InstallResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal doctor install --json output: %v\noutput: %s", err, out)
	}
	if len(result.Locations) == 0 {
		t.Errorf("Locations is empty, want per-agent skill locations")
	}
	if len(result.Dependencies) != 1 || result.Dependencies[0].Name != "openspec" {
		t.Errorf("Dependencies = %+v, want one entry named %q", result.Dependencies, "openspec")
	}
}

// captureStdout redirects os.Stdout for the duration of fn and returns what
// was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}

	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return string(buf)
}
