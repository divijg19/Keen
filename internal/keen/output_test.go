package keen

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
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
}
