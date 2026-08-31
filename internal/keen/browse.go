package keen

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// keyAction is the normalized result of reading one user input event in the
// browser. Both raw (unix) and fallback (non-unix) readers feed interpretSequence
// so that navigation semantics stay identical across platforms.
type keyAction int

const (
	keyNone keyAction = iota
	keyLeft
	keyRight
	keyScrollLeft
	keyScrollRight
	keyQuit
	keyEsc
	keyUp
	keyDown
	keyEnter
)

// BrowsePage identifies the active surface of the browser.
type BrowsePage int

const (
	BrowseList BrowsePage = iota
	BrowseDetail
	BrowseActivity
	// Deprecated aliases for v0.5.x compatibility.
	BrowseOverview = BrowseList
)

const browsePageCount = 3

func (p BrowsePage) label() string {
	switch p {
	case BrowseList:
		return "LIST"
	case BrowseDetail:
		return "DETAIL"
	case BrowseActivity:
		return "ACTIVITY"
	}
	return "?"
}

// cyclePage moves between surfaces with wraparound. Direction is -1 (left) or
// +1 (right); any other value is treated as +1. Retained for compatibility
// and horizontal navigation where applicable.
func cyclePage(page BrowsePage, dir int) BrowsePage {
	if dir != -1 {
		dir = 1
	}
	n := int(page) + dir
	switch {
	case n < 0:
		n = browsePageCount - 1
	case n >= browsePageCount:
		n = 0
	}
	return BrowsePage(n)
}

// interpretSequence normalizes a single input token into a keyAction. It
// accepts literal key characters and ANSI escape sequences so the function
// stays pure and unit-testable independent of terminal state.
func interpretSequence(seq string) keyAction {
	switch seq {
	case "h", "\x1b[D":
		return keyLeft
	case "l", "\x1b[C":
		return keyRight
	case "H", "\x1b[1;2D", "\x1b[1;5D":
		return keyScrollLeft
	case "L", "\x1b[1;2C", "\x1b[1;5C":
		return keyScrollRight
	case "q", "\x03":
		return keyQuit
	case "\x1b":
		return keyEsc
	case "\x1b[A":
		return keyUp
	case "\x1b[B":
		return keyDown
	case "\r", "\n", "\x1bOM": // Enter: CR, LF, keypad Enter
		return keyEnter
	}
	return keyNone
}

const browseIndent = "    "

// browseHeaderLine is the index of the view-indicator header within the screen
// produced by renderBrowse. The layout contract is fixed: renderBrowse emits
// the banner (line 0), a blank separator (line 1), then the header carrying
// the active view label and page counter (line 2). browserState.render keeps
// exactly this line exempt from horizontal slicing so the active view remains
// identifiable at every offset; only body content scrolls.
const browseHeaderLine = 2

// browserState holds the concrete, local view state for one interactive
// session. It deliberately models distinct values for selection, vertical
// viewport, and horizontal viewport.
type browserState struct {
	repos          []Repository
	total          int
	page           BrowsePage
	selected       int // index of selected repo; -1 if none
	listOffset     int // vertical viewport offset
	offset         int // horizontal viewport offset
	viewport       int // terminal width
	viewportHeight int // terminal height
}

func newBrowserState(repos []Repository, total int) *browserState {
	selected := -1
	if len(repos) > 0 {
		selected = 0
	}
	return &browserState{
		repos:          repos,
		total:          total,
		page:           BrowseList,
		selected:       selected,
		viewport:       terminalWidth(),
		viewportHeight: terminalHeight(),
	}
}

func (b *browserState) clampSelection() {
	if len(b.repos) == 0 {
		b.selected = -1
		b.listOffset = 0
		return
	}
	if b.selected < 0 {
		b.selected = 0
	}
	if b.selected >= len(b.repos) {
		b.selected = len(b.repos) - 1
	}
}

func (b *browserState) moveSelection(delta int) {
	if len(b.repos) == 0 {
		return
	}
	b.selected += delta
	b.clampSelection()
	b.ensureVisible()
}

func (b *browserState) ensureVisible() {
	if len(b.repos) == 0 {
		b.listOffset = 0
		return
	}
	availableRows := b.availableRows()
	if availableRows <= 0 {
		availableRows = 1
	}
	// Ensure selected is within visible window.
	if b.selected < b.listOffset {
		b.listOffset = b.selected
	}
	if b.selected >= b.listOffset+availableRows {
		b.listOffset = b.selected - availableRows + 1
	}
	// Clamp listOffset to valid range.
	maxOffset := len(b.repos) - availableRows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if b.listOffset < 0 {
		b.listOffset = 0
	}
	if b.listOffset > maxOffset {
		b.listOffset = maxOffset
	}
}

