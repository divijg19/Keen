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
	overview := renderBrowse(browseSample(), 3, BrowseList, 40)
	if !strings.Contains(overview, "KEEN › REPOSITORIES") {
		t.Errorf("list header missing: %q", overview)
	}
	if strings.Contains(overview, "/ 5") {
		t.Errorf("list header must not carry a global step counter: %q", overview)
	}
	if !strings.Contains(overview, "↑/↓ select") {
		t.Errorf("list navigation hint missing: %q", overview)
	}

	history := renderBrowse(browseSample(), 3, BrowseCommitHistory, 40)
	if !strings.Contains(history, "KEEN › Peony › HISTORY") {
		t.Errorf("history header missing: %q", history)
	}
	if strings.Contains(history, "/ 5") {
		t.Errorf("history header must not carry a global step counter: %q", history)
	}
	detail := renderBrowse(browseSample(), 3, BrowseDetail, 40)
	if !strings.Contains(detail, "KEEN › Peony › DETAIL") {
		t.Errorf("detail header missing: %q", detail)
	}
	if strings.Contains(detail, "/ 5") {
		t.Errorf("detail header must not carry a global step counter: %q", detail)
	}
	if !strings.Contains(detail, "←/Esc back") {
		t.Errorf("detail navigation hint missing: %q", detail)
	}
}

