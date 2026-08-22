package keen

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func browseSample() []Repository {
	return []Repository{
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
}

func TestRenderBrowseHeader(t *testing.T) {
	overview := renderBrowse(browseSample(), 3, BrowseOverview, 40)
	if !strings.Contains(overview, "‹ OVERVIEW ›") {
		t.Errorf("overview header missing: %q", overview)
	}
	if !strings.Contains(overview, "1 / 2") {
		t.Errorf("overview indicator missing: %q", overview)
	}
	if !strings.Contains(overview, "←/→ views") {
		t.Errorf("navigation hint missing: %q", overview)
	}

	activity := renderBrowse(browseSample(), 3, BrowseActivity, 40)
	if !strings.Contains(activity, "‹ ACTIVITY ›") {
		t.Errorf("activity header missing: %q", activity)
	}
	if !strings.Contains(activity, "2 / 2") {
		t.Errorf("activity indicator missing: %q", activity)
	}
}

func TestRenderOverviewIncludesUpstream(t *testing.T) {
	out := renderBrowse(browseSample(), 3, BrowseOverview, 80)

	if !strings.Contains(out, "CLEAN") || !strings.Contains(out, "DIRTY") {
		t.Errorf("expected CLEAN and DIRTY sections: %q", out)
	}
	if !strings.Contains(out, "[clean]") || !strings.Contains(out, "[dirty]") {
		t.Errorf("expected [clean]/[dirty] tags: %q", out)
	}
	// Overview contract: status, name, branch, upstream, ahead, behind.
	if !strings.Contains(out, "v0.8.x") || !strings.Contains(out, "main") {
		t.Errorf("expected branch tokens: %q", out)
	}
	if !strings.Contains(out, "origin/v0.8.x") || !strings.Contains(out, "origin/main") {
		t.Errorf("expected upstream to appear in Overview: %q", out)
	}
	if !strings.Contains(out, "detached") {
		t.Errorf("expected detached branch label: %q", out)
	}
	if !strings.Contains(out, "↑2 ↓1") {
		t.Errorf("expected numeric divergence for upstream repo: %q", out)
	}
	if !strings.Contains(out, "↑– ↓–") {
		t.Errorf("expected non-numeric marker for no-upstream repo: %q", out)
	}
}

func TestRenderActivityShowsCommitIdentity(t *testing.T) {
	out := renderBrowse(browseSample(), 3, BrowseActivity, 200)

	if !strings.Contains(out, "8f31ac2") {
		t.Errorf("expected short hash: %q", out)
	}
	// The interactive Activity view renders the full subject; the horizontal
	// viewport handles width rather than truncating.
	if !strings.Contains(out, "tighten memory format") {
		t.Errorf("expected full commit subject: %q", out)
	}
	if strings.Contains(out, "…") {
		t.Errorf("interactive Activity must not truncate the subject: %q", out)
	}
	if !strings.Contains(out, "2 days ago") {
		t.Errorf("expected relative time: %q", out)
	}
	if !strings.Contains(out, "wip refactor") {
		t.Errorf("expected dirty repo commit: %q", out)
	}
	if !strings.Contains(out, "No commits") {
		t.Errorf("expected 'No commits' for repository with no history: %q", out)
	}
}

func TestRenderBrowseEmptyStates(t *testing.T) {
	found := renderBrowse(nil, 0, BrowseOverview, 40)
	if !strings.Contains(found, "No repositories found.") {
		t.Errorf("expected 'No repositories found.': %q", found)
	}

	filtered := renderBrowse(nil, 5, BrowseOverview, 40)
	if !strings.Contains(filtered, "No repositories match the selected filters.") {
		t.Errorf("expected filtered empty message: %q", filtered)
	}
}

func TestCyclePageWrapsAround(t *testing.T) {
	cases := []struct {
		from BrowsePage
		dir  int
		want BrowsePage
	}{
		{BrowseOverview, -1, BrowseActivity},
		{BrowseActivity, 1, BrowseOverview},
		{BrowseOverview, 1, BrowseActivity},
		{BrowseActivity, -1, BrowseOverview},
	}
	for _, c := range cases {
		if got := cyclePage(c.from, c.dir); got != c.want {
			t.Errorf("cyclePage(%v, %d) = %v, want %v", c.from, c.dir, got, c.want)
		}
	}
}

func TestInterpretSequence(t *testing.T) {
	cases := map[string]keyAction{
		"h":         keyLeft,
		"l":         keyRight,
		"H":         keyScrollLeft,
		"L":         keyScrollRight,
		"q":         keyQuit,
		"\x03":      keyQuit, // Ctrl+C
		"\x1b":      keyEsc,
		"\x1b[D":    keyLeft,
		"\x1b[C":    keyRight,
		"\x1b[1;2D": keyScrollLeft,
		"\x1b[1;5D": keyScrollLeft,
		"\x1b[1;2C": keyScrollRight,
		"\x1b[1;5C": keyScrollRight,
		"\x1b[A":    keyNone,
		"\x1b[B":    keyNone,
		"x":         keyNone,
	}
	for in, want := range cases {
		if got := interpretSequence(in); got != want {
			t.Errorf("interpretSequence(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestSliceViewport(t *testing.T) {
	s := "abcdefghij"

	if got := sliceViewport(s, 0, 4); got != "abcd" {
		t.Errorf("slice(0,4) = %q, want abcd", got)
	}
	if got := sliceViewport(s, 3, 4); got != "defg" {
		t.Errorf("slice(3,4) = %q, want defg", got)
	}
	// Offset past the end clamps to empty.
	if got := sliceViewport(s, 100, 4); got != "" {
		t.Errorf("slice(100,4) = %q, want empty", got)
	}
	// End past the end clamps to the string length.
	if got := sliceViewport(s, 7, 10); got != "hij" {
		t.Errorf("slice(7,10) = %q, want hij", got)
	}
	// Negative offset clamps to zero.
	if got := sliceViewport(s, -5, 4); got != "abcd" {
		t.Errorf("slice(-5,4) = %q, want abcd", got)
	}
}

func TestScrollClampsOffset(t *testing.T) {
	state := &browserState{
		repos:    browseSample(),
		total:    3,
		page:     BrowseActivity,
		offset:   0,
		viewport: 20,
	}
	// Scrolling far right must not exceed contentWidth - viewport.
	state.scroll(10000)
	if state.offset < 0 {
		t.Errorf("offset went negative: %d", state.offset)
	}
	if cw := state.contentWidth(); state.offset > cw-state.viewport {
		t.Errorf("offset %d exceeds max %d", state.offset, cw-state.viewport)
	}
	// Scrolling far left must not go below zero.
	state.scroll(-10000)
	if state.offset != 0 {
		t.Errorf("offset should clamp to 0, got %d", state.offset)
	}
}

func TestTruncateIsRuneAware(t *testing.T) {
	if got := truncate("abcdef", 4); got != "abc…" {
		t.Errorf("truncate ascii = %q, want abc…", got)
	}
	wide := truncate("日本語日本語", 3)
	if got := []rune(wide); len(got) != 3 || got[2] != '…' {
		t.Errorf("truncate wide = %q (%v), want 2 chars + ellipsis", wide, []rune(wide))
	}
}

// TestBrowseHeaderLineContract pins the layout contract between renderBrowse
// and browseHeaderLine so the constant cannot silently drift out of sync with
// the emitted screen.
func TestBrowseHeaderLineContract(t *testing.T) {
	lines := strings.Split(renderBrowse(browseSample(), 3, BrowseOverview, 60), "\n")
	if browseHeaderLine < 0 || browseHeaderLine >= len(lines) {
		t.Fatalf("browseHeaderLine %d out of range for %d lines", browseHeaderLine, len(lines))
	}
	header := lines[browseHeaderLine]
	if !strings.Contains(header, "‹ OVERVIEW ›") || !strings.Contains(header, "1 / 2") {
		t.Errorf("line %d is not the view-indicator header: %q", browseHeaderLine, header)
	}
}

// TestRenderKeepsViewIdentityVisibleAtEveryOffset is the regression test for
// the audit's P1-1: the viewport must scroll body content, never navigation
// identity. It exercises browserState.render() (the composition that was
// buggy), not just renderBrowse.
func TestRenderKeepsViewIdentityVisibleAtEveryOffset(t *testing.T) {
	for _, page := range []BrowsePage{BrowseOverview, BrowseActivity} {
		t.Run(page.label(), func(t *testing.T) {
			state := &browserState{repos: browseSample(), total: 3, page: page, viewport: 20}
			maxOffset := state.contentWidth() - state.viewport
			if maxOffset < 0 {
				maxOffset = 0
			}
			offsets := []int{0}
			if maxOffset > 0 {
				offsets = append(offsets, maxOffset/2, maxOffset)
			}

			title := "‹ " + page.label() + " ›"
			counter := fmt.Sprintf("%d / %d", int(page)+1, browsePageCount)
			for _, offset := range offsets {
				state.offset = offset
				out := captureOutput(state.render)
				if !strings.Contains(out, title) {
					t.Errorf("offset %d: view title %q scrolled away:\n%q", offset, title, out)
				}
				if !strings.Contains(out, counter) {
					t.Errorf("offset %d: page counter %q scrolled away", offset, counter)
				}
			}
		})
	}
}

// TestRunExitsWhenInputStreamEnds is the regression test for the audit's
// P1-2: a dead input stream must terminate the loop after a single failed
// read instead of busy-looping on keyNone forever.
func TestRunExitsWhenInputStreamEnds(t *testing.T) {
	state := &browserState{repos: browseSample(), total: 3, page: BrowseOverview, viewport: 80}
	calls := 0
	captureOutput(func() {
		state.run(func() (keyAction, bool) {
			calls++
			if calls > 64 {
				// Fail fast instead of hanging the suite if a regression
				// makes the loop ignore stream death.
				t.Error("interactive loop ignored a closed input stream")
				return keyQuit, true
			}
			return keyNone, false
		})
	})
	if calls != 1 {
		t.Errorf("loop performed %d reads on a closed stream, want exactly 1", calls)
	}
}

func TestRunQuitsCyclesAndIgnoresUnknownInput(t *testing.T) {
	state := &browserState{repos: browseSample(), total: 3, page: BrowseOverview, viewport: 80}
	script := []struct {
		action keyAction
		ok     bool
	}{
		{keyRight, true},       // Overview -> Activity
		{keyLeft, true},        // Activity -> Overview (wraparound)
		{keyScrollRight, true}, // scroll within bounds
		{keyNone, true},        // unknown/vertical keys are ignored, loop continues
		{keyQuit, true},        // exit here
		{keyQuit, true},        // must never be consumed
	}
	i := 0
	captureOutput(func() {
		state.run(func() (keyAction, bool) {
			if i >= len(script) {
				t.Error("run continued past the quit event")
				return keyNone, false
			}
			s := script[i]
			i++
			return s.action, s.ok
		})
	})
	if i != len(script)-1 {
		t.Errorf("consumed %d events, want %d (stop at quit)", i, len(script)-1)
	}
	if state.page != BrowseOverview {
		t.Errorf("page = %v, want Overview after right/left wraparound", state.page)
	}
}

func TestRunExitsOnEscape(t *testing.T) {
	state := &browserState{repos: browseSample(), total: 3, page: BrowseActivity, viewport: 80}
	calls := 0
	captureOutput(func() {
		state.run(func() (keyAction, bool) {
			calls++
			return keyEsc, true
		})
	})
	if calls != 1 {
		t.Errorf("Esc should exit immediately; reads = %d", calls)
	}
	if state.page != BrowseActivity {
		t.Errorf("page changed without a navigation event: %v", state.page)
	}
}

// TestReadKeySignalsClosedInput drives the platform readKey against a closed
// stdin and requires an explicit stream-dead signal rather than keyNone.
func TestReadKeySignalsClosedInput(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	if _, ok := readKey(); ok {
		t.Error("readKey must report a closed stream instead of returning a live key event")
	}
}

func TestReadKeyParsesLiveInput(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	if _, err := w.WriteString("q"); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()

	action, ok := readKey()
	if !ok {
		t.Fatal("live single byte must not be reported as a closed stream")
	}
	if action != keyQuit {
		t.Errorf("action = %v, want keyQuit", action)
	}
}
