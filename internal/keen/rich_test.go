package keen

import (
	"fmt"
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

	for _, col := range []string{"NAME", "BRANCH", "UPSTREAM", "AHEAD", "BEHIND", "HASH", "SUBJECT", "TIME"} {
		if !strings.Contains(out, col) {
			t.Errorf("rich report missing column header %q: %q", col, out)
		}
	}
	// Status is structural: the CLEAN/DIRTY section headings are the single
	// signal, so the table itself must not repeat it as a column.
	if strings.Contains(out, "STATUS") {
		t.Errorf("rich report must not contain a STATUS column: %q", out)
	}
	if !strings.Contains(out, "    CLEAN\n") || !strings.Contains(out, "    DIRTY\n") {
		t.Errorf("rich report missing CLEAN/DIRTY section headings: %q", out)
	}

	// Name, branch, upstream, ahead/behind, short hash, subject, time.
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
	out := renderRich([]Repository{repo}, 1, 80)

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

// richMatrixFixture exercises every column, including the no-upstream and
// no-commit presentations.
func richMatrixFixture() []Repository {
	return []Repository{
		{
			Name:              "Peony-longish-name",
			Branch:            "release/v0.8.x",
			Upstream:          "origin/release/v0.8.x",
			Dirty:             false,
			Ahead:             2,
			Behind:            1,
			LastCommitHash:    "8f31ac2a1b",
			LastCommitSubject: "tighten memory format for large repositories",
			LastCommitTime:    "2 days ago",
		},
		{
			Name:   "Zinnia",
			Branch: "main",
			Dirty:  true,
		},
	}
}

func maxLineLength(out string) int {
	maxw := 0
	for _, line := range strings.Split(out, "\n") {
		if w := lineWidth(line); w > maxw {
			maxw = w
		}
	}
	return maxw
}

func hasHeader(cols []richColumn, name string) bool {
	for _, c := range cols {
		if c.header == name {
			return true
		}
	}
	return false
}

func lineWidth(s string) int {
	return stringCellWidth(s)
}

// TestRenderRichWidthMatrix pins the tiered adaptation contract across
// representative widths: no accidental wrapping, indented rule line spans
// exactly the terminal width, correct degradation ladder, atomic
// ahead/behind, and byte determinism. Boundaries include the four-space
// report indent (content budget = terminal width - 4).
//
// Widths are boundary triplets (N-1/N/N+1) around each measured transition
// (fallback|table = 50, TIME-drop|eight-column = 62, wide = 95) rather than
// arbitrary samples. Status is structural: with the STATUS column removed in
// v0.8.1 the table spans fewer columns, so every threshold shifts left and a
// full eight-column table fits from terminal width 62.
func TestRenderRichWidthMatrix(t *testing.T) {
	repos := richMatrixFixture()
	widths := []int{
		40,         // deep fallback probe
		49, 50, 51, // fallback | shallow 7-column table
		61, 62, 63, // 7-column | full eight-column threshold
		94, 95, 96, // tight adaptive | wide proportional
		120, 200, // comfortable wide probes
	}

	for _, width := range widths {
		t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
			out := renderRich(repos, len(repos), width)

			cols := richLayout(width - stringCellWidth(reportIndent))
			if cols == nil {
				// Canonical fallback: byte-identical to the grouped
				// rendering, no table headers. Canonical output is unconstrained
				// prose (it wraps naturally in real terminals), so the width cap
				// applies to tables only.
				if want := renderGrouped(repos, width); out != want {
					t.Errorf("fallback at width %d is not the canonical grouped report", width)
				}
				if strings.Contains(out, "SUBJECT") || strings.Contains(out, "UPSTREAM") {
					t.Errorf("fallback leaked table headers: %q", out)
				}
				return
			}

			if got := maxLineLength(out); got > width {
				t.Errorf("line length %d exceeds terminal width %d", got, width)
			}

			// Table tiers: the decorative rule spans exactly the terminal.
			ruleFound := false
			for _, line := range strings.Split(out, "\n") {
				if len(line) > 4 && strings.Trim(line, " -") == "" {
					ruleFound = true
					if lineWidth(line) != width {
						t.Errorf("rule line %d != terminal width %d", lineWidth(line), width)
					}
				}
			}
			if !ruleFound {
				t.Errorf("no rule line in table output: %q", out)
			}

			// NAME is never dropped; STATUS must never appear (structural);
			// AHEAD/BEHIND are atomic.
			if !hasHeader(cols, "NAME") {
				t.Errorf("NAME dropped at width %d: %+v", width, cols)
			}
			if hasHeader(cols, "STATUS") {
				t.Errorf("STATUS column present at width %d: %+v", width, cols)
			}
			if hasHeader(cols, "AHEAD") != hasHeader(cols, "BEHIND") {
				t.Errorf("sync pair split at width %d: %+v", width, cols)
			}

			// Degradation ladder (measured boundaries, indent included).
			switch {
			case width >= 95:
				if !hasHeader(cols, "SUBJECT") || !hasHeader(cols, "TIME") {
					t.Errorf("width %d: expected wide eight-column set, got %+v", width, cols)
				}
			case width >= 62:
				if !hasHeader(cols, "SUBJECT") || !hasHeader(cols, "TIME") {
					t.Errorf("width %d: expected full eight-column tight set, got %+v", width, cols)
				}
			case width >= 50:
				if hasHeader(cols, "TIME") || !hasHeader(cols, "SUBJECT") {
					t.Errorf("width %d: expected SUBJECT kept and TIME dropped, got %+v", width, cols)
				}
				for _, h := range []string{"NAME", "BRANCH", "UPSTREAM", "AHEAD", "BEHIND", "HASH", "SUBJECT"} {
					if !hasHeader(cols, h) {
						t.Errorf("width %d: shallow table lost %s: %+v", width, h, cols)
					}
				}
			}

			// Determinism: same state + same width -> identical bytes.
			if again := renderRich(repos, len(repos), width); again != out {
				t.Errorf("non-deterministic rendering at width %d", width)
			}
		})
	}
}
