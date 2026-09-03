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
	if !strings.Contains(overview, "‹ LIST ›") {
		t.Errorf("list header missing: %q", overview)
	}
	if !strings.Contains(overview, "1 / 6") {
		t.Errorf("list indicator missing: %q", overview)
	}
	if !strings.Contains(overview, "↑/↓ select") {
		t.Errorf("list navigation hint missing: %q", overview)
	}

	activity := renderBrowse(browseSample(), 3, BrowseActivity, 40)
	if !strings.Contains(activity, "‹ ACTIVITY ›") {
		t.Errorf("activity header missing: %q", activity)
	}
	if !strings.Contains(activity, "3 / 6") {
		t.Errorf("activity indicator missing: %q", activity)
	}
	detail := renderBrowse(browseSample(), 3, BrowseDetail, 40)
	if !strings.Contains(detail, "‹ DETAIL ›") {
		t.Errorf("detail header missing: %q", detail)
	}
	if !strings.Contains(detail, "2 / 6") {
		t.Errorf("detail indicator missing: %q", detail)
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
	lines := strings.Split(renderBrowse(browseSample(), 3, BrowseList, 60), "\n")
	if browseHeaderLine < 0 || browseHeaderLine >= len(lines) {
		t.Fatalf("browseHeaderLine %d out of range for %d lines", browseHeaderLine, len(lines))
	}
	header := lines[browseHeaderLine]
	if !strings.Contains(header, "‹ LIST ›") || !strings.Contains(header, "1 / 6") {
		t.Errorf("line %d is not the view-indicator header: %q", browseHeaderLine, header)
	}
}

