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

// TestWorkspaceSummary pins the orientation line: it counts the presented
// set, pluralizes the noun, and never appears for empty results or in the
// condensed compact mode.
func TestWorkspaceSummary(t *testing.T) {
	tests := []struct {
		name  string
		repos []Repository
		want  string
	}{
		{
			"mixed workspace",
			[]Repository{{Name: "a", Dirty: false}, {Name: "b", Dirty: false}, {Name: "c", Dirty: true}},
			"3 repositories, 2 clean, 1 dirty",
		},
		{
			"all clean",
			[]Repository{{Name: "a", Dirty: false}, {Name: "b", Dirty: false}},
			"2 repositories, 2 clean, 0 dirty",
		},
		{
			"all dirty",
			[]Repository{{Name: "a", Dirty: true}},
			"1 repository, 0 clean, 1 dirty",
		},
		{
			"single clean repository",
			[]Repository{{Name: "a", Dirty: false}},
			"1 repository, 1 clean, 0 dirty",
		},
		{
			"filtered subset counts what is shown",
			[]Repository{{Name: "c", Dirty: true}},
			"1 repository, 0 clean, 1 dirty",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := workspaceSummary(tc.repos); got != tc.want {
				t.Errorf("workspaceSummary = %q, want %q", got, tc.want)
			}
			grouped := renderGrouped(tc.repos)
			if !strings.Contains(grouped, tc.want) {
				t.Errorf("grouped report missing summary %q:\n%s", tc.want, grouped)
			}
			rich := renderRich(tc.repos, len(tc.repos), 200)
			if !strings.Contains(rich, tc.want) {
				t.Errorf("rich report missing summary %q:\n%s", tc.want, rich)
			}
			compact := captureOutput(func() {
				Print(tc.repos, OutputCompact, len(tc.repos))
			})
			if strings.Contains(compact, tc.want) {
				t.Errorf("compact output must stay condensed, found summary %q:\n%s", tc.want, compact)
			}
		})
	}

	t.Run("no summary for empty results", func(t *testing.T) {
		for _, total := range []int{0, 3} {
			grouped := captureOutput(func() {
				Print([]Repository{}, OutputGrouped, total)
			})
			if strings.Contains(grouped, "repositories,") {
				t.Errorf("empty grouped output must not manufacture a summary (total=%d): %q", total, grouped)
			}
			rich := renderRich([]Repository{}, total, 200)
			if strings.Contains(rich, "repositories,") {
				t.Errorf("empty rich output must not manufacture a summary (total=%d): %q", total, rich)
			}
		}
	})
}

// TestRendererFactContract pins one shared repository matrix across all three
// renderers: the same facts must be recognizable in each, in the shape suited
// to its density level (canonical combines, rich splits into columns, the
// interactive list identifies and defers commit detail to deeper surfaces).
// The sync counts 7/9 are deliberately distinctive so the rich columns can
// assert the exact numbers without colliding with other fixture digits.
func TestRendererFactContract(t *testing.T) {
	repos := []Repository{
		{Name: "synced", Branch: "main", Upstream: "origin/main", Dirty: false, LastCommitHash: "abcdef1234567890", LastCommitSubject: "initial commit", LastCommitTime: "1 day ago"},
		{Name: "diverged", Branch: "dev", Upstream: "origin/dev", Ahead: 7, Behind: 9, Dirty: true, LastCommitHash: "1234567abcdef0", LastCommitSubject: "work in progress", LastCommitTime: "2 hours ago"},
		{Name: "detached", Branch: "", Upstream: "", Dirty: false, LastCommitHash: "deadbee0123456", LastCommitSubject: "detached work", LastCommitTime: "3 days ago"},
		{Name: "empty", Branch: "main", Upstream: "", Dirty: false},
	}
	canonical := renderGrouped(repos)
	rich := renderRich(repos, len(repos), 200)
	browseList := renderBrowse(repos, len(repos), BrowseList, 200)

	for _, r := range repos {
		for mode, out := range map[string]string{"canonical": canonical, "rich": rich, "browse": browseList} {
			if !strings.Contains(out, r.Name) {
				t.Errorf("%s output missing repository %q", mode, r.Name)
			}
		}
		if r.LastCommitHash != "" {
			detail := renderDetail(r)
			for mode, out := range map[string]string{"canonical": canonical, "rich": rich, "browse-detail": detail} {
				if !strings.Contains(out, shortHash(r.LastCommitHash)) {
					t.Errorf("%s output missing short hash for %q", mode, r.Name)
				}
				if strings.Contains(out, r.LastCommitHash) {
					t.Errorf("%s output leaks full hash for %q", mode, r.Name)
				}
			}
		}
	}

	// Canonical combines facts into prose labels.
	for _, want := range []string{"main → origin/main", "↑7 ↓9", "detached", "↑– ↓–"} {
		if !strings.Contains(canonical, want) {
			t.Errorf("canonical output missing %q", want)
		}
	}
	// Rich splits the same facts into columns.
	for _, want := range []string{"AHEAD", "BEHIND", "origin/main", "7", "9", "detached", "—"} {
		if !strings.Contains(rich, want) {
			t.Errorf("rich output missing %q", want)
		}
	}
	// The interactive list identifies; it shares aheadBehindLabel with
	// canonical but keeps branch/upstream as separate columns.
	for _, want := range []string{"origin/main", "↑7 ↓9", "detached", "↑– ↓–"} {
		if !strings.Contains(browseList, want) {
			t.Errorf("browse list output missing %q", want)
		}
	}
}
