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
	// Records are NUL-terminated: <status>\0<path>\0 or <status>\0<old>\0<new>\0.
	raw := "M\x00internal/keen/browse.go\x00A\x00internal/keen/history.go\x00R100\x00old/path.go\x00new/path.go\x00"
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

// TestParseChangedFilesRawPaths pins that the -z NUL format carries raw
// (never quote-escaped) pathnames, so names with spaces, quotes, tabs, or
// non-ASCII bytes are preserved verbatim.
func TestParseChangedFilesRawPaths(t *testing.T) {
	raw := "A\x00日本語/函数.go\x00" +
		"A\x00has quote\"name.txt\x00" +
		"A\x00name with space.txt\x00"
	files, err := parseChangedFiles(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}
	if files[0].Path != "日本語/函数.go" {
		t.Errorf("unicode Path = %q", files[0].Path)
	}
	if files[1].Path != `has quote"name.txt` {
		t.Errorf("quoted Path = %q", files[1].Path)
	}
	if files[2].Path != "name with space.txt" {
		t.Errorf("spaced Path = %q", files[2].Path)
	}
}

func TestParseChangedFilesEmpty(t *testing.T) {
	files, err := parseChangedFiles("")
	if err != nil {
		t.Fatalf("empty input must not error, got %v", err)
	}
	if len(files) != 0 {
		t.Errorf("empty input produced %d files, want 0", len(files))
	}
}

// TestParseCommitHistoryEmpty pins that an empty history source yields no
// commits and no error: the repository simply has no history to present.
func TestParseCommitHistoryEmpty(t *testing.T) {
	commits, err := parseCommitHistory("")
	if err != nil {
		t.Fatalf("empty input must not error, got %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("empty input produced %d commits, want 0", len(commits))
	}

	whitespace, err := parseCommitHistory("\n\n \n")
	if err != nil {
		t.Fatalf("whitespace-only input must not error, got %v", err)
	}
	if len(whitespace) != 0 {
		t.Errorf("whitespace-only input produced %d commits, want 0", len(whitespace))
	}
}

// TestParseCommitHistoryRootCommit pins that a commit with no parent is
// represented with an empty Parents slice, never a fabricated parent.
func TestParseCommitHistoryRootCommit(t *testing.T) {
	raw := "abc1234567890abcdef1234567890abcdef123456" + commitFieldSep +
		"initial commit" + commitFieldSep +
		"first body" + commitFieldSep +
		"Root Author" + commitFieldSep +
		"2026-01-01T00:00:00+00:00" + commitFieldSep +
		"Root Committer" + commitFieldSep +
		"2026-01-01T00:00:00+00:00" + commitFieldSep +
		"" + commitRecordSep // empty parents field
	commits, err := parseCommitHistory(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	if len(commits[0].Parents) != 0 {
		t.Errorf("root commit Parents = %v, want empty", commits[0].Parents)
	}
}

// TestParseCommitHistoryMergeCommit pins that a merge commit's multiple parents
// are all retained without assuming exactly one parent.
func TestParseCommitHistoryMergeCommit(t *testing.T) {
	raw := "def4567890abcdef1234567890abcdef123456789" + commitFieldSep +
		"Merge branch 'feature'" + commitFieldSep +
		"merged" + commitFieldSep +
		"Merger Person" + commitFieldSep +
		"2026-03-15T10:30:00+00:00" + commitFieldSep +
		"Merger Person" + commitFieldSep +
		"2026-03-15T10:30:00+00:00" + commitFieldSep +
		"parentAAA parentBBB parentCCC" + commitRecordSep
	commits, err := parseCommitHistory(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	want := []string{"parentAAA", "parentBBB", "parentCCC"}
	if len(commits[0].Parents) != 3 {
		t.Fatalf("merge Parents = %v, want %v", commits[0].Parents, want)
	}
	for i, p := range want {
		if commits[0].Parents[i] != p {
			t.Errorf("parent[%d] = %q, want %q", i, commits[0].Parents[i], p)
		}
	}
}

// TestParseCommitHistoryMultipleCommits pins that multiple records are split
// into independent commits in order.
func TestParseCommitHistoryMultipleCommits(t *testing.T) {
	c1 := "1111111111111111111111111111111111111111" + commitFieldSep +
		"first" + commitFieldSep + "" + commitFieldSep +
		"A One" + commitFieldSep + "2026-01-01T00:00:00+00:00" + commitFieldSep +
		"A One" + commitFieldSep + "2026-01-01T00:00:00+00:00" + commitFieldSep +
		"" + commitRecordSep
	c2 := "2222222222222222222222222222222222222222" + commitFieldSep +
		"second" + commitFieldSep + "" + commitFieldSep +
		"B Two" + commitFieldSep + "2026-01-02T00:00:00+00:00" + commitFieldSep +
		"B Two" + commitFieldSep + "2026-01-02T00:00:00+00:00" + commitFieldSep +
		"1111111111111111111111111111111111111111" + commitRecordSep
	commits, err := parseCommitHistory(c1 + c2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
	if commits[0].Hash != "1111111111111111111111111111111111111111" || commits[1].Hash != "2222222222222222222222222222222222222222" {
		t.Errorf("slug hashes = %q, %q", commits[0].Hash, commits[1].Hash)
	}
}

// TestParseCommitHistorySpecialCharacters pins that subjects, authors, and
// bodies containing quotes, apostrophes, punctuation, tabs, and Unicode are
// preserved exactly (the machine-readable separators make boundaries
// unambiguous; no escaping is introduced).
func TestParseCommitHistorySpecialCharacters(t *testing.T) {
	subject := "feat: \"quoted\" & it's 'apostrophe' — em-dash 日本語"
	author := "O'Brien\twith\ttabs"
	body := "line one\nhttps://example.com?q=1&r=2\n日本語 text\n\"nested quotes\""
	raw := "3333333333333333333333333333333333333333" + commitFieldSep +
		subject + commitFieldSep +
		body + commitFieldSep +
		author + commitFieldSep +
		"2026-02-02T02:02:02+00:00" + commitFieldSep +
		"Committer" + commitFieldSep +
		"2026-02-02T02:02:02+00:00" + commitFieldSep +
		"4444444444444444444444444444444444444444" + commitRecordSep
	commits, err := parseCommitHistory(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	c := commits[0]
	if c.Subject != subject {
		t.Errorf("Subject = %q, want %q", c.Subject, subject)
	}
	if c.Author != author {
		t.Errorf("Author = %q, want %q", c.Author, author)
	}
	if c.Body != body {
		t.Errorf("Body = %q, want %q", c.Body, body)
	}
}

// TestParseCommitHistoryMalformedRecord pins that an unexpected number of
// fields is reported as an error rather than silently mis-parsed.
func TestParseCommitHistoryMalformedRecord(t *testing.T) {
	// Only 3 fields instead of the required 8.
	raw := "abc" + commitFieldSep + "subject" + commitFieldSep + "body" + commitRecordSep
	if _, err := parseCommitHistory(raw); err == nil {
		t.Error("malformed record with wrong field count must return an error")
	}
}