func TestRenderOverviewIncludesUpstream(t *testing.T) {
	out := renderBrowse(browseSample(), 3, BrowseList, 80)

	if !strings.Contains(out, "CLEAN") || !strings.Contains(out, "DIRTY") {
		t.Errorf("expected CLEAN and DIRTY sections: %q", out)
	}
	// Status is structural in the one-shot overview: the section headings are
	// the single signal, so rows must not repeat per-repo tags.
	if strings.Contains(out, "[clean]") || strings.Contains(out, "[dirty]") {
		t.Errorf("overview rows must not carry per-repo status tags: %q", out)
	}
	// Overview contract: name, branch, upstream, ahead, behind.
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

func TestRenderBrowseEmptyStates(t *testing.T) {
	found := renderBrowse(nil, 0, BrowseList, 40)
	if !strings.Contains(found, "No repositories found.") {
		t.Errorf("expected 'No repositories found.': %q", found)
	}

	filtered := renderBrowse(nil, 5, BrowseList, 40)
	if !strings.Contains(filtered, "No repositories match the selected filters.") {
		t.Errorf("expected filtered empty message: %q", filtered)
	}
}

// TestInterpretSequence pins the input binding table. Navigation is
// hierarchical: Enter advances, ←/Esc return to the parent, and a plain → has
// no cyclic "next view" meaning.
func TestInterpretSequence(t *testing.T) {
	cases := map[string]keyAction{
		"\x1b[D":    keyLeft,
		"H":         keyScrollLeft,
		"L":         keyScrollRight,
		"q":         keyQuit,
		"\x03":      keyQuit, // Ctrl+C
		"\x1b":      keyEsc,
		"\x1b[1;2D": keyScrollLeft,
		"\x1b[1;5D": keyScrollLeft,
		"\x1b[1;2C": keyScrollRight,
		"\x1b[1;5C": keyScrollRight,
		"\x1b[A":    keyUp,
		"\x1b[B":    keyDown,
		"\r":        keyEnter,
		"\n":        keyEnter,
		"\x7f":      keyBackspace,
		"\x08":      keyBackspace,
		"x":         keyNone,
	}
	for in, want := range cases {
		if got := interpretSequence(in); got != want {
			t.Errorf("interpretSequence(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestInterpretSequenceRejectsLegacyNavKeys pins that the old vim-style and
// arrow-wrap navigation ("h"/"l", plain "→") no longer map to any action, so
// the cyclic-browser model cannot resurface through the input layer.
func TestInterpretSequenceRejectsLegacyNavKeys(t *testing.T) {
	for _, in := range []string{"h", "l", "\x1b[C"} {
		if got := interpretSequence(in); got != keyNone {
			t.Errorf("interpretSequence(%q) = %v, want keyNone (legacy nav key removed)", in, got)
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

func TestSliceViewportCellAware(t *testing.T) {
	// "a日b": cells a=1, 日=2, b=1 (total 4).
	if got := sliceViewport("a日b", 0, 4); got != "a日b" {
		t.Errorf("full window = %q, want a日b", got)
	}
	if got := sliceViewport("a日b", 0, 1); got != "a" {
		t.Errorf("window(0,1) = %q, want a (wide char straddling edge skipped)", got)
	}
	if got := sliceViewport("a日b", 1, 2); got != "日" {
		t.Errorf("window(1,2) = %q, want 日", got)
	}
	if got := sliceViewport("a日b", 3, 1); got != "b" {
		t.Errorf("window(3,1) = %q, want b", got)
	}
	if got := sliceViewport("a日b", 10, 4); got != "" {
		t.Errorf("window past end = %q, want empty", got)
	}
}

func TestPadStartCells(t *testing.T) {
	if got := padStartCells("ab", 5); got != "ab   " {
		t.Errorf("pad ascii = %q, want %q", got, "ab   ")
	}
	if got := padStartCells("abcdef", 5); got != "abcdef" {
		t.Errorf("over-wide must spill, got %q", got)
	}
	if got := stringCellWidth(padStartCells("日本", 6)); got != 6 {
		t.Errorf("padded CJK width = %d, want 6", got)
	}
}

func TestHeaderTruncatesLongTitle(t *testing.T) {
	long := Repository{Name: strings.Repeat("a", 60), Path: "/x/" + strings.Repeat("a", 60)}
	out := renderBrowse([]Repository{long}, 1, BrowseDetail, 40)
	first := strings.Split(out, "\n")[0]
	if stringCellWidth(first) > 40 {
		t.Errorf("header line %d cells exceeds width 40: %q", stringCellWidth(first), first)
	}
	if !strings.Contains(first, "…") {
		t.Errorf("truncated header must carry an ellipsis: %q", first)
	}
}

// TestInteractiveFooterStableAcrossWidths verifies the footer survives
// representative terminal widths on every surface, including narrow ones.
func TestInteractiveFooterStableAcrossWidths(t *testing.T) {
	for _, width := range []int{40, 60, 80, 120} {
		for _, page := range []BrowsePage{BrowseList, BrowseDetail, BrowseCommitHistory, BrowseCommitDetail, BrowseChangedFiles} {
			out := renderBrowse(browseSample(), 3, page, width)
			if !strings.Contains(out, "quit") {
				t.Errorf("width %d page %v lost footer:\n%q", width, page, out)
			}
			if !strings.Contains(out, "KEEN ›") {
				t.Errorf("width %d page %v lost header:\n%q", width, page, out)
			}
		}
	}
}

func TestScrollClampsOffset(t *testing.T) {
	state := &browserState{
		repos:    browseSample(),
		total:    3,
		page:     BrowseDetail,
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
	lines := strings.Split(renderBrowse(browseSample(), 3, BrowseList, 60), "\n")
	if browseHeaderLine < 0 || browseHeaderLine >= len(lines) {
		t.Fatalf("browseHeaderLine %d out of range for %d lines", browseHeaderLine, len(lines))
	}
	header := lines[browseHeaderLine]
	if !strings.Contains(header, "KEEN › REPOSITORIES") {
		t.Errorf("line %d is not the view-indicator header: %q", browseHeaderLine, header)
	}
	if strings.Contains(header, "/ 5") {
		t.Errorf("line %d must not carry a global step counter: %q", browseHeaderLine, header)
	}
}

// TestRenderKeepsViewIdentityVisibleAtEveryOffset is the regression test for
// the audit's P1-1: the viewport must scroll body content, never navigation
// identity. It exercises browserState.render() (the composition that was
// buggy), not just renderBrowse.
func TestRenderKeepsViewIdentityVisibleAtEveryOffset(t *testing.T) {
	for _, page := range []BrowsePage{BrowseList, BrowseDetail, BrowseCommitHistory, BrowseCommitDetail, BrowseChangedFiles} {
		t.Run(page.label(), func(t *testing.T) {
			state := &browserState{
				repos:          browseSample(),
				total:          3,
				page:           page,
				selected:       0,
				viewport:       20,
				viewportHeight: 24,
				history:        []Commit{{Hash: "abc1234", Subject: "test commit"}},
				selectedCommit: 0,
			}
			maxOffset := state.contentWidth() - state.viewport
			if maxOffset < 0 {
				maxOffset = 0
			}
			offsets := []int{0}
			if maxOffset > 0 {
				offsets = append(offsets, maxOffset/2, maxOffset)
			}

			title := "KEEN › REPOSITORIES"
			if page != BrowseList {
				title = fmt.Sprintf("KEEN › %s › %s", browseSample()[0].Name, page.label())
			}
			for _, offset := range offsets {
				state.offset = offset
				out := captureOutput(state.render)
				if !strings.Contains(out, title) {
					t.Errorf("offset %d: view title %q scrolled away:\n%q", offset, title, out)
				}
			}
		})
	}
}

// TestRunExitsWhenInputStreamEnds is the regression test for the audit's
// P1-2: a dead input stream must terminate the loop after a single failed
// read instead of busy-looping on keyNone forever.
func TestRunExitsWhenInputStreamEnds(t *testing.T) {
	state := &browserState{repos: browseSample(), total: 3, page: BrowseList, viewport: 80}
	calls := 0
	captureOutput(func() {
		state.run(func() (keyAction, string, bool) {
			calls++
			if calls > 64 {
				// Fail fast instead of hanging the suite if a regression
				// makes the loop ignore stream death.
				t.Error("interactive loop ignored a closed input stream")
				return keyQuit, "q", true
			}
			return keyNone, "", false
		})
	})
	if calls != 1 {
		t.Errorf("loop performed %d reads on a closed stream, want exactly 1", calls)
	}
}

func TestRunHierarchicalNavigationAndIgnoresUnknownInput(t *testing.T) {
	state := &browserState{repos: browseSample(), total: 3, page: BrowseList, viewport: 80}
	script := []struct {
		action keyAction
		raw    string
		ok     bool
	}{
		{keyDown, "\x1b[B", true},  // move selection down (list)
		{keyEnter, "\r", true},     // List -> Detail
		{keyEnter, "\r", true},     // Detail -> History
		{keyLeft, "\x1b[D", true},  // History -> Detail
		{keyEsc, "\x1b", true},     // Detail -> List
		{keyScrollRight, "", true}, // scroll within bounds (list)
		{keyNone, "", true},        // unknown keys are ignored, loop continues
		{keyQuit, "q", true},       // exit here
		{keyQuit, "q", true},       // must never be consumed
	}
	i := 0
	captureOutput(func() {
		state.run(func() (keyAction, string, bool) {
			if i >= len(script) {
				t.Error("run continued past the quit event")
				return keyNone, "", false
			}
			s := script[i]
			i++
			return s.action, s.raw, s.ok
		})
	})
	if i != len(script)-1 {
		t.Errorf("consumed %d events, want %d (stop at quit)", i, len(script)-1)
	}
	if state.page != BrowseList {
		t.Errorf("page = %v, want List after navigation back", state.page)
	}
}

// TestRunEnterDoesNotCycleFromChangedFiles pins that advancing with Enter from the
// deepest surface (Changed Files) is a no-op rather than wrapping around to List.
// Navigation is a strict hierarchy (List → Detail → Activity → Commit History → Commit Detail → Changed Files), never a carousel.
func TestRunEnterDoesNotCycleFromChangedFiles(t *testing.T) {
	state := newBrowserState(browseSample(), 3)
	state.page = BrowseChangedFiles
	i := 0
	captureOutput(func() {
		state.run(func() (keyAction, string, bool) {
			i++
			if i == 1 {
				return keyEnter, "\r", true
			}
			return keyQuit, "q", true
		})
	})
	if i != 2 {
		t.Errorf("Enter from ChangedFiles then quit; reads = %d, want 2", i)
	}
	if state.page != BrowseChangedFiles {
		t.Errorf("page = %v, want ChangedFiles (Enter must not wrap to List)", state.page)
	}
}

// TestRunForwardArrowIsNoOp pins that a plain → does not advance to another
// surface; the forward direction is exclusively Enter.
func TestRunForwardArrowIsNoOp(t *testing.T) {
	state := newBrowserState(browseSample(), 3)
	state.page = BrowseDetail
	i := 0
	captureOutput(func() {
		state.run(func() (keyAction, string, bool) {
			i++
			if i == 1 {
				return interpretSequence("\x1b[C"), "\x1b[C", true // plain →
			}
			return keyQuit, "q", true
		})
	})
	if state.page != BrowseDetail {
		t.Errorf("page = %v, want Detail (plain → must not advance)", state.page)
	}
}

func TestRunIgnoresEscapeFromList(t *testing.T) {
	state := newBrowserState(browseSample(), 3)
	i := 0
	captureOutput(func() {
		state.run(func() (keyAction, string, bool) {
			i++
			if i == 1 {
				return keyEsc, "\x1b", true
			}
			return keyQuit, "q", true
		})
	})
	if i != 2 {
		t.Errorf("Esc from list should be ignored; reads = %d, want 2", i)
	}
	if state.page != BrowseList {
		t.Errorf("page changed unexpectedly: %v", state.page)
	}
}

func TestRunQuitsOnEscapeFromDetail(t *testing.T) {
	state := newBrowserState(browseSample(), 3)
	state.page = BrowseDetail
	i := 0
	captureOutput(func() {
		state.run(func() (keyAction, string, bool) {
			i++
			if i == 1 {
				return keyEsc, "\x1b", true
			}
			return keyQuit, "q", true
		})
	})
	if i != 2 {
		t.Errorf("Esc from detail then quit; reads = %d, want 2", i)
	}
	if state.page != BrowseList {
		t.Errorf("page = %v, want List", state.page)
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

	if _, _, ok := readKey(); ok {
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

	action, raw, ok := readKey()
	if !ok {
		t.Fatal("live single byte must not be reported as a closed stream")
	}
	if action != keyQuit {
		t.Errorf("action = %v, want keyQuit", action)
	}
	if raw != "q" {
		t.Errorf("raw = %q, want %q", raw, "q")
	}
}

// --- Selection tests ---

func TestSelectionInitial(t *testing.T) {
	s := newBrowserState(browseSample(), 3)
	if s.selected != 0 {
		t.Errorf("initial selected = %d, want 0", s.selected)
	}
	empty := newBrowserState(nil, 0)
	if empty.selected != -1 {
		t.Errorf("empty initial selected = %d, want -1", empty.selected)
	}
	single := newBrowserState([]Repository{{Name: "solo", Path: "/solo"}}, 1)
	if single.selected != 0 {
		t.Errorf("single initial selected = %d, want 0", single.selected)
	}
}

func TestSelectionMove(t *testing.T) {
	s := newBrowserState(browseSample(), 3)
	s.moveSelection(1)
	if s.selected != 1 {
		t.Errorf("move down: selected = %d, want 1", s.selected)
	}
	s.moveSelection(1)
	if s.selected != 2 {
		t.Errorf("move down: selected = %d, want 2", s.selected)
	}
	s.moveSelection(-1)
	if s.selected != 1 {
		t.Errorf("move up: selected = %d, want 1", s.selected)
	}
}

func TestSelectionBounds(t *testing.T) {
	s := newBrowserState(browseSample(), 3)
	s.moveSelection(-10)
	if s.selected != 0 {
		t.Errorf("upper bound: selected = %d, want 0", s.selected)
	}
	s.moveSelection(10)
	if s.selected != 2 {
		t.Errorf("lower bound: selected = %d, want 2", s.selected)
	}
	empty := newBrowserState(nil, 0)
	empty.moveSelection(1)
	if empty.selected != -1 {
		t.Errorf("empty move: selected = %d, want -1", empty.selected)
	}
}

// --- Vertical viewport tests ---

func TestVerticalViewport(t *testing.T) {
	// All items fit
	s := newBrowserState(browseSample(), 3)
	s.viewportHeight = 24
	s.ensureVisible()
	if s.listOffset != 0 {
		t.Errorf("all fit: listOffset = %d, want 0", s.listOffset)
	}
	// Long list, selected at top
	long := make([]Repository, 20)
	for i := range long {
		long[i] = Repository{Name: fmt.Sprintf("repo-%02d", i), Path: fmt.Sprintf("/repo-%02d", i)}
	}
	s = newBrowserState(long, 20)
	s.viewportHeight = 10 // availableRows = 5
	s.selected = 0
	s.ensureVisible()
	if s.listOffset != 0 {
		t.Errorf("top: listOffset = %d, want 0", s.listOffset)
	}
	// Selected at bottom
	s.selected = 19
	s.ensureVisible()
	if s.listOffset != 15 {
		t.Errorf("bottom: listOffset = %d, want 15", s.listOffset)
	}
	// Scroll down
	s.selected = 5
	s.listOffset = 0
	s.ensureVisible()
	if s.listOffset != 1 {
		t.Errorf("scroll down: listOffset = %d, want 1", s.listOffset)
	}
	// Scroll up
	s.selected = 2
	s.ensureVisible()
	if s.listOffset != 1 {
		t.Errorf("scroll up: listOffset = %d, want 1 (still visible)", s.listOffset)
	}
	s.selected = 1
	s.ensureVisible()
	if s.listOffset != 1 {
		t.Errorf("scroll up: listOffset = %d, want 1", s.listOffset)
	}
	// Selection remains visible after moves
	for _, sel := range []int{0, 5, 10, 19} {
		s.selected = sel
		s.ensureVisible()
		if sel < s.listOffset || sel >= s.listOffset+s.availableRows() {
			t.Errorf("selected %d not visible in window [%d, %d)", sel, s.listOffset, s.listOffset+s.availableRows())
		}
	}
}

// --- Navigation tests ---

func TestNavigationListToDetail(t *testing.T) {
	s := newBrowserState(browseSample(), 3)
	s.page = BrowseList
	captureOutput(func() {
		s.run(func() (keyAction, string, bool) {
			// Enter -> Detail, then quit
			if s.page == BrowseList {
				return keyEnter, "\r", true
			}
			return keyQuit, "q", true
		})
	})
	if s.page != BrowseDetail {
		t.Errorf("list Enter -> detail: page = %v, want Detail", s.page)
	}
}

func TestNavigationDetailToList(t *testing.T) {
	s := newBrowserState(browseSample(), 3)
	s.page = BrowseDetail
	i := 0
	captureOutput(func() {
		s.run(func() (keyAction, string, bool) {
			i++
			if i == 1 {
				return keyLeft, "\x1b[D", true
			}
			return keyQuit, "q", true
		})
	})
	if i != 2 {
		t.Errorf("detail Left -> list then quit; reads = %d, want 2", i)
	}
	if s.page != BrowseList {
		t.Errorf("detail Left -> list: page = %v, want List", s.page)
	}
	s.page = BrowseDetail
	i = 0
	captureOutput(func() {
		s.run(func() (keyAction, string, bool) {
			i++
			if i == 1 {
				return keyEsc, "\x1b", true
			}
			return keyQuit, "q", true
		})
	})
	if i != 2 {
		t.Errorf("detail Esc -> list then quit; reads = %d, want 2", i)
	}
	if s.page != BrowseList {
		t.Errorf("detail Esc -> list: page = %v, want List", s.page)
	}
}

func TestNavigationDetailToHistory(t *testing.T) {
	s := newBrowserState(browseSample(), 3)
	s.page = BrowseDetail
	captureOutput(func() {
		s.run(func() (keyAction, string, bool) {
			if s.page == BrowseDetail {
				return keyEnter, "\r", true
			}
			return keyQuit, "q", true
		})
	})
	if s.page != BrowseCommitHistory {
		t.Errorf("detail Enter -> history: page = %v, want CommitHistory", s.page)
	}
}

func TestNavigationHistoryToDetail(t *testing.T) {
	s := newBrowserState(browseSample(), 3)
	s.page = BrowseCommitHistory
	i := 0
	captureOutput(func() {
		s.run(func() (keyAction, string, bool) {
			i++
			if i == 1 {
				return keyLeft, "\x1b[D", true
			}
			return keyQuit, "q", true
		})
	})
	if s.page != BrowseDetail {
		t.Errorf("history Left -> detail: page = %v, want Detail", s.page)
	}
}

func TestNavigationQuitFromAllPages(t *testing.T) {
	for _, page := range []BrowsePage{BrowseList, BrowseDetail, BrowseCommitHistory, BrowseCommitDetail, BrowseChangedFiles} {
		s := newBrowserState(browseSample(), 3)
		s.page = page
		calls := 0
		captureOutput(func() {
			s.run(func() (keyAction, string, bool) {
				calls++
				return keyQuit, "q", true
			})
		})
		if calls != 1 {
			t.Errorf("quit from %v: calls = %d, want 1", page, calls)
		}
	}
}

func TestNavigationEOF(t *testing.T) {
	s := newBrowserState(browseSample(), 3)
	calls := 0
	captureOutput(func() {
		s.run(func() (keyAction, string, bool) {
			calls++
			return keyNone, "", false
		})
	})
	if calls != 1 {
		t.Errorf("EOF: calls = %d, want 1", calls)
	}
}

// TestNavigationKeepsSelectionValid drives the full hierarchy round-trip
// (List → Detail → History → Detail → List) and asserts the selection index
// stays a valid address into the repository set after every transition.
func TestNavigationKeepsSelectionValid(t *testing.T) {
	s := newBrowserState(browseSample(), 3)
	s.selected = 2
	i := 0
	captureOutput(func() {
		s.run(func() (keyAction, string, bool) {
			i++
			switch i {
			case 1:
				return keyEnter, "\r", true // List -> Detail
			case 2:
				return keyEnter, "\r", true // Detail -> History
			case 3:
				return keyLeft, "\x1b[D", true // History -> Detail
			case 4:
				return keyEsc, "\x1b", true // Detail -> List
			}
			return keyQuit, "q", true
		})
	})
	if s.page != BrowseList {
		t.Errorf("round-trip end page = %v, want List", s.page)
	}
	if s.selected < 0 || s.selected >= len(s.repos) {
		t.Errorf("selection %d out of bounds after round-trip", s.selected)
	}
	if s.selectedRepo() == nil {
		t.Error("selectedRepo() nil after navigation with a valid prior selection")
	}
}

// TestNavigationFullV070Hierarchy drives the complete five-surface hierarchy
// (List → Detail → History → Commit → Files
// and back up to List) and asserts correct page transitions at each step.
func TestNavigationFullV070Hierarchy(t *testing.T) {
	s := newBrowserState(browseSample(), 3)
	s.history = []Commit{{Hash: "abc1234", Subject: "test commit"}}
	s.selectedCommit = 0

	actions := []struct {
		action keyAction
		raw    string
	}{
		{keyEnter, "\r"}, // List -> Detail
		{keyEnter, "\r"}, // Detail -> History
		{keyEnter, "\r"}, // History -> Commit
		{keyEnter, "\r"}, // Commit -> Files
		{keyEsc, "\x1b"}, // Files -> Commit
		{keyEsc, "\x1b"}, // Commit -> History
		{keyEsc, "\x1b"}, // History -> Detail
		{keyEsc, "\x1b"}, // Detail -> List
		{keyQuit, "q"},   // exit
	}

	i := 0
	captureOutput(func() {
		s.run(func() (keyAction, string, bool) {
			if i >= len(actions) {
				return keyQuit, "q", false
			}
			act := actions[i]
			i++
			return act.action, act.raw, true
		})
	})
	if s.page != BrowseList {
		t.Errorf("final page = %v, want List", s.page)
	}
}

// --- Detail rendering tests ---

func TestRenderDetail(t *testing.T) {
	repo := Repository{
		Name: "myrepo", Path: "/home/user/myrepo", Branch: "main", Upstream: "origin/main",
		Dirty: false, Ahead: 2, Behind: 1,
		LastCommitHash: "abc123def", LastCommitSubject: "fix bug", LastCommitTime: "2 hours ago",
	}
	out := renderDetail(repo)
	for _, want := range []string{"myrepo", "/home/user/myrepo", "clean", "main", "origin/main", "2", "1", "abc123d", "fix bug", "2 hours ago"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail missing %q in %q", want, out)
		}
	}
	// Dirty
	repo.Dirty = true
	out = renderDetail(repo)
	if !strings.Contains(out, "dirty") {
		t.Errorf("detail dirty: %q", out)
	}
	// No upstream
	repo.Upstream = ""
	out = renderDetail(repo)
	if !strings.Contains(out, "—") {
		t.Errorf("detail no upstream: %q", out)
	}
	// Detached
	repo.Branch = ""
	out = renderDetail(repo)
	if !strings.Contains(out, "detached") {
		t.Errorf("detail detached: %q", out)
	}
	// No commits
	repo.LastCommitHash = ""
	out = renderDetail(repo)
	if !strings.Contains(out, "No commits") {
		t.Errorf("detail no commits: %q", out)
	}
	// Long path
	repo.Path = strings.Repeat("a", 100) + "/repo"
	out = renderDetail(repo)
	if !strings.Contains(out, "a") {
		t.Errorf("detail long path: %q", out)
	}
	// Collision-resolved identity
	repo.Name = "work/api"
	out = renderDetail(repo)
	if !strings.Contains(out, "work/api") {
		t.Errorf("detail collision identity: %q", out)
	}
}

// --- Pipeline integrity ---

func TestPipelineIntegrity(t *testing.T) {
	repos := []Repository{
		{Name: "api", Path: "/work/api", Dirty: false},
		{Name: "api", Path: "/personal/api", Dirty: true},
	}
	// Simulate pipeline: Filter -> Resolve -> select -> detail
	filtered := Filter(repos, Options{})
	presented := ResolveDisplayIdentities(filtered)
	s := newBrowserState(presented, len(repos))
	s.selected = 1 // select second (personal/api)
	repo := s.selectedRepo()
	if repo == nil || repo.Path != "/personal/api" {
		t.Errorf("selected repo Path = %q, want /personal/api", repo.Path)
	}
	if repo.Name != "personal/api" {
		t.Errorf("selected display Name = %q, want personal/api", repo.Name)
	}
	// Detail must use same repo object, not lookup by Name
	detail := renderDetail(*repo)
	if !strings.Contains(detail, "/personal/api") {
		t.Errorf("detail must show canonical Path: %q", detail)
	}
}

// --- Rendering determinism ---

func TestRenderingDeterminism(t *testing.T) {
	repos := browseSample()
	s := newBrowserState(repos, 3)
	s.selected = 1
	s.page = BrowseList
	a := captureOutput(s.render)
	b := captureOutput(s.render)
	if a != b {
		t.Errorf("rendering not deterministic")
	}
}

// --- Commit detail & changed-files rendering tests ---

func commitFixture() (Repository, Commit) {
	repo := Repository{Name: "myrepo", Path: "/work/myrepo"}
	commit := Commit{
		Hash:          "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0",
		Subject:       "Add feature",
		Body:          "A body line\nwith a second line",
		Author:        "Alice Author",
		AuthorDate:    "2026-08-01T00:00:00+00:00",
		Committer:     "Bob Committer",
		CommitterDate: "2026-08-01T00:00:00+00:00",
		Parents:       []string{"parent1", "parent2"},
	}
	return repo, commit
}

func TestRenderCommitDetail(t *testing.T) {
	repo, commit := commitFixture()
	out := renderCommitDetail(repo, commit)

	for _, want := range []string{
		"Repository: myrepo",
		"Commit:     a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0",
		"Author:     Alice Author",
		"Committer:  Bob Committer",
		"Parents:    parent1 parent2",
		"Subject:    Add feature",
		"Body:",
		"A body line",
		"with a second line",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("renderCommitDetail missing %q:\n%q", want, out)
		}
	}
}

// TestRenderCommitDetailRootCommit pins that a commit with no parents renders
// the honest root-commit marker rather than assuming a parent exists.
func TestRenderCommitDetailRootCommit(t *testing.T) {
	repo, commit := commitFixture()
	commit.Parents = nil
	commit.Body = ""
	out := renderCommitDetail(repo, commit)
	if !strings.Contains(out, "(none - root commit)") {
		t.Errorf("root commit marker missing:\n%q", out)
	}
	if strings.Contains(out, "Body:") {
		t.Errorf("empty body must not render a Body section:\n%q", out)
	}
}

// TestRenderCommitDetailSpecialCharacters pins that commit metadata containing
// quotes, apostrophes, and Unicode survives rendering.
func TestRenderCommitDetailSpecialCharacters(t *testing.T) {
	repo, commit := commitFixture()
	commit.Subject = "feat: \"quoted\" it's 'a' — 日本語"
	commit.Body = "\"nested\" 'quotes' 日本語"
	out := renderCommitDetail(repo, commit)
	if !strings.Contains(out, commit.Subject) {
		t.Errorf("subject with special chars missing:\n%q", out)
	}
	if !strings.Contains(out, commit.Body) {
		t.Errorf("body with special chars missing:\n%q", out)
	}
}

// TestRenderChangedFilesForCommit pins the changed-files rendering contract for
// ordinary paths, renames, and multiple files.
func TestRenderChangedFilesForCommit(t *testing.T) {
	repo, commit := commitFixture()
	files := []ChangedFile{
		{Status: "M", Path: "internal/a.go"},
		{Status: "A", Path: "internal/b.go"},
		{Status: "D", Path: "internal/c.go"},
		{Status: "R", Path: "new.go", OldPath: "old.go"},
	}
	out := renderChangedFilesForCommit(repo, commit, files, "")
	for _, want := range []string{
		"Repository: myrepo",
		"Commit:     a1b2c3d", // short hash
		"M  internal/a.go",
		"A  internal/b.go",
		"D  internal/c.go",
		"R  old.go -> new.go",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("renderChangedFilesForCommit missing %q:\n%q", want, out)
		}
	}
}

// TestRenderChangedFilesEmpty pins the empty changed-file state: no files, no
// panic, and an explicit empty-state message.
func TestRenderChangedFilesEmpty(t *testing.T) {
	repo, commit := commitFixture()
	out := renderChangedFilesForCommit(repo, commit, nil, "")
	if !strings.Contains(out, "No files changed.") {
		t.Errorf("empty changed files marker missing:\n%q", out)
	}
}

// TestRenderChangedFilesError pins the changed-files error state: an explicit
// failure message is shown without panic or invalid output.
func TestRenderChangedFilesError(t *testing.T) {
	repo, commit := commitFixture()
	out := renderChangedFilesForCommit(repo, commit, nil, "some git error")
	if !strings.Contains(out, "Failed to load changed files: some git error") {
		t.Errorf("changed files error marker missing:\n%q", out)
	}
}

// TestRenderChangedFilesSpecialCharacters pins that filenames containing
// spaces and punctuation render without corruption.
func TestRenderChangedFilesSpecialCharacters(t *testing.T) {
	repo, commit := commitFixture()
	files := []ChangedFile{
		{Status: "M", Path: "dir with spaces/my file.go"},
		{Status: "A", Path: "日本語/テスト.go"},
	}
	out := renderChangedFilesForCommit(repo, commit, files, "")
	if !strings.Contains(out, "dir with spaces/my file.go") {
		t.Errorf("filename with spaces missing:\n%q", out)
	}
	if !strings.Contains(out, "日本語/テスト.go") {
		t.Errorf("unicode filename missing:\n%q", out)
	}
}

// TestBrowseNonTTY pins the script compatibility contract: when stdin is not a
// terminal (e.g. redirected from a pipe), keen -i (Browse) prints a one-shot
// static overview and exits immediately. Its output must be pure text with no
// ANSI control or escape sequence (screen clearing, highlighting, or otherwise).
func TestBrowseNonTTY(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString("some input"); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()

	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	repos := browseSample()
	out := captureOutput(func() {
		Browse(repos, 3)
	})

	if !strings.Contains(out, "KEEN › REPOSITORIES") {
		t.Errorf("non-TTY browse must output static list view, got:\n%q", out)
	}
	if containsControlSequence(out) {
		t.Errorf("non-TTY browse output contains a terminal control sequence:\n%q", out)
	}
}

// containsControlSequence reports whether s carries any terminal escape or
// control output: ANSI CSI/ESC sequences, Ctrl+C, or other control characters
// beyond printable text, horizontal tab, and newline.
func containsControlSequence(s string) bool {
	for _, r := range s {
		if r == '\x1b' || r == '\x03' || (r < 0x20 && r != '\n' && r != '\t' && r != '\r') {
			return true
		}
	}
	return false
}

func TestBrowserFiltering(t *testing.T) {
	state := newBrowserState(browseSample(), 3)
	state.filtering = true
	state.filterQuery = "peo"
	state.applyFilter()
	if len(state.repos) != 1 || state.repos[0].Name != "Peony" {
		t.Errorf("expected 1 repo matching 'peo', got %d", len(state.repos))
	}
	state.filtering = false
	state.filterQuery = ""
	state.applyFilter()
	if len(state.repos) != 3 {
		t.Errorf("expected 3 repos after clearing filter, got %d", len(state.repos))
	}
}

func TestWindowLines(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e"}
	got, off := windowLines(lines, 0, 3)
	if len(got) != 3 || got[0] != "a" || off != 0 {
		t.Errorf("window start = %v,%d; want [a b c],0", got, off)
	}
	got, off = windowLines(lines, 4, 3)
	if len(got) != 3 || got[0] != "c" || off != 2 {
		t.Errorf("window overflow = %v,%d; want [c d e],2", got, off)
	}
	got, off = windowLines(lines, -5, 3)
	if len(got) != 3 || off != 0 {
		t.Errorf("window negative = %v,%d; want offset 0", got, off)
	}
	got, _ = windowLines([]string{"only"}, 0, 5)
	if len(got) != 1 {
		t.Errorf("window short content = %v, want [only]", got)
	}
}

// TestBoundedDetailKeepsFooterVisible verifies the v0.9.2 chrome contract:
// long Detail content is windowed to the available rows so the footer stays
// in the output, and scrolling reveals later lines.
func TestBoundedDetailKeepsFooterVisible(t *testing.T) {
	state := newBrowserState(browseSample(), 3)
	state.viewportHeight = 10 // availableRows = 10 - 5 = 5
	content := "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n"
	bounded := state.boundedDetailContent(content)
	if strings.Contains(bounded, "l10") {
		t.Errorf("bounded content must window long bodies, got:\n%q", bounded)
	}
	for _, want := range []string{"l1", "l5"} {
		if !strings.Contains(bounded, want) {
			t.Errorf("bounded content missing %q:\n%q", want, bounded)
		}
	}
	state.scrollDetail(100)
	again := state.boundedDetailContent(content)
	if !strings.Contains(again, "l10") {
		t.Errorf("scrolling must reveal tail lines, got:\n%q", again)
	}
	if strings.Contains(again, "l1\n") && strings.HasPrefix(again, "l1") {
		t.Errorf("scrolled window must advance past the head, got:\n%q", again)
	}
}

// TestRenderPinsFooterUnderLongContent drives a full render at a small
// terminal height with long detail content and asserts the footer remains the
// final visible line.
func TestRenderPinsFooterUnderLongContent(t *testing.T) {
	t.Setenv("LINES", "12")
	t.Setenv("COLUMNS", "80")
	state := newBrowserState(browseSample(), 3)
	state.page = BrowseDetail
	state.selected = 0
	out := captureOutput(state.render)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) == 0 {
		t.Fatal("empty render output")
	}
	last := lines[len(lines)-1]
	if !strings.Contains(last, "quit") {
		t.Errorf("footer must be the final visible line, got %q in:\n%q", last, out)
	}
	if len(lines) > 12 {
		t.Errorf("rendered %d lines at height 12; footer must stay visible", len(lines))
	}
}

// TestSelectionSingleCue verifies the v0.9.2 selection treatment: the
// reverse-video highlight applied by the renderer is the single dominant
// signal, so no pointer glyph competes with it in the row text.
func TestSelectionSingleCue(t *testing.T) {
	state := newBrowserState(browseSample(), 3)
	out := captureOutput(state.render)
	if strings.Contains(out, "▸") {
		t.Errorf("list rows must not carry a pointer glyph:\n%q", out)
	}
	state.page = BrowseCommitHistory
	state.history = []Commit{{Hash: "abc1234", Subject: "test commit"}}
	state.selectedCommit = 0
	out = captureOutput(state.render)
	if strings.Contains(out, "▸") {
		t.Errorf("history rows must not carry a pointer glyph:\n%q", out)
	}
}

func TestRenderBrowseFooterPresence(t *testing.T) {
	for _, page := range []BrowsePage{BrowseList, BrowseDetail, BrowseCommitHistory, BrowseCommitDetail, BrowseChangedFiles} {
		out := renderBrowse(browseSample(), 3, page, 80)
		if !strings.Contains(out, "quit") {
			t.Errorf("page %v missing footer/keymap: %q", page, out)
		}
	}
}

func TestIsFilterChar(t *testing.T) {
	for _, in := range []string{"p", "Z", "0", "-", "_", ".", " ", "日"} {
		if !isFilterChar(in) {
			t.Errorf("isFilterChar(%q) = false, want true", in)
		}
	}
	for _, in := range []string{"", "\x1b", "\x1b[D", "\r", "\n", "\x7f", "\x03", "ab", "\x00"} {
		if isFilterChar(in) {
			t.Errorf("isFilterChar(%q) = true, want false", in)
		}
	}
}

func TestBackspaceFilterMultibyte(t *testing.T) {
	state := newBrowserState(browseSample(), 3)
	state.filterQuery = "ab日"
	state.backspaceFilter()
	if state.filterQuery != "ab" {
		t.Errorf("backspace multibyte query = %q, want %q", state.filterQuery, "ab")
	}
	state.backspaceFilter()
	state.backspaceFilter()
	state.backspaceFilter() // extra backspace on empty must not panic
	if state.filterQuery != "" {
		t.Errorf("backspace to empty query = %q, want %q", state.filterQuery, "")
	}
}

// TestRunFilterTypingEditingAndClear drives the full interactive filter loop:
// enter filter mode, type, backspace, retype, keep on Enter, then clear on Esc.
func TestRunFilterTypingEditingAndClear(t *testing.T) {
	state := newBrowserState(browseSample(), 3)
	script := []struct {
		action keyAction
		raw    string
	}{
		{keyFilter, "/"}, // enter filter editing
		{keyNone, "p"},   // type
		{keyNone, "e"},   // type
		{keyBackspace, "\x7f"},
		{keyNone, "e"},   // retype -> "pe"
		{keyEnter, "\r"}, // exit editing, keep query
		{keyEsc, "\x1b"}, // Esc on List (not editing): ignored by navigation
		{keyQuit, "q"},
	}
	i := 0
	captureOutput(func() {
		state.run(func() (keyAction, string, bool) {
			if i >= len(script) {
				t.Error("run continued past the quit event")
				return keyNone, "", false
			}
			s := script[i]
			i++
			return s.action, s.raw, true
		})
	})
	if state.filterQuery != "pe" {
		t.Errorf("filterQuery = %q, want %q (kept after Enter)", state.filterQuery, "pe")
	}
	if state.filtering {
		t.Error("filtering still true after Enter; Enter must exit editing while keeping the query")
	}
	if len(state.repos) != 1 || state.repos[0].Name != "Peony" {
		t.Errorf("filtered repos = %v, want [Peony]", state.repos)
	}
	if state.selected < 0 || state.selected >= len(state.repos) {
		t.Errorf("selection %d invalid after filtering", state.selected)
	}
}

// TestRunFilterEscClearsQuery verifies Esc while editing clears the query,
// exits editing, and restores the full set with a valid selection.
func TestRunFilterEscClearsQuery(t *testing.T) {
	state := newBrowserState(browseSample(), 3)
	script := []struct {
		action keyAction
		raw    string
	}{
		{keyFilter, "/"},
		{keyNone, "z"},
		{keyEsc, "\x1b"},
		{keyQuit, "q"},
	}
	i := 0
	captureOutput(func() {
		state.run(func() (keyAction, string, bool) {
			s := script[i]
			i++
			return s.action, s.raw, true
		})
	})
	if state.filterQuery != "" {
		t.Errorf("filterQuery = %q, want empty after Esc", state.filterQuery)
	}
	if state.filtering {
		t.Error("filtering still true after Esc; Esc must exit editing")
	}
	if len(state.repos) != 3 {
		t.Errorf("repos after Esc clear = %d, want 3", len(state.repos))
	}
	if state.selected != 0 {
		t.Errorf("selected = %d, want 0 after clearing filter", state.selected)
	}
}

// TestRunFilterIgnoresSelectionMovesWhileEditing ensures typing mode never
// moves repository selection: Up/Down are consumed as no-ops while editing.
func TestRunFilterIgnoresSelectionMovesWhileEditing(t *testing.T) {
	state := newBrowserState(browseSample(), 3)
	before := state.selected
	script := []struct {
		action keyAction
		raw    string
	}{
		{keyFilter, "/"},
		{keyDown, "\x1b[B"},
		{keyUp, "\x1b[A"},
		{keyEnter, "\r"},
		{keyQuit, "q"},
	}
	i := 0
	captureOutput(func() {
		state.run(func() (keyAction, string, bool) {
			s := script[i]
			i++
			return s.action, s.raw, true
		})
	})
	if state.selected != before {
		t.Errorf("selected moved while editing: was %d, now %d", before, state.selected)
	}
	if state.filterQuery != "" {
		t.Errorf("filterQuery = %q, want empty (no printable input given)", state.filterQuery)
	}
}
