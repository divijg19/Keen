package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// gitCommand runs git in dir, failing the test on error.
func gitCommand(t *testing.T, dir string, args ...string) string {
	return gitCommandEnv(t, dir, nil, args...)
}

// gitCommandEnv runs git in dir with extra environment (KEY=VALUE pairs),
// failing the test on error.
func gitCommandEnv(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed in %s: %v\n%s", args, dir, err, out)
	}
	return string(out)
}

// initRepo creates a fresh git repository in dir/name with test identity and
// returns its path.
func initRepo(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	gitCommand(t, root, "init", "-q", dir)
	gitCommand(t, dir, "config", "user.email", "test@keen.test")
	gitCommand(t, dir, "config", "user.name", "Keen Test")
	return dir
}

// commitFile writes content to file in dir and commits it. committerDate, when
// non-empty, backdates the commit's committer date (the timestamp enrichment
// reads) via GIT_COMMITTER_DATE; the author date is backdated in step so the
// two stay consistent.
func commitFile(t *testing.T, dir, file, content, message, committerDate string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
	gitCommand(t, dir, "add", file)
	env := []string{
		"GIT_AUTHOR_DATE=" + committerDate,
		"GIT_COMMITTER_DATE=" + committerDate,
	}
	args := []string{"commit", "-q", "-m", message}
	if committerDate == "" {
		env = nil
	}
	gitCommandEnv(t, dir, env, args...)
}

// dirtyAfterCommit modifies file in dir without committing, leaving the
// working tree dirty.
func dirtyAfterCommit(t *testing.T, dir, file, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
}

// runKeen re-executes the test binary as the real Keen binary against
// workingDir with the given arguments, capturing stdout. The subprocess calls
// main() directly so the full pipeline (parse, discover, enrich, sort, filter,
// resolve, present) runs exactly as in production.
func runKeen(t *testing.T, workingDir string, env []string, args ...string) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	cmd := exec.Command(exe, "-test.run=TestKeenCLIHelper")
	cmd.Dir = workingDir
	cmd.Env = append(os.Environ(), append(env, "KEEN_CLI_HELPER=1", "KEEN_CLI_ARGS="+string(encoded))...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("keen %v failed in %s: %v\nstderr: %s", args, workingDir, err, errb.String())
	}
	return out.String()
}

// hasControlSequence reports whether s carries any terminal escape or control
// output: ANSI ESC sequences, Ctrl+C, or other control characters beyond
// printable text, horizontal tab, and newline. It is the shared gate for
// asserting that redirected non-TTY output stays pure text.
func hasControlSequence(s string) bool {
	for _, r := range s {
		if r == '\x1b' || r == '\x03' || (r < 0x20 && r != '\n' && r != '\t' && r != '\r') {
			return true
		}
	}
	return false
}

// TestKeenCLIHelper is the re-exec entry point shared by all CLI-level tests:
// a helper subprocess that installs the requested arguments and runs the real
// main(), then exits. Output is captured by the parent as the binary's report.
func TestKeenCLIHelper(t *testing.T) {
	if os.Getenv("KEEN_CLI_HELPER") != "1" {
		t.Skip("helper entry point")
	}
	var args []string
	if err := json.Unmarshal([]byte(os.Getenv("KEEN_CLI_ARGS")), &args); err != nil {
		t.Fatalf("bad KEEN_CLI_ARGS: %v", err)
	}
	os.Args = append([]string{"keen"}, args...)
	main()
	os.Exit(0)
}

