package keen

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initTestGitRepo initializes a git repo in dir with standard test user identity.
func initTestGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGitCommand(t, dir, "init", "-q")
	runGitCommand(t, dir, "config", "user.email", "test@keen.test")
	runGitCommand(t, dir, "config", "user.name", "Keen Test")
}

// runGitCommand executes a git command in dir, failing the test on error.
func runGitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", cmdArgs...)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v failed in %s: %v", args, dir, err)
	}
}

// runGitOutput executes a git command in dir and returns stdout, failing on error.
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

// mustCreateGitRepo creates a real git repo at path for discovery tests.
func mustCreateGitRepo(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("failed to mkdir %s: %v", path, err)
	}
	initTestGitRepo(t, path)
}

// writeFileAndCommit writes content to file in dir, stages it, commits with message,
// and returns the commit hash.
func writeFileAndCommit(t *testing.T, dir, file, content, message string) string {
	t.Helper()
	filePath := filepath.Join(dir, file)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("failed to mkdir for %s: %v", filePath, err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", filePath, err)
	}
	runGitCommand(t, dir, "add", file)
	runGitCommand(t, dir, "commit", "-m", message, "-q")
	out := runGitOutput(t, dir, "rev-parse", "HEAD")
	if len(out) >= 40 {
		return out[:40]
	}
	return out
}
