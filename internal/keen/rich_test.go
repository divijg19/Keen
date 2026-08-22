package keen

import (
	"strings"
	"testing"
)

func TestRenderRichColumns(t *testing.T) {
	repos := []Repository{
		{
			Name:              "Peony",
			Branch:            "v0.8.x",
			Dirty:             false,
			Ahead:             0,
			Behind:            0,
			Upstream:          "origin/v0.8.x",
			LastCommitHash:    "8f31ac2a1b",
			LastCommitSubject: "tighten memory format",
			LastCommitTime:    "2 days ago",
		},
		{
			Name:              "Zinnia",
			Branch:            "main",
			Dirty:             true,
			Ahead:             2,
			Behind:            1,
			Upstream:          "origin/main",
			LastCommitHash:    "a1b2c3d4e5",
			LastCommitSubject: "wip refactor",
			LastCommitTime:    "5 hours ago",
		},
		{
			Name:   "Tulip",
			Branch: "",
			Dirty:  false,
			Ahead:  0,
			Behind: 0,
		},
	}

	out := renderRich(repos, len(repos), 160)

	for _, col := range []string{"STATUS", "NAME", "BRANCH", "UPSTREAM", "AHEAD", "BEHIND", "HASH", "SUBJECT", "TIME"} {
		if !strings.Contains(out, col) {
			t.Errorf("rich report missing column header %q: %q", col, out)
		}
	}

	// Status, name, branch, upstream, ahead/behind, short hash, subject, time.
	if !strings.Contains(out, "clean") || !strings.Contains(out, "dirty") {
		t.Errorf("expected status values: %q", out)
	}
	if !strings.Contains(out, "Peony") || !strings.Contains(out, "Zinnia") || !strings.Contains(out, "Tulip") {
		t.Errorf("expected repository names: %q", out)
	}
	if !strings.Contains(out, "v0.8.x") || !strings.Contains(out, "main") {
		t.Errorf("expected branch tokens: %q", out)
	}
	if !strings.Contains(out, "origin/v0.8.x") || !strings.Contains(out, "origin/main") {
		t.Errorf("expected upstream values: %q", out)
	}
	if !strings.Contains(out, "detached") {
		t.Errorf("expected detached branch label: %q", out)
	}
	if !strings.Contains(out, "8f31ac2") {
		t.Errorf("expected short hash: %q", out)
	}
	if !strings.Contains(out, "tighten memory format") {
		t.Errorf("expected commit subject: %q", out)
	}
	if !strings.Contains(out, "2 days ago") {
		t.Errorf("expected relative time: %q", out)
	}
	// Ahead/behind are presented as separate numeric columns; a synchronized
	// upstream shows the actual divergence.
	if !strings.Contains(out, "2      1") {
		t.Errorf("expected numeric divergence for upstream repo: %q", out)
	}
	// No-upstream row uses em-dash markers, never implying synchronization.
	if !strings.Contains(out, "—") {
		t.Errorf("expected em-dash for absent upstream: %q", out)
	}
}

func TestRenderRichDeterministicTruncation(t *testing.T) {
	long := strings.Repeat("x", 60)
	repo := Repository{
		Name:              strings.Repeat("n", 40),
		Branch:            "main",
		Upstream:          strings.Repeat("u", 40),
		Dirty:             false,
		LastCommitHash:    "abcdef1234567890",
		LastCommitSubject: long,
		LastCommitTime:    "1 day ago",
	}
	out := renderRich([]Repository{repo}, 1, 40)

	if !strings.Contains(out, "…") {
		t.Errorf("expected truncation ellipsis in narrow rich report: %q", out)
	}
	// Canonical subject must remain untouched.
	if repo.LastCommitSubject != long {
		t.Errorf("canonical subject must not be altered, got %q", repo.LastCommitSubject)
	}
	// Full hash must never appear; only the 7-char short form.
	if strings.Contains(out, "abcdef1234567890") {
		t.Errorf("full hash must not appear in rich report: %q", out)
	}
}

func TestRenderRichEmptyStates(t *testing.T) {
	found := renderRich(nil, 0, 40)
	if !strings.Contains(found, "No repositories found.") {
		t.Errorf("expected 'No repositories found.': %q", found)
	}
	filtered := renderRich(nil, 5, 40)
	if !strings.Contains(filtered, "No repositories match the selected filters.") {
		t.Errorf("expected filtered empty message: %q", filtered)
	}
}
