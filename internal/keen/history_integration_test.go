package keen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests exercise loadCommitHistory and loadChangedFiles against real
// `git log --format` and `git diff-tree --name-status` output, so the parser
// contracts are verified on the shapes Git actually emits — not only on
// synthetic strings. They are integration tests and skip when git is absent.

func TestIntegrationCommitHistory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping git integration test: git executable not found")
	}

	t.Run("multiline body and special-char subject", func(t *testing.T) {
		dir := t.TempDir()
		initTestGitRepo(t, dir)
		if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("a\n"), 0644); err != nil {
			t.Fatal(err)
		}
		runGitCommand(t, dir, "add", "f.txt")
		// A subject and body with punctuation, quotes, and non-ASCII.
		message := "feat: \"quoted\" — 日本語\n\nFirst line\n\nSecond line: ok & so on"
		runGitCommand(t, dir, "commit", "-m", message)

		commits, err := loadCommitHistory(dir)
		if err != nil {
			t.Fatalf("loadCommitHistory error: %v", err)
		}
		if len(commits) != 1 {
			t.Fatalf("expected 1 commit, got %d", len(commits))
		}
		c := commits[0]
		if c.Subject != "feat: \"quoted\" — 日本語" {
			t.Errorf("Subject = %q", c.Subject)
		}
		if !strings.Contains(c.Body, "First line") || !strings.Contains(c.Body, "Second line: ok & so on") {
			t.Errorf("Body multiline = %q", c.Body)
		}
		if len(c.Parents) != 0 {
			t.Errorf("root commit Parents = %v, want empty", c.Parents)
		}
		if c.Author == "" || c.AuthorDate == "" {
			t.Errorf("expected author and date populated: %+v", c)
		}
	})

	t.Run("empty body", func(t *testing.T) {
		dir := t.TempDir()
		initTestGitRepo(t, dir)
		if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("a\n"), 0644); err != nil {
			t.Fatal(err)
		}
		runGitCommand(t, dir, "add", "f.txt")
		runGitCommand(t, dir, "commit", "-m", "no body")

		commits, err := loadCommitHistory(dir)
		if err != nil {
			t.Fatalf("loadCommitHistory error: %v", err)
		}
		if len(commits) != 1 || commits[0].Subject != "no body" || commits[0].Body != "" {
			t.Errorf("empty body commit mis-parsed: %+v", commits)
		}
	})

	t.Run("merge commit retains all parents", func(t *testing.T) {
		dir := t.TempDir()
		initTestGitRepo(t, dir)
		if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0644); err != nil {
			t.Fatal(err)
		}
		runGitCommand(t, dir, "add", "base.txt")
		runGitCommand(t, dir, "commit", "-m", "base")
		runGitCommand(t, dir, "branch", "-M", "main")

		// Feature branch with a commit.
		runGitCommand(t, dir, "checkout", "-b", "feature", "-q")
		if err := os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feat\n"), 0644); err != nil {
			t.Fatal(err)
		}
		runGitCommand(t, dir, "add", "feature.txt")
		runGitCommand(t, dir, "commit", "-m", "feature work")

		// Merge back into main (no fast-forward so it is a true merge commit).
		runGitCommand(t, dir, "checkout", "main", "-q")
		runGitCommand(t, dir, "merge", "--no-ff", "feature", "-m", "merge feature")

		commits, err := loadCommitHistory(dir)
		if err != nil {
			t.Fatalf("loadCommitHistory error: %v", err)
		}
		// The merge commit is the most recent.
		merge := commits[0]
		if len(merge.Parents) != 2 {
			t.Errorf("merge commit Parents = %v, want 2 parents", merge.Parents)
		}
	})

	t.Run("empty repository yields no error", func(t *testing.T) {
		dir := t.TempDir()
		initTestGitRepo(t, dir)
		commits, err := loadCommitHistory(dir)
		if err != nil {
			t.Fatalf("empty repo loadCommitHistory error: %v", err)
		}
		if len(commits) != 0 {
			t.Errorf("empty repo produced %d commits, want 0", len(commits))
		}
	})
}

func TestIntegrationChangedFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping git integration test: git executable not found")
	}

	t.Run("added and modified files", func(t *testing.T) {
		dir := t.TempDir()
		initTestGitRepo(t, dir)
		hash := writeFileAndCommit(t, dir, "a.txt", "one\n", "add a")

		hash2 := writeFileAndCommit(t, dir, "a.txt", "two\n", "edit a")

		files, err := loadChangedFiles(dir, hash)
		if err != nil {
			t.Fatalf("loadChangedFiles(add) error: %v", err)
		}
		if len(files) != 1 || files[0].Status != "A" || files[0].Path != "a.txt" {
			t.Errorf("add commit files = %+v", files)
		}

		files2, err := loadChangedFiles(dir, hash2)
		if err != nil {
			t.Fatalf("loadChangedFiles(edit) error: %v", err)
		}
		if len(files2) != 1 || files2[0].Status != "M" || files2[0].Path != "a.txt" {
			t.Errorf("edit commit files = %+v", files2)
		}
	})

	t.Run("rename captures old and new path", func(t *testing.T) {
		dir := t.TempDir()
		initTestGitRepo(t, dir)
		// Identical content guarantees 100% similarity so git reports R100.
		writeFileAndCommit(t, dir, "old.go", "package old\nconst version = 1\n", "add old")
		runGitCommand(t, dir, "mv", "old.go", "new.go")
		renameHash := writeFileAndCommit(t, dir, "new.go", "package old\nconst version = 1\n", "rename old to new")

		files, err := loadChangedFiles(dir, renameHash)
		if err != nil {
			t.Fatalf("loadChangedFiles(rename) error: %v", err)
		}
		if len(files) != 1 {
			t.Fatalf("expected 1 rename record, got %+v", files)
		}
		f := files[0]
		if f.Status != "R" {
			t.Errorf("rename Status = %q, want R (full record %+v)", f.Status, f)
		}
		if f.OldPath != "old.go" || f.Path != "new.go" {
			t.Errorf("rename paths = old %q new %q", f.OldPath, f.Path)
		}
	})

	t.Run("unicode path preserved", func(t *testing.T) {
		dir := t.TempDir()
		initTestGitRepo(t, dir)
		hash := writeFileAndCommit(t, dir, "日本語/函数.go", "package x\n", "unicode path")

		files, err := loadChangedFiles(dir, hash)
		if err != nil {
			t.Fatalf("loadChangedFiles(unicode) error: %v", err)
		}
		if len(files) != 1 {
			t.Fatalf("expected 1 file, got %+v", files)
		}
		if files[0].Path != "日本語/函数.go" {
			t.Errorf("unicode Path = %q", files[0].Path)
		}
	})
}

// writeFileAndCommit writes content to relPath under dir and commits it,
// returning the resulting commit hash.
func writeFileAndCommit(t *testing.T, dir, relPath, content, message string) string {
	t.Helper()
	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, dir, "add", "-A")
	runGitCommand(t, dir, "commit", "-m", message)
	return strings.TrimSpace(runGitOutput(t, dir, "rev-parse", "HEAD"))
}
