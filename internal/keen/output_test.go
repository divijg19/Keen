package keen

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRepoStatus(t *testing.T) {
	if got := repoStatus(Repository{Dirty: false}); got != "clean" {
		t.Errorf("repoStatus(clean) = %q, want %q", got, "clean")
	}
	if got := repoStatus(Repository{Dirty: true}); got != "dirty" {
		t.Errorf("repoStatus(dirty) = %q, want %q", got, "dirty")
	}
}

func captureOutput(f func()) string {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()

	f()

	_ = w.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()

	return buf.String()
}

func TestPrint(t *testing.T) {
	t.Run("empty discovery output", func(t *testing.T) {
		out := captureOutput(func() {
			Print([]Repository{}, OutputGrouped, 0)
		})
		if !strings.Contains(out, "No repositories found.") {
			t.Errorf("expected 'No repositories found.', got %q", out)
		}
	})

	t.Run("empty filtered output when repos discovered", func(t *testing.T) {
		out := captureOutput(func() {
			Print([]Repository{}, OutputGrouped, 3)
		})
		if !strings.Contains(out, "No repositories match the selected filters.") {
			t.Errorf("expected 'No repositories match the selected filters.', got %q", out)
		}
		if strings.Contains(out, "No repositories found.") {
			t.Errorf("unexpected 'No repositories found.' when repositories were discovered: %q", out)
		}
	})

	t.Run("grouped mixed output", func(t *testing.T) {
		repos := []Repository{
			{Name: "my-clean", Branch: "main", Dirty: false, LastCommitTime: "1 day ago"},
			{Name: "my-dirty", Branch: "dev", Dirty: true, LastCommitTime: "2 hours ago"},
		}
		out := captureOutput(func() {
			Print(repos, OutputGrouped, len(repos))
		})
		if !strings.Contains(out, "Git Status: CLEAN") {
			t.Errorf("missing CLEAN header in output: %q", out)
		}
		if !strings.Contains(out, "Git Status: DIRTY") {
			t.Errorf("missing DIRTY header in output: %q", out)
		}
		if !strings.Contains(out, "my-clean") || !strings.Contains(out, "my-dirty") {
			t.Errorf("missing repo names in output: %q", out)
		}
	})

	t.Run("grouped clean-only output suppresses dirty header", func(t *testing.T) {
		repos := []Repository{
			{Name: "my-clean", Branch: "main", Dirty: false, LastCommitTime: "1 day ago"},
		}
		out := captureOutput(func() {
			Print(repos, OutputGrouped, len(repos))
		})
		if !strings.Contains(out, "Git Status: CLEAN") {
			t.Errorf("missing CLEAN header: %q", out)
		}
		if strings.Contains(out, "Git Status: DIRTY") {
			t.Errorf("unexpected DIRTY header in clean-only output: %q", out)
		}
	})

	t.Run("compact output", func(t *testing.T) {
		repos := []Repository{
			{Name: "my-clean", Branch: "main", Dirty: false, LastCommitTime: "1 day ago"},
			{Name: "my-dirty", Branch: "dev", Dirty: true, LastCommitTime: "2 hours ago"},
		}
		out := captureOutput(func() {
			Print(repos, OutputCompact, len(repos))
		})
		if strings.Contains(out, "Git Status: CLEAN") || strings.Contains(out, "Git Status: DIRTY") {
			t.Errorf("compact output should not contain section headers: %q", out)
		}
		if !strings.Contains(out, "[clean]") || !strings.Contains(out, "[dirty]") {
			t.Errorf("missing status tags in compact output: %q", out)
		}
		if !strings.Contains(out, "my-clean") || !strings.Contains(out, "my-dirty") {
			t.Errorf("missing repo names in compact output: %q", out)
		}
	})

	t.Run("grouped renders upstream relationship", func(t *testing.T) {
		repos := []Repository{
			{Name: "my-clean", Branch: "main", Upstream: "origin/main", Dirty: false, LastCommitTime: "1 day ago", LastCommitHash: "abcdef1234567890", LastCommitSubject: "initial commit"},
		}
		out := captureOutput(func() {
			Print(repos, OutputGrouped, len(repos))
		})
		if !strings.Contains(out, "main → origin/main") {
			t.Errorf("expected upstream relationship in output: %q", out)
		}
		if !strings.Contains(out, "abcdef1 initial commit") {
			t.Errorf("expected short hash and subject in output: %q", out)
		}
		if strings.Contains(out, "abcdef1234567890") {
			t.Errorf("full hash should not appear in output: %q", out)
		}
	})

	t.Run("compact renders upstream relationship", func(t *testing.T) {
		repos := []Repository{
			{Name: "my-clean", Branch: "main", Upstream: "origin/main", Dirty: false, LastCommitTime: "1 day ago", LastCommitHash: "abcdef1234567890", LastCommitSubject: "initial commit"},
		}
		out := captureOutput(func() {
			Print(repos, OutputCompact, len(repos))
		})
		if !strings.Contains(out, "main → origin/main") {
			t.Errorf("expected upstream relationship in compact output: %q", out)
		}
		if !strings.Contains(out, "abcdef1 initial commit") {
			t.Errorf("expected short hash and subject in compact output: %q", out)
		}
	})

	t.Run("no upstream does not imply synchronization", func(t *testing.T) {
		repos := []Repository{
			{Name: "my-clean", Branch: "main", Upstream: "", Dirty: false, LastCommitTime: "1 day ago", LastCommitHash: "abcdef1234567890", LastCommitSubject: "initial commit"},
		}
		out := captureOutput(func() {
			Print(repos, OutputGrouped, len(repos))
		})
		if strings.Contains(out, "→") {
			t.Errorf("expected no upstream relationship in output: %q", out)
		}
		if !strings.Contains(out, "↑– ↓–") {
			t.Errorf("expected non-numeric no-upstream marker in output: %q", out)
		}
	})

	t.Run("subject truncation is bounded but canonical value untouched", func(t *testing.T) {
		longSubject := strings.Repeat("x", 60)
		repo := Repository{Name: "my-clean", Branch: "main", Upstream: "", Dirty: false, LastCommitTime: "1 day ago", LastCommitHash: "abcdef1234567890", LastCommitSubject: longSubject}
		out := captureOutput(func() {
			Print([]Repository{repo}, OutputGrouped, 1)
		})
		if repo.LastCommitSubject != longSubject {
			t.Errorf("canonical LastCommitSubject must be untouched, got %q", repo.LastCommitSubject)
		}
		if !strings.Contains(out, "…") {
			t.Errorf("expected truncation ellipsis in output: %q", out)
		}
		if strings.Contains(out, strings.Repeat("x", 50)) {
			t.Errorf("displayed subject must be bounded, got: %q", out)
		}
	})

	t.Run("no commits shows no fake commit identity", func(t *testing.T) {
		repos := []Repository{
			{Name: "my-clean", Branch: "main", Upstream: "", Dirty: false, LastCommitTime: "No commits", LastCommitHash: "", LastCommitSubject: ""},
		}
		out := captureOutput(func() {
			Print(repos, OutputGrouped, len(repos))
		})
		if strings.Contains(out, "abcdef") {
			t.Errorf("no-commit repo must not show a hash: %q", out)
		}
	})

	t.Run("commit identity precedes relative time in grouped output", func(t *testing.T) {
		repos := []Repository{
			{Name: "my-clean", Branch: "main", Upstream: "origin/main", Dirty: false, LastCommitTime: "1 day ago", LastCommitHash: "abcdef1234567890", LastCommitSubject: "initial commit"},
		}
		out := captureOutput(func() {
			Print(repos, OutputGrouped, len(repos))
		})
		if !strings.Contains(out, "abcdef1 initial commit | 1 day ago") {
			t.Errorf("expected commit identity before relative time, got: %q", out)
		}
	})

	t.Run("commit identity precedes relative time in compact output", func(t *testing.T) {
		repos := []Repository{
			{Name: "my-clean", Branch: "main", Upstream: "origin/main", Dirty: false, LastCommitTime: "1 day ago", LastCommitHash: "abcdef1234567890", LastCommitSubject: "initial commit"},
		}
		out := captureOutput(func() {
			Print(repos, OutputCompact, len(repos))
		})
		if !strings.Contains(out, "abcdef1 initial commit | 1 day ago") {
			t.Errorf("expected commit identity before relative time, got: %q", out)
		}
	})

	t.Run("Unicode subject truncation is rune-safe", func(t *testing.T) {
		longSubject := strings.Repeat("世", 60)
		repo := Repository{Name: "my-clean", Branch: "main", Upstream: "", Dirty: false, LastCommitTime: "1 day ago", LastCommitHash: "abcdef1234567890", LastCommitSubject: longSubject}
		out := captureOutput(func() {
			Print([]Repository{repo}, OutputGrouped, 1)
		})
		if repo.LastCommitSubject != longSubject {
			t.Errorf("canonical LastCommitSubject must be untouched, got %q", repo.LastCommitSubject)
		}
		if !utf8.ValidString(out) {
			t.Errorf("output must remain valid UTF-8: %q", out)
		}
		if strings.Contains(out, strings.Repeat("世", 50)) {
			t.Errorf("displayed subject must be bounded, got: %q", out)
		}
		if !strings.Contains(out, "世…") {
			t.Errorf("expected complete rune before ellipsis, got: %q", out)
		}
	})

	t.Run("synchronized upstream renders numeric zero divergence", func(t *testing.T) {
		repos := []Repository{
			{Name: "my-clean", Branch: "main", Upstream: "origin/main", Dirty: false, LastCommitTime: "1 day ago", LastCommitHash: "abcdef1234567890", LastCommitSubject: "initial commit"},
		}
		out := captureOutput(func() {
			Print(repos, OutputGrouped, len(repos))
		})
		if !strings.Contains(out, "main → origin/main") {
			t.Errorf("expected upstream relationship in output: %q", out)
		}
		if !strings.Contains(out, "↑0 ↓0") {
			t.Errorf("expected numeric sync marker for synchronized upstream, got: %q", out)
		}
		if strings.Contains(out, "↑– ↓–") {
			t.Errorf("synchronized upstream must not use the no-upstream marker, got: %q", out)
		}
	})

	t.Run("detached HEAD without upstream renders detached", func(t *testing.T) {
		repos := []Repository{
			{Name: "my-clean", Branch: "", Upstream: "", Dirty: false, LastCommitTime: "1 day ago", LastCommitHash: "abcdef1234567890", LastCommitSubject: "initial commit"},
		}
		out := captureOutput(func() {
			Print(repos, OutputGrouped, len(repos))
		})
		if !strings.Contains(out, "detached") {
			t.Errorf("expected detached label, got: %q", out)
		}
		if strings.Contains(out, "→") {
			t.Errorf("detached without upstream must not show a relationship arrow, got: %q", out)
		}
		if !strings.Contains(out, "↑– ↓–") {
			t.Errorf("expected no-upstream marker, got: %q", out)
		}
	})

	t.Run("detached HEAD with upstream renders detached relationship", func(t *testing.T) {
		repos := []Repository{
			{Name: "my-clean", Branch: "", Upstream: "origin/main", Dirty: false, LastCommitTime: "1 day ago", LastCommitHash: "abcdef1234567890", LastCommitSubject: "initial commit"},
		}
		out := captureOutput(func() {
			Print(repos, OutputGrouped, len(repos))
		})
		if !strings.Contains(out, "detached → origin/main") {
			t.Errorf("expected detached-with-upstream label, got: %q", out)
		}
		if !strings.Contains(out, "↑0 ↓0") {
			t.Errorf("expected numeric divergence for configured upstream, got: %q", out)
		}
	})
}
