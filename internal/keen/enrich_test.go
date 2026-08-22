package keen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnrich(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping git integration test: git executable not found")
	}

	t.Run("clean repository with commit", func(t *testing.T) {
		repoDir := t.TempDir()
		initTestGitRepo(t, repoDir)

		if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# test"), 0644); err != nil {
			t.Fatal(err)
		}
		runGitCommand(t, repoDir, "add", "README.md")
		runGitCommand(t, repoDir, "commit", "-m", "initial commit")

		repo := Repository{Path: repoDir}
		if err := Enrich(&repo); err != nil {
			t.Fatalf("Enrich() error = %v", err)
		}

		if repo.Dirty {
			t.Errorf("expected repo to be clean, got Dirty = true")
		}
		if repo.Name != filepath.Base(repoDir) {
			t.Errorf("expected Name = %q, got %q", filepath.Base(repoDir), repo.Name)
		}
		if repo.Branch == "" {
			t.Errorf("expected branch name to be populated, got empty")
		}
		if repo.LastCommitTime == "No commits" || repo.LastCommitTime == "" {
			t.Errorf("expected valid last commit time, got %q", repo.LastCommitTime)
		}
		if repo.LastCommitAt.IsZero() {
			t.Errorf("expected LastCommitAt to be non-zero for committed repository")
		}
	})

	t.Run("dirty repository with untracked file", func(t *testing.T) {
		repoDir := t.TempDir()
		initTestGitRepo(t, repoDir)

		if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# test"), 0644); err != nil {
			t.Fatal(err)
		}
		runGitCommand(t, repoDir, "add", "README.md")
		runGitCommand(t, repoDir, "commit", "-m", "initial commit")

		if err := os.WriteFile(filepath.Join(repoDir, "untracked.txt"), []byte("dirty"), 0644); err != nil {
			t.Fatal(err)
		}

		repo := Repository{Path: repoDir}
		if err := Enrich(&repo); err != nil {
			t.Fatalf("Enrich() error = %v", err)
		}

		if !repo.Dirty {
			t.Errorf("expected repo to be dirty, got Dirty = false")
		}
		if repo.LastCommitAt.IsZero() {
			t.Errorf("expected LastCommitAt to be non-zero for dirty repository with commit")
		}
	})

	t.Run("committed repository has hash and subject matching git HEAD", func(t *testing.T) {
		repoDir := t.TempDir()
		initTestGitRepo(t, repoDir)

		if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
		runGitCommand(t, repoDir, "add", "file.txt")
		runGitCommand(t, repoDir, "commit", "-m", "implement repository enrichment")

		wantHash := strings.TrimSpace(runGitOutput(t, repoDir, "rev-parse", "HEAD"))
		wantSubject := strings.TrimSpace(runGitOutput(t, repoDir, "log", "-1", "--format=%s"))

		repo := Repository{Path: repoDir}
		if err := Enrich(&repo); err != nil {
			t.Fatalf("Enrich() error = %v", err)
		}

		if repo.LastCommitHash != wantHash {
			t.Errorf("LastCommitHash = %q, want %q", repo.LastCommitHash, wantHash)
		}
		if repo.LastCommitSubject != wantSubject {
			t.Errorf("LastCommitSubject = %q, want %q", repo.LastCommitSubject, wantSubject)
		}
		if repo.Upstream != "" {
			t.Errorf("expected empty Upstream for local repo without remote, got %q", repo.Upstream)
		}
	})

	t.Run("repository with no commits", func(t *testing.T) {
		repoDir := t.TempDir()
		initTestGitRepo(t, repoDir)

		repo := Repository{Path: repoDir}
		if err := Enrich(&repo); err != nil {
			t.Fatalf("Enrich() error = %v", err)
		}

		if repo.LastCommitTime != "No commits" {
			t.Errorf("expected LastCommitTime = %q, got %q", "No commits", repo.LastCommitTime)
		}
		if !repo.LastCommitAt.IsZero() {
			t.Errorf("expected LastCommitAt.IsZero() = true for repo with no commits, got %v", repo.LastCommitAt)
		}
		if repo.LastCommitHash != "" {
			t.Errorf("expected empty LastCommitHash for repo with no commits, got %q", repo.LastCommitHash)
		}
		if repo.LastCommitSubject != "" {
			t.Errorf("expected empty LastCommitSubject for repo with no commits, got %q", repo.LastCommitSubject)
		}
	})

	t.Run("repository without upstream", func(t *testing.T) {
		repoDir := t.TempDir()
		initTestGitRepo(t, repoDir)

		if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
		runGitCommand(t, repoDir, "add", "file.txt")
		runGitCommand(t, repoDir, "commit", "-m", "commit")

		repo := Repository{Path: repoDir}
		if err := Enrich(&repo); err != nil {
			t.Fatalf("Enrich() error = %v", err)
		}

		if repo.Ahead != 0 || repo.Behind != 0 {
			t.Errorf("expected Ahead=0, Behind=0 without upstream, got Ahead=%d, Behind=%d", repo.Ahead, repo.Behind)
		}
		if repo.Upstream != "" {
			t.Errorf("expected empty Upstream without upstream, got %q", repo.Upstream)
		}
		if repo.LastCommitAt.IsZero() {
			t.Errorf("expected LastCommitAt to be non-zero for repo without upstream")
		}
	})

	t.Run("enrich non-existent path returns error", func(t *testing.T) {
		repo := Repository{Path: filepath.Join(t.TempDir(), "non-existent-repo-path")}
		err := Enrich(&repo)
		if err == nil {
			t.Errorf("expected error when enriching non-existent path, got nil")
		}
	})

	t.Run("LastCommitAt matches git commit timestamp", func(t *testing.T) {
		repoDir := t.TempDir()
		initTestGitRepo(t, repoDir)

		if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
		runGitCommand(t, repoDir, "add", "file.txt")
		runGitCommand(t, repoDir, "commit", "-m", "commit")

		want := runGitOutput(t, repoDir, "log", "-1", "--format=%cI")
		parsedWant, err := time.Parse(time.RFC3339, strings.TrimSpace(want))
		if err != nil {
			t.Fatalf("failed to parse reference git timestamp %q: %v", want, err)
		}

		repo := Repository{Path: repoDir}
		if err := Enrich(&repo); err != nil {
			t.Fatalf("Enrich() error = %v", err)
		}

		if !repo.LastCommitAt.Equal(parsedWant) {
			t.Errorf("LastCommitAt = %v, want git commit timestamp %v", repo.LastCommitAt, parsedWant)
		}
	})

	t.Run("detached HEAD reports empty branch", func(t *testing.T) {
		repoDir := t.TempDir()
		initTestGitRepo(t, repoDir)

		if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
		runGitCommand(t, repoDir, "add", "file.txt")
		runGitCommand(t, repoDir, "commit", "-m", "commit")

		hash := strings.TrimSpace(runGitOutput(t, repoDir, "rev-parse", "HEAD"))
		runGitCommand(t, repoDir, "checkout", "--detach", hash)

		repo := Repository{Path: repoDir}
		if err := Enrich(&repo); err != nil {
			t.Fatalf("Enrich() error = %v", err)
		}

		if repo.Branch != "" {
			t.Errorf("expected detached HEAD to report empty Branch, got %q", repo.Branch)
		}
		if repo.Name != filepath.Base(repoDir) {
			t.Errorf("expected Name = %q, got %q", filepath.Base(repoDir), repo.Name)
		}
		if repo.Dirty {
			t.Errorf("expected detached HEAD repository to be clean, got Dirty = true")
		}
	})

	t.Run("diverged repository parses ahead and behind", func(t *testing.T) {
		bare := t.TempDir()
		runGitCommand(t, bare, "init", "--bare", "-q")

		work := t.TempDir()
		runGitCommand(t, work, "init", "-q")
		runGitCommand(t, work, "config", "user.email", "test@keen.test")
		runGitCommand(t, work, "config", "user.name", "Keen Test")
		if err := os.WriteFile(filepath.Join(work, "base.txt"), []byte("base"), 0644); err != nil {
			t.Fatal(err)
		}
		runGitCommand(t, work, "add", "base.txt")
		runGitCommand(t, work, "commit", "-m", "base")
		runGitCommand(t, work, "branch", "-M", "main")
		runGitCommand(t, work, "remote", "add", "origin", bare)
		runGitCommand(t, work, "push", "-q", "-u", "origin", "main")

		// A second clone advances the remote so that work is behind.
		peer := t.TempDir()
		runGitCommand(t, peer, "clone", "-q", bare, ".")
		runGitCommand(t, peer, "config", "user.email", "test@keen.test")
		runGitCommand(t, peer, "config", "user.name", "Keen Test")
		if err := os.WriteFile(filepath.Join(peer, "remote.txt"), []byte("remote"), 0644); err != nil {
			t.Fatal(err)
		}
		runGitCommand(t, peer, "add", "remote.txt")
		runGitCommand(t, peer, "commit", "-m", "remote-only")
		runGitCommand(t, peer, "push", "-q", "origin", "main")

		// Refresh work's remote-tracking ref so @{upstream} reflects the peer push.
		runGitCommand(t, work, "fetch", "-q", "origin")

		// work commits locally without pulling, so it is ahead AND behind.
		if err := os.WriteFile(filepath.Join(work, "local.txt"), []byte("local"), 0644); err != nil {
			t.Fatal(err)
		}
		runGitCommand(t, work, "add", "local.txt")
		runGitCommand(t, work, "commit", "-m", "local-only")

		repo := Repository{Path: work}
		if err := Enrich(&repo); err != nil {
			t.Fatalf("Enrich() error = %v", err)
		}

		if repo.Ahead <= 0 || repo.Behind <= 0 {
			t.Errorf("expected diverged repository (Ahead > 0 && Behind > 0), got Ahead=%d Behind=%d", repo.Ahead, repo.Behind)
		}
		if repo.Upstream != "origin/main" {
			t.Errorf("expected Upstream = %q, got %q", "origin/main", repo.Upstream)
		}
	})
}

func initTestGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGitCommand(t, dir, "init", "-q")
	runGitCommand(t, dir, "config", "user.email", "test@keen.test")
	runGitCommand(t, dir, "config", "user.name", "Keen Test")
}

func runGitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", cmdArgs...)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v failed in %s: %v", args, dir, err)
	}
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", cmdArgs...)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v failed in %s: %v", args, dir, err)
	}
	return string(output)
}