// TestMainPipelineE2E drives the real CLI end to end against freshly built Git
// workspaces and pins the presentation contracts: canonical status is
// structural (CLEAN/DIRTY section headings), never duplicated as per-row tags;
// the rich report drops the STATUS column; compact flat mode keeps per-row
// tags; the banner precedes every non-interactive report.
func TestMainPipelineE2E(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root, "clean-repo")
	commitFile(t, filepath.Join(root, "clean-repo"), "a.txt", "a\n", "initial", "")
	initRepo(t, root, "dirty-repo")
	commitFile(t, filepath.Join(root, "dirty-repo"), "b.txt", "b\n", "initial", "")
	dirtyAfterCommit(t, filepath.Join(root, "dirty-repo"), "b.txt", "b\nmodified\n")

	cases := []struct {
		name string
		args []string
		want []string
		not  []string
		env  []string
	}{
		{
			name: "canonical grouped",
			args: nil,
			want: []string{"===KEEN===", "    CLEAN\n", "    DIRTY\n", "clean-repo", "dirty-repo"},
			not:  []string{"[clean]", "[dirty]", "STATUS"},
		},
		{
			name: "rich",
			args: []string{"-r"},
			want: []string{"===KEEN===", "    CLEAN\n", "    DIRTY\n", "SUBJECT", "clean-repo", "dirty-repo"},
			not:  []string{"STATUS"},
			env:  []string{"COLUMNS=120"},
		},
		{
			name: "compact flat",
			args: []string{"--compact"},
			want: []string{"===KEEN===", "[clean]", "[dirty]", "clean-repo", "dirty-repo"},
			not:  []string{"\n    CLEAN\n", "\n    DIRTY\n"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := runKeen(t, root, c.env, c.args...)
			for _, w := range c.want {
				if !strings.Contains(out, w) {
					t.Errorf("missing %q in output:\n%s", w, out)
				}
			}
			for _, n := range c.not {
				if strings.Contains(out, n) {
					t.Errorf("unexpected %q in output:\n%s", n, out)
				}
			}
		})
	}
}

// TestNonTTYInteractiveFilterMatrix runs the real binary with -i under a
// pipe (non-terminal stdin) with stdout captured as a pipe, across every
// selection flag, and asserts the portability contract (no terminal
// escape/control sequence in the redirected output) holds with the filters
// genuinely applied end to end.
func TestNonTTYInteractiveFilterMatrix(t *testing.T) {
	type tc struct {
		name string
		args []string
		want []string
		not  []string
	}

	cases := []tc{
		{name: "interactive", args: []string{"-i"}, want: []string{"fresh", "stale"}},
		{name: "clean only", args: []string{"-i", "--clean"}, want: []string{"fresh"}, not: []string{"stale"}},
		{name: "dirty only", args: []string{"-i", "--dirty"}, want: []string{"stale"}, not: []string{"fresh"}},
		{name: "recent only", args: []string{"-i", "--recent", "7d"}, want: []string{"fresh"}, not: []string{"stale"}},
	}

	if id := os.Getenv("KEEN_NONTTY_CASE"); id != "" {
		idx, err := strconv.Atoi(id)
		if err != nil || idx < 0 || idx >= len(cases) {
			t.Fatalf("bad KEEN_NONTTY_CASE %q", id)
		}
		os.Args = append([]string{"keen"}, cases[idx].args...)
		main()
		os.Exit(0)
	}

	root := t.TempDir()
	fresh := initRepo(t, root, "fresh")
	commitFile(t, fresh, "f.txt", "f\n", "recent work", "")
	stale := initRepo(t, root, "stale")
	commitFile(t, stale, "s.txt", "s\n", "old work", "2026-01-15T00:00:00+00:00")
	dirtyAfterCommit(t, stale, "s.txt", "s\nmodified\n")

	for ci, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			exe, err := os.Executable()
			if err != nil {
				t.Fatalf("os.Executable: %v", err)
			}
			cmd := exec.Command(exe, "-test.run=TestNonTTYInteractiveFilterMatrix")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "KEEN_NONTTY_CASE="+strconv.Itoa(ci))
			cmd.Stdin = strings.NewReader("") // pipe, not a TTY
			var out, errb bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &errb
			if err := cmd.Run(); err != nil {
				t.Fatalf("subprocess failed: %v\nstderr: %s", err, errb.String())
			}
			res := out.String()
			if hasControlSequence(res) {
				t.Fatalf("terminal escape/control sequence leaked in %s: %q", c.name, res)
			}
			for _, w := range c.want {
				if !strings.Contains(res, w) {
					t.Errorf("missing %q in %s output:\n%s", w, c.name, res)
				}
			}
			for _, n := range c.not {
				if strings.Contains(res, n) {
					t.Errorf("unexpected %q in %s output:\n%s", n, c.name, res)
				}
			}
		})
	}
}
