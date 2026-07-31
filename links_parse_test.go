package specsync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseLinksMD_BlockedBy(t *testing.T) {
	dir := t.TempDir()
	changeDir := filepath.Join(dir, "test-change")
	os.MkdirAll(changeDir, 0o755)

	linksMD := `## Related
- owner/repo#1

## Blocked by
- owner/repo#2
- owner/repo#3
`
	os.WriteFile(filepath.Join(changeDir, "links.md"), []byte(linksMD), 0o644)

	links, blockedBy, blocks := parseLinksMD(changeDir, "")
	if len(links) != 1 {
		t.Errorf("expected 1 link, got %d", len(links))
	}
	if len(blockedBy) != 2 {
		t.Errorf("expected 2 blocked-by refs, got %d", len(blockedBy))
	}
	if len(blocks) != 0 {
		t.Errorf("expected 0 blocks refs, got %d", len(blocks))
	}
	if blockedBy[0].ID != "2" || blockedBy[1].ID != "3" {
		t.Errorf("blocked-by IDs = %q, %q, want 2, 3", blockedBy[0].ID, blockedBy[1].ID)
	}
}

func TestParseLinksMD_Blocks(t *testing.T) {
	dir := t.TempDir()
	changeDir := filepath.Join(dir, "test-change")
	os.MkdirAll(changeDir, 0o755)

	linksMD := `## Related
- owner/repo#1

## Blocks
- owner/repo#4
`
	os.WriteFile(filepath.Join(changeDir, "links.md"), []byte(linksMD), 0o644)

	links, blockedBy, blocks := parseLinksMD(changeDir, "")
	if len(links) != 1 {
		t.Errorf("expected 1 link, got %d", len(links))
	}
	if len(blockedBy) != 0 {
		t.Errorf("expected 0 blocked-by refs, got %d", len(blockedBy))
	}
	if len(blocks) != 1 {
		t.Errorf("expected 1 blocks ref, got %d", len(blocks))
	}
	if blocks[0].ID != "4" {
		t.Errorf("blocks ID = %q, want 4", blocks[0].ID)
	}
}

func TestParseLinksMD_AllSections(t *testing.T) {
	dir := t.TempDir()
	changeDir := filepath.Join(dir, "test-change")
	os.MkdirAll(changeDir, 0o755)

	linksMD := `## Related
- owner/repo#1

## Blocked by
- owner/repo#2

## Blocks
- owner/repo#3
`
	os.WriteFile(filepath.Join(changeDir, "links.md"), []byte(linksMD), 0o644)

	links, blockedBy, blocks := parseLinksMD(changeDir, "")
	if len(links) != 1 || blockedBy[0].ID != "2" || blocks[0].ID != "3" {
		t.Errorf("links=%d, blockedBy=%q, blocks=%q, want 1, 2, 3", len(links), blockedBy[0].ID, blocks[0].ID)
	}
}

func TestParseLinksMD_NoSectionHeader(t *testing.T) {
	// Backward compat: entries with no section header go to Links
	dir := t.TempDir()
	changeDir := filepath.Join(dir, "test-change")
	os.MkdirAll(changeDir, 0o755)

	linksMD := `- owner/repo#1
- owner/repo#2
`
	os.WriteFile(filepath.Join(changeDir, "links.md"), []byte(linksMD), 0o644)

	links, blockedBy, blocks := parseLinksMD(changeDir, "")
	if len(links) != 2 {
		t.Errorf("expected 2 links, got %d", len(links))
	}
	if len(blockedBy) != 0 || len(blocks) != 0 {
		t.Errorf("expected empty blockedBy/blocks, got %d/%d", len(blockedBy), len(blocks))
	}
}

func TestParseLinksMD_FullURLs(t *testing.T) {
	dir := t.TempDir()
	changeDir := filepath.Join(dir, "test-change")
	os.MkdirAll(changeDir, 0o755)

	linksMD := `## Blocked by
- https://github.com/owner/repo/issues/42
`
	os.WriteFile(filepath.Join(changeDir, "links.md"), []byte(linksMD), 0o644)

	_, blockedBy, _ := parseLinksMD(changeDir, "")
	if len(blockedBy) != 1 {
		t.Errorf("expected 1 blocked-by ref, got %d", len(blockedBy))
	}
	if blockedBy[0].ID != "42" {
		t.Errorf("blocked-by ID = %q, want 42", blockedBy[0].ID)
	}
}

func TestParseLinksMD_EmptySections(t *testing.T) {
	dir := t.TempDir()
	changeDir := filepath.Join(dir, "test-change")
	os.MkdirAll(changeDir, 0o755)

	linksMD := `## Related
- owner/repo#1

## Blocked by

## Blocks
- owner/repo#2
`
	os.WriteFile(filepath.Join(changeDir, "links.md"), []byte(linksMD), 0o644)

	links, blockedBy, blocks := parseLinksMD(changeDir, "")
	if len(links) != 1 {
		t.Errorf("expected 1 link, got %d", len(links))
	}
	if len(blockedBy) != 0 {
		t.Errorf("expected 0 blocked-by refs, got %d", len(blockedBy))
	}
	if len(blocks) != 1 {
		t.Errorf("expected 1 blocks ref, got %d", len(blocks))
	}
}

func TestParseLinksMD_NoFile(t *testing.T) {
	links, blockedBy, blocks := parseLinksMD("/nonexistent/dir", "")
	if links != nil || blockedBy != nil || blocks != nil {
		t.Error("expected nil results for missing file")
	}
}