func (b *browserState) availableRows() int {
	// Banner (1) + blank (1) + header (1) + blank (1) + hint (1) + blank (1) = 6
	// Detail/Activity have similar chrome. Keep conservative.
	h := b.viewportHeight
	if h <= 0 {
		h = 24
	}
	avail := h - 6
	if avail < 1 {
		avail = 1
	}
	return avail
}

func (b *browserState) selectedRepo() *Repository {
	if b.selected < 0 || b.selected >= len(b.repos) {
		return nil
	}
	return &b.repos[b.selected]
}

func (b *browserState) contentWidth() int {
	// Width is determined by the current page's full content.
	var full string
	switch b.page {
	case BrowseList:
		full = b.renderListContent()
	case BrowseDetail:
		if repo := b.selectedRepo(); repo != nil {
			full = renderDetail(*repo, b.viewport)
		} else {
			full = renderBrowse(b.repos, b.total, b.page, b.viewport)
		}
	case BrowseActivity:
		if repo := b.selectedRepo(); repo != nil {
			full = renderActivityForSelected(*repo, b.viewport)
		} else {
			full = renderBrowse(b.repos, b.total, b.page, b.viewport)
		}
	default:
		full = renderBrowse(b.repos, b.total, b.page, b.viewport)
	}
	max := 0
	for _, line := range strings.Split(full, "\n") {
		if w := len([]rune(line)); w > max {
			max = w
		}
	}
	return max
}

func (b *browserState) scroll(delta int) {
	maxOffset := b.contentWidth() - b.viewport
	if maxOffset < 0 {
		maxOffset = 0
	}
	b.offset += delta
	if b.offset < 0 {
		b.offset = 0
	}
	if b.offset > maxOffset {
		b.offset = maxOffset
	}
}

func (b *browserState) render() {
	// Refresh viewport dimensions each render to avoid stale startup width.
	b.viewport = terminalWidth()
	b.viewportHeight = terminalHeight()
	b.ensureVisible()

	var full string
	switch b.page {
	case BrowseList:
		full = b.renderListContent()
	case BrowseDetail:
		if repo := b.selectedRepo(); repo != nil {
			full = renderDetail(*repo, b.viewport)
			// Wrap detail with banner/header chrome for consistency.
			header := b.renderHeader()
			full = header + full
		} else {
			full = renderBrowse(b.repos, b.total, b.page, b.viewport)
		}
	case BrowseActivity:
		if repo := b.selectedRepo(); repo != nil {
			full = renderActivityForSelected(*repo, b.viewport)
			header := b.renderHeader()
			full = header + full
		} else {
			full = renderBrowse(b.repos, b.total, b.page, b.viewport)
		}
	default:
		full = renderBrowse(b.repos, b.total, b.page, b.viewport)
	}
	lines := strings.Split(full, "\n")
	var out strings.Builder
	for i, line := range lines {
		if i == browseHeaderLine {
			// The view indicator is navigation identity, not content: it
			// must remain visible and stable at every horizontal offset.
			// Only body lines scroll.
			out.WriteString(line)
		} else {
			out.WriteString(sliceViewport(line, b.offset, b.viewport))
		}
		if i < len(lines)-1 {
			out.WriteString("\n")
		}
	}
	fmt.Print("\x1b[2J\x1b[H")
	fmt.Print(out.String())
}

func (b *browserState) renderHeader() string {
	var sb strings.Builder
	sb.WriteString("===KEEN===\n\n")
	title := "‹ " + b.page.label() + " ›"
	indicator := fmt.Sprintf("%d / %d", int(b.page)+1, browsePageCount)
	pad := b.viewport - len([]rune(title)) - len([]rune(indicator))
	if pad < 0 {
		pad = 0
	}
	sb.WriteString(title)
	sb.WriteString(strings.Repeat(" ", pad))
	sb.WriteString(indicator)
	sb.WriteString("\n\n")
	return sb.String()
}

