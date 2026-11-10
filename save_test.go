package specsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func saveOpts(t *testing.T, dir string, msg string) SaveOptions {
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	return SaveOptions{Dir: dir, Message: msg, OpenRequest: func(context.Context, string) (int, error) { return 0, nil }}
}

func TestSaveCommitsTrackedChangesAndNeverStagesUntrackedFiles(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	os.WriteFile(filepath.Join(clone, "a.txt"), []byte("changed\n"), 0o644)
	os.WriteFile(filepath.Join(clone, "new.txt"), []byte("new\n"), 0o644)
	res, err := Save(context.Background(), saveOpts(t, clone, "update a"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Committed || res.Pushed {
		t.Fatalf("a plain save commits and does not push: %+v", res)
	}
	if got := gitFixture(t, clone, "show", "--name-only", "--format=", "HEAD"); got != "a.txt" {
		t.Fatalf("only the tracked file may be committed, got %q", got)
	}
	if st := gitFixture(t, clone, "status", "--porcelain"); !strings.Contains(st, "?? new.txt") {
		t.Fatalf("the untracked file must stay untracked: %q", st)
	}
}

func TestSaveAddsOnlyNamedUntrackedFiles(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	os.WriteFile(filepath.Join(clone, "new.txt"), []byte("new\n"), 0o644)
	os.WriteFile(filepath.Join(clone, "other.txt"), []byte("other\n"), 0o644)
	o := saveOpts(t, clone, "add new")
	o.Add = []string{"new.txt"}
	if _, err := Save(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if got := gitFixture(t, clone, "show", "--name-only", "--format=", "HEAD"); got != "new.txt" {
		t.Fatalf("got %q", got)
	}
	o.Add = []string{"../outside.txt"}
	if _, err := Save(context.Background(), o); err == nil {
		t.Fatal("a path outside the worktree must be refused")
	}
}

func TestSaveRefusesSecretLookingFilesByNameAndContent(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	for name, content := range map[string]string{
		".env.local":  "A=1\n",
		"deploy.pem":  "x\n",
		"notes.txt":   "token ghp_" + strings.Repeat("a", 36) + "\n",
		"key.txt":     "-----BEGIN RSA PRIVATE KEY-----\nabc\n",
		"id_ed25519":  "x\n",
		"creds.cfg":   "ok\n",
		".env.sample": "A=\n",
	} {
		os.WriteFile(filepath.Join(clone, name), []byte(content), 0o644)
	}
	cases := map[string]bool{".env.local": true, "deploy.pem": true, "notes.txt": true, "key.txt": true, "id_ed25519": true, "creds.cfg": false, ".env.sample": false}
	for name, refused := range cases {
		o := saveOpts(t, clone, "x")
		o.Add = []string{name}
		_, err := Save(context.Background(), o)
		if refused && (err == nil || !strings.Contains(err.Error(), name)) {
			t.Errorf("%s must be refused, got %v", name, err)
		}
		if !refused && err != nil {
			t.Errorf("%s must be allowed, got %v", name, err)
		}
	}
}

func TestSaveRefusesConfiguredFilesAndTrailersAndNamesThem(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	os.WriteFile(filepath.Join(clone, "dump.sql"), []byte("x\n"), 0o644)
	o := saveOpts(t, clone, "add dump")
	o.Add = []string{"dump.sql"}
	o.Ways = Ways{Save: SaveConfig{ForbidFiles: []string{"*.sql"}}}
	if _, err := Save(context.Background(), o); err == nil || !strings.Contains(err.Error(), "dump.sql") {
		t.Fatalf("got %v", err)
	}
	os.WriteFile(filepath.Join(clone, "a.txt"), []byte("c\n"), 0o644)
	o = saveOpts(t, clone, "fix\n\nCo-Authored-By: Someone <s@example.com>")
	o.Ways = Ways{Save: SaveConfig{ForbidTrailers: []string{"co-authored-by"}}}
	if _, err := Save(context.Background(), o); err == nil || !strings.Contains(err.Error(), "Co-Authored-By") {
		t.Fatalf("a forbidden trailer must be refused by name, got %v", err)
	}
	if n := gitFixture(t, clone, "rev-list", "--count", "origin/feature..HEAD"); n != "0" {
		t.Fatalf("nothing may be committed on a refusal, got %s", n)
	}
}

func TestSavePushesOnlyWhenAskedAndRefusesAnOpenRequest(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	os.WriteFile(filepath.Join(clone, "a.txt"), []byte("c\n"), 0o644)
	o := saveOpts(t, clone, "work")
	o.Push = true
	o.OpenRequest = func(context.Context, string) (int, error) { return 12, nil }
	res, err := Save(context.Background(), o)
	if err == nil || !strings.Contains(err.Error(), "#12") || !strings.Contains(err.Error(), "-push-to-request") {
		t.Fatalf("an open request blocks the push: %v", err)
	}
	if res.Committed || res.Pushed {
		t.Fatalf("a refused push guard stops before anything is committed: %+v", res)
	}
	if n := gitFixture(t, clone, "rev-list", "--count", "origin/feature..HEAD"); n != "0" {
		t.Fatalf("nothing may be committed, ahead=%s", n)
	}
	o.PushToRequest = true
	res, err = Save(context.Background(), o)
	if err != nil || !res.Pushed {
		t.Fatalf("explicit permission pushes: %v %+v", err, res)
	}
	if n := gitFixture(t, clone, "rev-list", "--count", "origin/feature..HEAD"); n != "0" {
		t.Fatalf("remote should be current, ahead=%s", n)
	}
}

func TestSavePushRefusesWhenTheRequestStateIsUnknown(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	os.WriteFile(filepath.Join(clone, "a.txt"), []byte("c\n"), 0o644)
	o := saveOpts(t, clone, "work")
	o.Push = true
	o.OpenRequest = nil
	if _, err := Save(context.Background(), o); err == nil || !strings.Contains(err.Error(), "cannot tell") {
		t.Fatalf("got %v", err)
	}
}

func TestSavePushSetsAnUpstreamForANewBranchAndNeverForces(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	gitFixture(t, clone, "checkout", "-q", "-b", "feat/new")
	os.WriteFile(filepath.Join(clone, "a.txt"), []byte("c\n"), 0o644)
	o := saveOpts(t, clone, "work")
	o.Push = true
	if res, err := Save(context.Background(), o); err != nil || !res.Pushed {
		t.Fatalf("%v %+v", err, res)
	}
	if up := gitFixture(t, clone, "rev-parse", "--abbrev-ref", "@{u}"); up != "origin/feat/new" {
		t.Fatalf("upstream = %q", up)
	}
}

func TestSaveWithNothingToCommitAndDryRun(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	if _, err := Save(context.Background(), saveOpts(t, clone, "x")); err == nil || !strings.Contains(err.Error(), "nothing to commit") {
		t.Fatalf("got %v", err)
	}
	os.WriteFile(filepath.Join(clone, "a.txt"), []byte("c\n"), 0o644)
	o := saveOpts(t, clone, "x")
	o.DryRun = true
	o.Push = true
	res, err := Save(context.Background(), o)
	if err != nil || res.Committed || res.Pushed || len(res.Files) != 1 {
		t.Fatalf("a dry run reports and changes nothing: %v %+v", err, res)
	}
	if gitFixture(t, clone, "rev-list", "--count", "origin/feature..HEAD") != "0" {
		t.Fatal("dry run committed")
	}
	if _, err := Save(context.Background(), saveOpts(t, clone, "  ")); err == nil {
		t.Fatal("an empty message must be refused")
	}
}

func TestSavePushNeverGoesToTheUpstreamNameWhenItDiffersFromTheBranch(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	gitFixture(t, clone, "checkout", "-q", "-b", "work", "origin/main")
	gitFixture(t, clone, "config", "push.default", "upstream")
	os.WriteFile(filepath.Join(clone, "a.txt"), []byte("c\n"), 0o644)
	mainBefore := gitFixture(t, clone, "rev-parse", "origin/main")
	o := saveOpts(t, clone, "work")
	o.Push = true
	if res, err := Save(context.Background(), o); err != nil || !res.Pushed {
		t.Fatalf("%v %+v", err, res)
	}
	if got := gitFixture(t, clone, "ls-remote", "origin", "refs/heads/main"); !strings.HasPrefix(got, mainBefore) {
		t.Fatalf("main on the remote moved: %q (was %s)", got, mainBefore)
	}
	if got := gitFixture(t, clone, "ls-remote", "origin", "refs/heads/work"); got == "" {
		t.Fatal("the branch must be pushed under its own name")
	}
	if up := gitFixture(t, clone, "rev-parse", "--abbrev-ref", "@{u}"); up != "origin/work" {
		t.Fatalf("the upstream must be fixed to the branch's own name, got %q", up)
	}
}

func TestSaveFromASubdirectoryUsesRepoRootPathsForTheSecretScanAndAdd(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	sub := filepath.Join(clone, "sub")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "key.txt"), []byte("ok\n"), 0o644)
	gitFixture(t, clone, "add", "sub/key.txt")
	gitFixture(t, clone, "commit", "-q", "-m", "key file")
	os.WriteFile(filepath.Join(sub, "key.txt"), []byte("-----BEGIN RSA PRIVATE KEY-----\nabc\n"), 0o644)
	o := saveOpts(t, sub, "x")
	if _, err := Save(context.Background(), o); err == nil || !strings.Contains(err.Error(), "sub/key.txt") {
		t.Fatalf("a private key in a tracked file under the cwd must be refused, got %v", err)
	}
	os.WriteFile(filepath.Join(sub, "key.txt"), []byte("ok\n"), 0o644)
	os.WriteFile(filepath.Join(sub, "new.txt"), []byte("new\n"), 0o644)
	o.Add = []string{"new.txt"}
	res, err := Save(context.Background(), o)
	if err != nil || len(res.Files) != 1 || res.Files[0] != "sub/new.txt" {
		t.Fatalf("-add is relative to the cwd and files are reported from the root: %v %+v", err, res)
	}
}

func commitOnBranch(t *testing.T, clone string) {
	t.Helper()
	os.WriteFile(filepath.Join(clone, "a.txt"), []byte("changed\n"), 0o644)
}

func TestSaveAddUsesLiteralPathspecsSoGlobsStageOnlyTheNamedFile(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	for _, n := range []string{"c.txt", "d.txt", "[cd].txt", "x*", "xa.txt"} {
		os.WriteFile(filepath.Join(clone, n), []byte(n+"\n"), 0o644)
	}
	o := saveOpts(t, clone, "add the literal names")
	o.Add = []string{"[cd].txt", "x*"}
	res, err := Save(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	got := gitFixture(t, clone, "show", "--name-only", "--format=", "HEAD")
	if got != "[cd].txt\nx*" && got != "x*\n[cd].txt" {
		t.Fatalf("only the literal names may be committed, got %q", got)
	}
	if len(res.Files) != 2 {
		t.Fatalf("the scanned set must equal the staged set: %+v", res.Files)
	}
}

func TestSaveScansOnlyAddedLinesOfTrackedFilesAndWholeNewFiles(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	token := "ghp_" + strings.Repeat("a", 36)
	os.WriteFile(filepath.Join(clone, "fixture.txt"), []byte("old "+token+"\nline\n"), 0o644)
	gitFixture(t, clone, "add", "fixture.txt")
	gitFixture(t, clone, "commit", "-q", "-m", "old fixture")
	os.WriteFile(filepath.Join(clone, "fixture.txt"), []byte("old "+token+"\nline changed\n"), 0o644)
	if _, err := Save(context.Background(), saveOpts(t, clone, "touch another line")); err != nil {
		t.Fatalf("an old fixture must not block forever: %v", err)
	}
	os.WriteFile(filepath.Join(clone, "fixture.txt"), []byte("old "+token+"\nline changed\nnew "+token+"\n"), 0o644)
	if _, err := Save(context.Background(), saveOpts(t, clone, "add a token")); err == nil || !strings.Contains(err.Error(), "fixture.txt") {
		t.Fatalf("a newly added token must be refused: %v", err)
	}
}

func TestSaveRefusesFilesItCannotScanUnlessAllowedAndLogsTheAllowance(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	os.WriteFile(filepath.Join(clone, "big.bin"), []byte(strings.Repeat("x", 1<<20+1)), 0o644)
	o := saveOpts(t, clone, "add big")
	o.Add = []string{"big.bin"}
	if _, err := Save(context.Background(), o); err == nil || !strings.Contains(err.Error(), "could not scan") {
		t.Fatalf("got %v", err)
	}
	os.WriteFile(filepath.Join(clone, ".env.local"), []byte("A=1\n"), 0o644)
	o.Add = []string{"big.bin", ".env.local"}
	o.Allow = []string{"big.bin", ".env.local"}
	res, err := Save(context.Background(), o)
	if err != nil || len(res.Allowed) != 2 {
		t.Fatalf("an explicit, logged allowance overrides a reviewed false positive: %v %+v", err, res)
	}
}

func TestSaveScansTheCommitMessageForSecrets(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	commitOnBranch(t, clone)
	o := saveOpts(t, clone, "rotate ghp_"+strings.Repeat("b", 36))
	if _, err := Save(context.Background(), o); err == nil || !strings.Contains(err.Error(), "message") {
		t.Fatalf("got %v", err)
	}
}

func TestSaveWidenedSecretNamesAndTokenPatterns(t *testing.T) {
	for _, name := range []string{".git-credentials", "credentials", "prod.tfstate", "prod.tfvars", "keys.p12", "cert.pfx", "app.keystore", "id_rsa", "id_ed25519"} {
		if secretReason(name, nil) == "" {
			t.Errorf("%s must look like a secret by name", name)
		}
	}
	for _, tok := range []string{"xoxb-1234567890-abcdefghij", "sk_live_" + strings.Repeat("a", 24), "rk_live_" + strings.Repeat("a", 24),
		"AIza" + strings.Repeat("a", 35), "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abcdefghijk", "AKIA" + strings.Repeat("A", 16),
		"-----BEGIN OPENSSH PRIVATE KEY-----"} {
		if !ScanSecrets(tok) {
			t.Errorf("%q must be recognised as a secret", tok)
		}
	}
	if ScanSecrets("a normal sentence about sk and tokens") {
		t.Error("prose is not a secret")
	}
}

func TestSaveRefusesADetachedHeadBeforeCommitting(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	gitFixture(t, clone, "checkout", "-q", "--detach")
	commitOnBranch(t, clone)
	before := gitFixture(t, clone, "rev-parse", "HEAD")
	if _, err := Save(context.Background(), saveOpts(t, clone, "x")); err == nil || !strings.Contains(err.Error(), "detached") {
		t.Fatalf("got %v", err)
	}
	if gitFixture(t, clone, "rev-parse", "HEAD") != before {
		t.Fatal("nothing may be committed on a detached HEAD")
	}
}

func TestSavePushFromTheDefaultBranchNeedsPushToRequestAndCommitsNothingOtherwise(t *testing.T) {
	clone, _ := repoWithOrigin(t)
	gitFixture(t, clone, "checkout", "-q", "main")
	commitOnBranch(t, clone)
	o := saveOpts(t, clone, "to main")
	o.Push = true
	if res, err := Save(context.Background(), o); err == nil || !strings.Contains(err.Error(), "default branch") || res.Committed {
		t.Fatalf("got %v %+v", err, res)
	}
	if n := gitFixture(t, clone, "rev-list", "--count", "origin/main..HEAD"); n != "0" {
		t.Fatalf("nothing may be committed, ahead=%s", n)
	}
	o.PushToRequest = true
	if res, err := Save(context.Background(), o); err != nil || !res.Pushed {
		t.Fatalf("explicit permission: %v %+v", err, res)
	}
}