// TestRenderKeepsViewIdentityVisibleAtEveryOffset is the regression test for
// the audit's P1-1: the viewport must scroll body content, never navigation
// identity. It exercises browserState.render() (the composition that was
// buggy), not just renderBrowse.
func TestRenderKeepsViewIdentityVisibleAtEveryOffset(t *testing.T) {
	for _, page := range []BrowsePage{BrowseList, BrowseDetail, BrowseActivity, BrowseCommitHistory, BrowseCommitDetail, BrowseChangedFiles} {
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
	state := &browserState{repos: browseSample(), total: 3, page: BrowseList, viewport: 80}
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

func TestRunHierarchicalNavigationAndIgnoresUnknownInput(t *testing.T) {
	state := &browserState{repos: browseSample(), total: 3, page: BrowseList, viewport: 80}
	script := []struct {
		action keyAction
		ok     bool
	}{
		{keyDown, true},        // move selection down (list)
		{keyEnter, true},       // List -> Detail
		{keyEnter, true},       // Detail -> Activity
		{keyLeft, true},        // Activity -> Detail
		{keyEsc, true},         // Detail -> List
		{keyScrollRight, true}, // scroll within bounds (list)
		{keyNone, true},        // unknown keys are ignored, loop continues
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
		state.run(func() (keyAction, bool) {
			i++
			if i == 1 {
				return keyEnter, true
			}
			return keyQuit, true
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
		state.run(func() (keyAction, bool) {
			i++
			if i == 1 {
				return interpretSequence("\x1b[C"), true // plain →
			}
			return keyQuit, true
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
		state.run(func() (keyAction, bool) {
			i++
			if i == 1 {
				return keyEsc, true
			}
			return keyQuit, true
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
		state.run(func() (keyAction, bool) {
			i++
			if i == 1 {
				return keyEsc, true
			}
			return keyQuit, true
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

// --- v0.6.0: Selection tests ---

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

// --- v0.6.0: Vertical viewport tests ---

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
	s.viewportHeight = 10 // availableRows = 4
	s.selected = 0
	s.ensureVisible()
	if s.listOffset != 0 {
		t.Errorf("top: listOffset = %d, want 0", s.listOffset)
	}
	// Selected at bottom
	s.selected = 19
	s.ensureVisible()
	if s.listOffset != 16 {
		t.Errorf("bottom: listOffset = %d, want 16", s.listOffset)
	}
	// Scroll down
	s.selected = 5
	s.listOffset = 0
	s.ensureVisible()
	if s.listOffset != 2 {
		t.Errorf("scroll down: listOffset = %d, want 2", s.listOffset)
	}
	// Scroll up
	s.selected = 2
	s.ensureVisible()
	if s.listOffset != 2 {
		t.Errorf("scroll up: listOffset = %d, want 2 (still visible)", s.listOffset)
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

// --- v0.6.0: Navigation tests ---

func TestNavigationListToDetail(t *testing.T) {
	s := newBrowserState(browseSample(), 3)
	s.page = BrowseList
	captureOutput(func() {
		s.run(func() (keyAction, bool) {
			// Enter -> Detail, then quit
			if s.page == BrowseList {
				return keyEnter, true
			}
			return keyQuit, true
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
		s.run(func() (keyAction, bool) {
			i++
			if i == 1 {
				return keyLeft, true
			}
			return keyQuit, true
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
		s.run(func() (keyAction, bool) {
			i++
			if i == 1 {
				return keyEsc, true
			}
			return keyQuit, true
		})
	})
	if i != 2 {
		t.Errorf("detail Esc -> list then quit; reads = %d, want 2", i)
	}
	if s.page != BrowseList {
		t.Errorf("detail Esc -> list: page = %v, want List", s.page)
	}
}

func TestNavigationDetailToActivity(t *testing.T) {
	s := newBrowserState(browseSample(), 3)
	s.page = BrowseDetail
	captureOutput(func() {
		s.run(func() (keyAction, bool) {
			if s.page == BrowseDetail {
				return keyEnter, true
			}
			return keyQuit, true
		})
	})
	if s.page != BrowseActivity {
		t.Errorf("detail Enter -> activity: page = %v, want Activity", s.page)
	}
}

func TestNavigationActivityToDetail(t *testing.T) {
	s := newBrowserState(browseSample(), 3)
	s.page = BrowseActivity
	i := 0
	captureOutput(func() {
		s.run(func() (keyAction, bool) {
			i++
			if i == 1 {
				return keyLeft, true
			}
			return keyQuit, true
		})
	})
	if s.page != BrowseDetail {
		t.Errorf("activity Left -> detail: page = %v, want Detail", s.page)
	}
}

func TestNavigationQuitFromAllPages(t *testing.T) {
	for _, page := range []BrowsePage{BrowseList, BrowseDetail, BrowseActivity} {
		s := newBrowserState(browseSample(), 3)
		s.page = page
		calls := 0
		captureOutput(func() {
			s.run(func() (keyAction, bool) {
				calls++
				return keyQuit, true
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
		s.run(func() (keyAction, bool) {
			calls++
			return keyNone, false
		})
	})
	if calls != 1 {
		t.Errorf("EOF: calls = %d, want 1", calls)
	}
}

// TestNavigationKeepsSelectionValid drives the full hierarchy round-trip
// (List → Detail → Activity → Detail → List) and asserts the selection index
// stays a valid address into the repository set after every transition.
func TestNavigationKeepsSelectionValid(t *testing.T) {
	s := newBrowserState(browseSample(), 3)
	s.selected = 2
	i := 0
	captureOutput(func() {
		s.run(func() (keyAction, bool) {
			i++
			switch i {
			case 1:
				return keyEnter, true // List -> Detail
			case 2:
				return keyEnter, true // Detail -> Activity
			case 3:
				return keyLeft, true // Activity -> Detail
			case 4:
				return keyEsc, true // Detail -> List
			}
			return keyQuit, true
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

// TestNavigationFullV070Hierarchy drives the complete v0.7.0 hierarchy
// (List → Detail → Activity → Commit History → Commit Detail → Changed Files
// and back up to List) and asserts correct page transitions at each step.
func TestNavigationFullV070Hierarchy(t *testing.T) {
	s := newBrowserState(browseSample(), 3)
	s.history = []Commit{{Hash: "abc1234", Subject: "test commit"}}
	s.selectedCommit = 0

	actions := []keyAction{
		keyEnter, // List -> Detail
		keyEnter, // Detail -> Activity
		keyEnter, // Activity -> History
		keyEnter, // History -> Commit
		keyEnter, // Commit -> Files
		keyEsc,   // Files -> Commit
		keyEsc,   // Commit -> History
		keyEsc,   // History -> Activity
		keyEsc,   // Activity -> Detail
		keyEsc,   // Detail -> List
		keyQuit,  // exit
	}

	i := 0
	captureOutput(func() {
		s.run(func() (keyAction, bool) {
			if i >= len(actions) {
				return keyQuit, false
			}
			act := actions[i]
			i++
			return act, true
		})
	})
	if s.page != BrowseList {
		t.Errorf("final page = %v, want List", s.page)
	}
}

// --- v0.6.0: Detail rendering tests ---

func TestRenderDetail(t *testing.T) {
	repo := Repository{
		Name: "myrepo", Path: "/home/user/myrepo", Branch: "main", Upstream: "origin/main",
		Dirty: false, Ahead: 2, Behind: 1,
		LastCommitHash: "abc123def", LastCommitSubject: "fix bug", LastCommitTime: "2 hours ago",
	}
	out := renderDetail(repo, 80)
	for _, want := range []string{"myrepo", "/home/user/myrepo", "clean", "main", "origin/main", "2", "1", "abc123d", "fix bug", "2 hours ago"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail missing %q in %q", want, out)
		}
	}
	// Dirty
	repo.Dirty = true
	out = renderDetail(repo, 80)
	if !strings.Contains(out, "dirty") {
		t.Errorf("detail dirty: %q", out)
	}
	// No upstream
	repo.Upstream = ""
	out = renderDetail(repo, 80)
	if !strings.Contains(out, "—") {
		t.Errorf("detail no upstream: %q", out)
	}
	// Detached
	repo.Branch = ""
	out = renderDetail(repo, 80)
	if !strings.Contains(out, "detached") {
		t.Errorf("detail detached: %q", out)
	}
	// No commits
	repo.LastCommitHash = ""
	out = renderDetail(repo, 80)
	if !strings.Contains(out, "No commits") {
		t.Errorf("detail no commits: %q", out)
	}
	// Long path
	repo.Path = strings.Repeat("a", 100) + "/repo"
	out = renderDetail(repo, 80)
	if !strings.Contains(out, "a") {
		t.Errorf("detail long path: %q", out)
	}
	// Collision-resolved identity
	repo.Name = "work/api"
	out = renderDetail(repo, 80)
	if !strings.Contains(out, "work/api") {
		t.Errorf("detail collision identity: %q", out)
	}
}

// --- v0.6.0: Activity rendering tests (contextual) ---

func TestRenderActivitySelected(t *testing.T) {
	repo := Repository{Name: "myrepo", LastCommitHash: "abc123", LastCommitSubject: "feat", LastCommitTime: "now"}
	out := renderActivityForSelected(repo, 80)
	if !strings.Contains(out, "myrepo") || !strings.Contains(out, "abc123") {
		t.Errorf("activity selected: %q", out)
	}
	repo.LastCommitHash = ""
	out = renderActivityForSelected(repo, 80)
	if !strings.Contains(out, "No commits") {
		t.Errorf("activity no commits: %q", out)
	}
	long := Repository{Name: "r", LastCommitHash: "abc", LastCommitSubject: strings.Repeat("x", 100), LastCommitTime: "now"}
	out = renderActivityForSelected(long, 80)
	if !strings.Contains(out, "x") {
		t.Errorf("activity long subject: %q", out)
	}
	coll := Repository{Name: "work/api", LastCommitHash: "abc", LastCommitSubject: "s", LastCommitTime: "now"}
	out = renderActivityForSelected(coll, 80)
	if !strings.Contains(out, "work/api") {
		t.Errorf("activity collision identity: %q", out)
	}
}

// --- v0.6.0: Pipeline integrity ---

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
	detail := renderDetail(*repo, 80)
	if !strings.Contains(detail, "/personal/api") {
		t.Errorf("detail must show canonical Path: %q", detail)
	}
}

// --- v0.6.0: Rendering determinism ---

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

// --- v0.7.0: Commit detail & changed-files rendering tests ---

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
// static overview and exits immediately without attempting terminal control.
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

	if !strings.Contains(out, "‹ LIST ›") {
		t.Errorf("non-TTY browse must output static list view, got:\n%q", out)
	}
	if strings.Contains(out, "\x1b[2J") {
		t.Errorf("non-TTY browse must not emit clear-screen escape sequences:\n%q", out)
	}
}