func (b *browserState) renderListContent() string {
	var sb strings.Builder
	sb.WriteString("===KEEN===\n\n")
	title := "‹ " + b.page.label() + " ›"
	indicator := fmt.Sprintf("%d / %d", int(b.page)+1, browsePageCount)
	pad := b.viewport - len([]rune(title)) - len([]rune(indicator))
	if pad < 0 {
		pad = 0
	}
	sb.WriteString(title)
	sb.WriteString(strings.Repeat(" ", pad))
	sb.WriteString(indicator)
	sb.WriteString("\n\n")

	if len(b.repos) == 0 {
		sb.WriteString(browseIndent + emptyMessage(b.total) + "\n\n")
		sb.WriteString(browseIndent + "←/→ views    Shift+←/→ scroll    q quit\n")
		return sb.String()
	}

	// Ensure selected visible before rendering.
	b.ensureVisible()
	availableRows := b.availableRows()
	start := b.listOffset
	end := start + availableRows
	if end > len(b.repos) {
		end = len(b.repos)
	}

	nameW, branchW, upstreamW := overviewWidths(b.viewport)
	for i := start; i < end; i++ {
		r := b.repos[i]
		cursor := "  "
		if i == b.selected {
			cursor = "> "
		}
		// Compact row: identity, status, branch, upstream where appropriate.
		row := fmt.Sprintf("%s%s[%-5s] %-*s %-*s %-*s %s",
			browseIndent+cursor, "", statusOf(r),
			nameW, r.Name,
			branchW, overviewBranch(r),
			upstreamW, overviewUpstream(r),
			overviewAheadBehind(r))
		sb.WriteString(row + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString(browseIndent + "↑/↓ select    Enter inspect    q quit\n")
	return sb.String()
}

// run consumes input events until the user quits or the input stream ends.
// The read function is injected so the navigation loop stays exercisable
// without a terminal; production Browse passes readKey.
func (b *browserState) run(read func() (keyAction, bool)) {
	for {
		action, ok := read()
		if !ok {
			// The stream ended or failed permanently (EOF, closed or hung-up
			// tty, I/O error). Quit cleanly so the caller's deferred
			// restoreRaw executes; never busy-loop on a dead stream.
			return
		}
		switch action {
		case keyUp:
			if b.page == BrowseList {
				b.moveSelection(-1)
				b.render()
			}
		case keyDown:
			if b.page == BrowseList {
				b.moveSelection(1)
				b.render()
			}
		case keyEnter:
			if b.page == BrowseList && b.selected >= 0 {
				b.page = BrowseDetail
				b.offset = 0
				b.render()
			}
		case keyLeft, keyEsc:
			switch b.page {
			case BrowseDetail:
				b.page = BrowseList
				b.offset = 0
				b.render()
			case BrowseActivity:
				b.page = BrowseDetail
				b.offset = 0
				b.render()
			case BrowseList:
				// Left/Esc are not bound from the list; q or Ctrl+C quit.
			}
		case keyRight:
			// In list, right could also enter detail; keep Enter as primary.
			// For detail, right enters activity.
			if b.page == BrowseDetail {
				b.page = BrowseActivity
				b.offset = 0
				b.render()
			}
		case keyScrollLeft:
			b.scroll(-max(1, b.viewport/2))
			b.render()
		case keyScrollRight:
			b.scroll(max(1, b.viewport/2))
			b.render()
		case keyQuit:
			return
		}
		// Horizontal scroll via Shift+arrows already handled; vertical via Up/Down.
		// Unknown keys (keyNone) are ignored.
	}
}

// Browse opens the interactive browser over the already discovered, enriched,
// sorted, and filtered repository set, entered via `keen -i`. It answers the
// two questions the flat CLI output leaves implicit: Overview shows what needs
// attention (status, branch, upstream, ahead/behind); Activity shows what
// happened (short hash, subject, relative time).
//
// Terminal handling uses the standard library only (no added dependencies).
// When the input is not a terminal, makeRaw fails and Browse degrades to a
// single static Overview render so the command still produces useful output.
func Browse(repos []Repository, totalDiscovered int) {
	width := terminalWidth()
	fd := int(os.Stdin.Fd())
	if err := makeRaw(fd); err != nil {
		fmt.Print(renderBrowse(repos, totalDiscovered, BrowseList, width))
		return
	}
	defer restoreRaw(fd)

	// Minimal signal protection for raw terminal: restore on SIGTERM/SIGHUP.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sigCh
		restoreRaw(fd)
		// Use raw syscall exit to avoid running other defers that might clear again.
		syscall.Exit(1)
	}()
	defer signal.Stop(sigCh)

	state := newBrowserState(repos, totalDiscovered)
	state.viewport = width
	state.viewportHeight = terminalHeight()
	state.render()
	state.run(readKey)
}

// sliceViewport returns the rune window [offset, offset+width) of s, with the
// offset and end clamped to the string bounds. It is the single primitive that
// realizes the horizontal viewport: a view wider than the terminal is revealed
// progressively by moving the offset.
func sliceViewport(s string, offset, width int) string {
	r := []rune(s)
	if offset < 0 {
		offset = 0
	}
	if offset > len(r) {
		offset = len(r)
	}
	end := offset + width
	if end > len(r) {
		end = len(r)
	}
	return string(r[offset:end])
}

// renderBrowse assembles the full (untruncated) screen for a page. Width
// controls only the header padding and the decorative rules; body lines are
// rendered at full content width and revealed through the horizontal viewport
// by the browser. This keeps the renderers independent of terminal state and
// directly assertable in tests.
func renderBrowse(repos []Repository, totalDiscovered int, page BrowsePage, width int) string {
	var sb strings.Builder
	sb.WriteString("===KEEN===\n\n")

	title := "‹ " + page.label() + " ›"
	indicator := fmt.Sprintf("%d / %d", int(page)+1, browsePageCount)
	pad := width - len([]rune(title)) - len([]rune(indicator))
	if pad < 0 {
		pad = 0
	}
	sb.WriteString(title)
	sb.WriteString(strings.Repeat(" ", pad))
	sb.WriteString(indicator)
	sb.WriteString("\n\n")

	switch page {
	case BrowseList:
		sb.WriteString(renderOverview(repos, totalDiscovered, width))
	case BrowseDetail:
		// Detail via browse's contextual renderer; fallback to overview for tests.
		if len(repos) > 0 {
			sb.WriteString(renderDetail(repos[0], width))
		} else {
			sb.WriteString(renderOverview(repos, totalDiscovered, width))
		}
	case BrowseActivity:
		sb.WriteString(renderActivity(repos, totalDiscovered, width))
	default:
		sb.WriteString(renderOverview(repos, totalDiscovered, width))
	}

	var hint string
	switch page {
	case BrowseList:
		hint = "↑/↓ select    Enter inspect    q quit"
	case BrowseDetail, BrowseActivity:
		hint = "←/Esc back    q quit"
	}
	sb.WriteString(browseIndent + hint + "\n")
	return sb.String()
}

func renderOverview(repos []Repository, totalDiscovered int, width int) string {
	if len(repos) == 0 {
		return browseIndent + emptyMessage(totalDiscovered) + "\n"
	}

	nameW, branchW, upstreamW := overviewWidths(width)
	var sb strings.Builder

	writeSection := func(title string, dirty bool) {
		var rows []string
		for _, r := range repos {
			if r.Dirty == dirty {
				rows = append(rows, overviewRow(r, nameW, branchW, upstreamW))
			}
		}
		if len(rows) == 0 {
			return
		}
		sb.WriteString(browseIndent + title + "\n")
		sb.WriteString(browseIndent + dashes(width) + "\n\n")
		for _, row := range rows {
			sb.WriteString(row + "\n")
		}
		sb.WriteString("\n")
	}

	writeSection("CLEAN", false)
	writeSection("DIRTY", true)
	return sb.String()
}

func renderActivity(repos []Repository, totalDiscovered int, width int) string {
	if len(repos) == 0 {
		return browseIndent + emptyMessage(totalDiscovered) + "\n"
	}

	var sb strings.Builder

	writeSection := func(title string, dirty bool) {
		var entries []string
		for _, r := range repos {
			if r.Dirty == dirty {
				entries = append(entries, activityEntry(r))
			}
		}
		if len(entries) == 0 {
			return
		}
		sb.WriteString(browseIndent + title + "\n")
		sb.WriteString(browseIndent + dashes(width) + "\n\n")
		for _, e := range entries {
			sb.WriteString(e)
		}
		sb.WriteString("\n")
	}

	writeSection("CLEAN", false)
	writeSection("DIRTY", true)
	return sb.String()
}

// overviewRow presents the Overview contract: status, name, branch, upstream,
// ahead, behind. The row is rendered at full content width (no field
// truncation); the horizontal viewport reveals any content wider than the
// terminal.
func overviewRow(r Repository, nameW, branchW, upstreamW int) string {
	return fmt.Sprintf("%s[%-5s] %-*s %-*s %-*s %s",
		browseIndent, statusOf(r),
		nameW, r.Name,
		branchW, overviewBranch(r),
		upstreamW, overviewUpstream(r),
		overviewAheadBehind(r))
}

// activityEntry presents the Activity contract: name, short hash, subject,
// relative commit time. When a repository has no commits it shows "No commits".
// The subject is shown in full; the horizontal viewport handles width.
func activityEntry(r Repository) string {
	var sb strings.Builder
	sb.WriteString(browseIndent + r.Name + "\n")

	if r.LastCommitHash == "" {
		sb.WriteString(browseIndent + "  No commits\n")
		return sb.String()
	}

	sb.WriteString(browseIndent + "  " + shortHash(r.LastCommitHash) + "  " + r.LastCommitSubject + "\n")
	sb.WriteString(browseIndent + "  " + r.LastCommitTime + "\n")
	return sb.String()
}

// renderDetail renders the bounded repository detail view for one selected
// repository. It consumes only existing Repository facts.
func renderDetail(repo Repository, width int) string {
	var sb strings.Builder
	sb.WriteString(browseIndent + "Repository: " + repo.Name + "\n")
	sb.WriteString(browseIndent + "Path:       " + repo.Path + "\n")
	status := statusOf(repo)
	sb.WriteString(browseIndent + "Status:     " + status + "\n")
	branch := overviewBranch(repo)
	sb.WriteString(browseIndent + "Branch:     " + branch + "\n")
	upstream := overviewUpstream(repo)
	sb.WriteString(browseIndent + "Upstream:   " + upstream + "\n")
	// Ahead/Behind honest handling.
	ahead, behind := "–", "–"
	if repo.Upstream != "" {
		ahead = fmt.Sprintf("%d", repo.Ahead)
		behind = fmt.Sprintf("%d", repo.Behind)
	}
	sb.WriteString(browseIndent + "Ahead:      " + ahead + "\n")
	sb.WriteString(browseIndent + "Behind:     " + behind + "\n")
	sb.WriteString(browseIndent + "Last commit:\n")
	if repo.LastCommitHash == "" {
		sb.WriteString(browseIndent + "  No commits\n")
	} else {
		sb.WriteString(browseIndent + "  " + shortHash(repo.LastCommitHash) + "  " + repo.LastCommitSubject + "\n")
		sb.WriteString(browseIndent + "  " + repo.LastCommitTime + "\n")
	}
	// Use width to avoid accidental wrapping; detail relies on horizontal viewport.
	_ = width
	return sb.String()
}

// renderActivityForSelected renders Activity contextual to the selected repository.
func renderActivityForSelected(repo Repository, width int) string {
	var sb strings.Builder
	sb.WriteString(browseIndent + "Repository: " + repo.Name + "\n\n")
	if repo.LastCommitHash == "" {
		sb.WriteString(browseIndent + "  No commits\n")
		return sb.String()
	}
	sb.WriteString(browseIndent + "  " + shortHash(repo.LastCommitHash) + "  " + repo.LastCommitSubject + "\n")
	sb.WriteString(browseIndent + "  " + repo.LastCommitTime + "\n")
	_ = width
	return sb.String()
}

func overviewWidths(width int) (int, int, int) {
	avail := width - 22
	if avail < 18 {
		avail = 18
	}
	nameW := avail * 3 / 7
	branchW := avail * 2 / 7
	upstreamW := avail * 2 / 7
	if nameW < 6 {
		nameW = 6
	}
	if branchW < 6 {
		branchW = 6
	}
	if upstreamW < 6 {
		upstreamW = 6
	}
	if nameW > 30 {
		nameW = 30
	}
	if branchW > 24 {
		branchW = 24
	}
	if upstreamW > 24 {
		upstreamW = 24
	}
	return nameW, branchW, upstreamW
}

func dashes(width int) string {
	n := width - len([]rune(browseIndent))
	if n < 8 {
		n = 8
	}
	if n > 60 {
		n = 60
	}
	return strings.Repeat("-", n)
}

func emptyMessage(totalDiscovered int) string {
	if totalDiscovered == 0 {
		return "No repositories found."
	}
	return "No repositories match the selected filters."
}

func statusOf(r Repository) string {
	if r.Dirty {
		return "dirty"
	}
	return "clean"
}

func overviewBranch(r Repository) string {
	if r.Branch == "" {
		return "detached"
	}
	return r.Branch
}

func overviewUpstream(r Repository) string {
	if r.Upstream == "" {
		return "—"
	}
	return r.Upstream
}

func overviewAheadBehind(r Repository) string {
	if r.Upstream == "" {
		return "↑– ↓–"
	}
	return fmt.Sprintf("↑%d ↓%d", r.Ahead, r.Behind)
}

func shortHash(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}

// truncate shortens s to at most max runes, appending an ellipsis when the
// content is longer. It is rune-aware so multi-byte characters are never
// split mid-codepoint. Used by the static rich report; the interactive browser
// relies on the horizontal viewport instead.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}
