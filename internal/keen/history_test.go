package keen

import (
	"testing"
)

func TestParseCommitHistory(t *testing.T) {
	// Format: %H%x1f%s%x1f%b%x1f%an%x1f%ai%x1f%cn%x1f%ci%x1f%P%x1e
	raw := "abc1234567890abcdef1234567890abcdef123456" + commitFieldSep +
		"feat: add feature" + commitFieldSep +
		"detailed body text\nwith multiple lines" + commitFieldSep +
		"Alice Author" + commitFieldSep +
		"2026-08-31T12:00:00+00:00" + commitFieldSep +
		"Bob Committer" + commitFieldSep +
		"2026-08-31T13:00:00+00:00" + commitFieldSep +
		"parent123 parent456" + commitRecordSep

	commits, err := parseCommitHistory(raw)
	if err != nil {
		t.Fatalf("unexpected error parsing commit history: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	c := commits[0]
	if c.Hash != "abc1234567890abcdef1234567890abcdef123456" {
		t.Errorf("Hash = %q", c.Hash)
	}
	if c.Subject != "feat: add feature" {
		t.Errorf("Subject = %q", c.Subject)
	}
	if c.Body != "detailed body text\nwith multiple lines" {
		t.Errorf("Body = %q", c.Body)
	}
	if c.Author != "Alice Author" {
		t.Errorf("Author = %q", c.Author)
	}
	if len(c.Parents) != 2 || c.Parents[0] != "parent123" || c.Parents[1] != "parent456" {
		t.Errorf("Parents = %v", c.Parents)
	}
}

func TestParseChangedFiles(t *testing.T) {
	raw := "M\tinternal/keen/browse.go\nA\tinternal/keen/history.go\nR100\told/path.go\tnew/path.go"
	files, err := parseChangedFiles(raw)
	if err != nil {
		t.Fatalf("unexpected error parsing changed files: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}

	if files[0].Status != "M" || files[0].Path != "internal/keen/browse.go" || files[0].OldPath != "" {
		t.Errorf("file 0 = %+v", files[0])
	}
	if files[1].Status != "A" || files[1].Path != "internal/keen/history.go" || files[1].OldPath != "" {
		t.Errorf("file 1 = %+v", files[1])
	}
	if files[2].Status != "R" || files[2].Path != "new/path.go" || files[2].OldPath != "old/path.go" {
		t.Errorf("file 2 = %+v", files[2])
	}
}
