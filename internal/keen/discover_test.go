package keen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover(t *testing.T) {
	tmpDir := t.TempDir()

	repoA := filepath.Join(tmpDir, "repo-a")
	repoB := filepath.Join(tmpDir, "project", "repo-b")
	outerRepo := filepath.Join(tmpDir, "outer-repo")
	innerRepo := filepath.Join(outerRepo, "inner-repo")
	ordinaryDir := filepath.Join(tmpDir, "ordinary-dir")

	mustCreateGitRepo(t, repoA)
	mustCreateGitRepo(t, repoB)
	mustCreateGitRepo(t, outerRepo)
	mustCreateGitRepo(t, innerRepo)

	if err := os.MkdirAll(ordinaryDir, 0755); err != nil {
		t.Fatal(err)
	}

	repos, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	foundPaths := make(map[string]bool)
	for _, r := range repos {
		rel, _ := filepath.Rel(tmpDir, r.Path)
		foundPaths[filepath.ToSlash(rel)] = true
	}

	expected := []string{"repo-a", "project/repo-b", "outer-repo"}
	for _, exp := range expected {
		if !foundPaths[exp] {
			t.Errorf("expected repository %q to be discovered, but was not found", exp)
		}
	}

	if foundPaths["outer-repo/inner-repo"] {
		t.Errorf("nested repository 'outer-repo/inner-repo' should have been skipped via SkipDir pruning")
	}

	if len(repos) != len(expected) {
		t.Errorf("expected %d repositories, got %d (%v)", len(expected), len(repos), repos)
	}
}

func TestDiscover_ConcurrencyStress(t *testing.T) {
	tmpDir := t.TempDir()
	// Create 25 temporary git repos to exercise concurrent discovery under race detector
	for i := 0; i < 25; i++ {
		repoPath := filepath.Join(tmpDir, "group", "repo-"+string(rune('a'+i)))
		mustCreateGitRepo(t, repoPath)
	}

	repos, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover() concurrency stress error = %v", err)
	}
	if len(repos) != 25 {
		t.Errorf("expected 25 repositories, got %d", len(repos))
	}
}

func TestDiscover_NonExistentRoot(t *testing.T) {
	_, err := Discover(filepath.Join(t.TempDir(), "non-existent-root-path"))
	if err == nil {
		t.Errorf("expected error when discovering non-existent root, got nil")
	}
}
