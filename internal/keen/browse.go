package keen

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// keyAction is the normalized result of reading one user input event in the
// interactive investigation surface. Both raw (unix) and fallback (non-unix)
// readers feed interpretSequence so that navigation semantics stay identical
// across platforms.
type keyAction int

const (
	keyNone keyAction = iota
	keyLeft
	keyScrollLeft
	keyScrollRight
	keyQuit
	keyEsc
	keyUp
	keyDown
	keyEnter
)

// BrowsePage identifies the active surface of the interactive investigation.
type BrowsePage int

const (
	BrowseList BrowsePage = iota
	BrowseDetail
	BrowseActivity
	BrowseCommitHistory
	BrowseCommitDetail
	BrowseChangedFiles
)

const browsePageCount = 6

func (p BrowsePage) label() string {
	switch p {
	case BrowseList:
		return "LIST"
	case BrowseDetail:
		return "DETAIL"
	case BrowseActivity:
		return "ACTIVITY"
	case BrowseCommitHistory:
		return "HISTORY"
	case BrowseCommitDetail:
		return "COMMIT"
	case BrowseChangedFiles:
		return "FILES"
	}
	return "?"
}

// interpretSequence normalizes a single input token into a keyAction. It
// accepts literal key characters and ANSI escape sequences so the function
// stays pure and unit-testable independent of terminal state.
//
// Navigation is hierarchical, not cyclic: Enter advances to a child surface,
// ←/Esc return to the parent, and there is no wrap-around "next view" key.
func interpretSequence(seq string) keyAction {
	switch seq {
	case "\x1b[D":
		return keyLeft
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

// browseHeaderLine is the index of the view-indicator header within the screen
// produced by renderBrowse. The layout contract is fixed: renderBrowse emits
// the banner (line 0), a blank separator (line 1), then the header carrying
// the active view label and page counter (line 2). browserState.render keeps
// exactly this line exempt from horizontal slicing so the active view remains
// identifiable at every offset; only body content scrolls.
const browseHeaderLine = 2

// browseChromeRows is the fixed count of vertical chrome rows occupied by the
// banner, separators, header, and hint on every interactive page. availableRows
// reserves this many rows before laying out selectable body content so the
// chrome never overlaps the visible selection window.
const browseChromeRows = 6

// browserState holds the concrete, local view state for one interactive
// session. It deliberately models distinct values for selection, vertical
// viewport, and horizontal viewport.
type browserState struct {
	repos              []Repository
	total              int
	page               BrowsePage
	selected           int // index of selected repo; -1 if none
	listOffset         int // vertical viewport offset
	offset             int // horizontal viewport offset
	viewport           int // terminal width
	viewportHeight     int // terminal height
	history            []Commit
	selectedCommit     int // index of selected commit; -1 if none
	historyOffset      int // vertical viewport offset for commit history
	changedFiles       []ChangedFile
	changedFilesOffset int
	historyErr         string
	changedFilesErr    string
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
		selectedCommit: -1,
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
	h := b.viewportHeight
	if h <= 0 {
		h = 24
	}
	avail := h - browseChromeRows
	if avail < 1 {
		avail = 1
	}
	return avail
}

func (b *browserState) clampCommitSelection() {
	if len(b.history) == 0 {
		b.selectedCommit = -1
		b.historyOffset = 0
		return
	}
	if b.selectedCommit < 0 {
		b.selectedCommit = 0
	}
	if b.selectedCommit >= len(b.history) {
		b.selectedCommit = len(b.history) - 1
	}
}

func (b *browserState) moveCommitSelection(delta int) {
	if len(b.history) == 0 {
		return
	}
	b.selectedCommit += delta
	b.clampCommitSelection()
	b.ensureCommitVisible()
}

func (b *browserState) ensureCommitVisible() {
	if len(b.history) == 0 {
		b.historyOffset = 0
		return
	}
	availableRows := b.availableRows()
	if availableRows <= 0 {
		availableRows = 1
	}
	if b.selectedCommit < b.historyOffset {
		b.historyOffset = b.selectedCommit
	}
	if b.selectedCommit >= b.historyOffset+availableRows {
		b.historyOffset = b.selectedCommit - availableRows + 1
	}
	maxOffset := len(b.history) - availableRows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if b.historyOffset < 0 {
		b.historyOffset = 0
	}
	if b.historyOffset > maxOffset {
		b.historyOffset = maxOffset
	}
}

func (b *browserState) selectedCommitObj() *Commit {
	if b.selectedCommit < 0 || b.selectedCommit >= len(b.history) {
		return nil
	}
	return &b.history[b.selectedCommit]
}

func (b *browserState) loadHistoryForSelected() {
	b.history = nil
	b.selectedCommit = -1
	b.historyOffset = 0
	b.historyErr = ""
	b.changedFiles = nil
	b.changedFilesOffset = 0
	b.changedFilesErr = ""
	repo := b.selectedRepo()
	if repo == nil {
		return
	}
	commits, err := loadCommitHistory(repo.Path)
	if err != nil {
		b.historyErr = err.Error()
		return
	}
	b.history = commits
	if len(commits) > 0 {
		b.selectedCommit = 0
	} else {
		b.selectedCommit = -1
	}
}

func (b *browserState) loadChangedFilesForSelectedCommit() {
	b.changedFiles = nil
	b.changedFilesOffset = 0
	b.changedFilesErr = ""
	repo := b.selectedRepo()
	commit := b.selectedCommitObj()
	if repo == nil || commit == nil {
		return
	}
	files, err := loadChangedFiles(repo.Path, commit.Hash)
	if err != nil {
		b.changedFilesErr = err.Error()
		return
	}
	b.changedFiles = files
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
			full = renderDetail(*repo)
		} else {
			full = renderBrowse(b.repos, b.total, b.page, b.viewport)
		}
	case BrowseActivity:
		if repo := b.selectedRepo(); repo != nil {
			full = renderActivityForSelected(*repo, b.viewport)
		} else {
			full = renderBrowse(b.repos, b.total, b.page, b.viewport)
		}
	case BrowseCommitHistory:
		full = b.renderCommitHistoryContent()
	case BrowseCommitDetail:
		if c := b.selectedCommitObj(); c != nil {
			if repo := b.selectedRepo(); repo != nil {
				full = renderCommitDetail(*repo, *c)
			} else {
				full = renderBrowse(b.repos, b.total, b.page, b.viewport)
			}
		} else {
			full = renderBrowse(b.repos, b.total, b.page, b.viewport)
		}
	case BrowseChangedFiles:
		if c := b.selectedCommitObj(); c != nil {
			if repo := b.selectedRepo(); repo != nil {
				full = renderChangedFilesForCommit(*repo, *c, b.changedFiles, b.changedFilesErr)
			} else {
				full = renderBrowse(b.repos, b.total, b.page, b.viewport)
			}
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
	if b.page == BrowseCommitHistory {
		b.ensureCommitVisible()
	}

	var full string
	switch b.page {
	case BrowseList:
		full = b.renderListContent()
	case BrowseDetail:
		if repo := b.selectedRepo(); repo != nil {
			full = renderDetail(*repo)
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
	case BrowseCommitHistory:
		full = b.renderCommitHistoryContent()
	case BrowseCommitDetail:
		if c := b.selectedCommitObj(); c != nil {
			if repo := b.selectedRepo(); repo != nil {
				full = renderCommitDetail(*repo, *c)
				header := b.renderHeader()
				full = header + full
			} else {
				full = renderBrowse(b.repos, b.total, b.page, b.viewport)
			}
		} else {
			full = renderBrowse(b.repos, b.total, b.page, b.viewport)
		}
	case BrowseChangedFiles:
		if c := b.selectedCommitObj(); c != nil {
			if repo := b.selectedRepo(); repo != nil {
				full = renderChangedFilesForCommit(*repo, *c, b.changedFiles, b.changedFilesErr)
				header := b.renderHeader()
				full = header + full
			} else {
				full = renderBrowse(b.repos, b.total, b.page, b.viewport)
			}
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
	return renderBannerAndHeader(b.page, b.viewport)
}

// renderListContent renders the List surface, prefixed by the shared banner
// and active-view header.
func (b *browserState) renderListContent() string {
	var sb strings.Builder
	sb.WriteString(renderBannerAndHeader(b.page, b.viewport))

	if len(b.repos) == 0 {
		sb.WriteString(reportIndent + emptyMessage(b.total) + "\n\n")
		sb.WriteString(reportIndent + "←/→ views    Shift+←/→ scroll    q quit\n")
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
			reportIndent+cursor, "", repoStatus(r),
			nameW, r.Name,
			branchW, branchLabel(r),
			upstreamW, upstreamLabel(r),
			aheadBehindLabel(r))
		sb.WriteString(row + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString(reportIndent + "↑/↓ select    Enter inspect    q quit\n")
	return sb.String()
}

func (b *browserState) renderCommitHistoryContent() string {
	var sb strings.Builder
	sb.WriteString(renderBannerAndHeader(b.page, b.viewport))

	repo := b.selectedRepo()
	if repo == nil {
		sb.WriteString(reportIndent + "No repository selected.\n\n")
		sb.WriteString(reportIndent + "←/Esc back    q quit\n")
		return sb.String()
	}
	sb.WriteString(reportIndent + "Repository: " + repo.Name + "\n\n")
	if b.historyErr != "" {
		sb.WriteString(reportIndent + "Failed to load history: " + b.historyErr + "\n\n")
		sb.WriteString(reportIndent + "←/Esc back    q quit\n")
		return sb.String()
	}
	if len(b.history) == 0 {
		sb.WriteString(reportIndent + "No commits.\n\n")
		sb.WriteString(reportIndent + "←/Esc back    q quit\n")
		return sb.String()
	}
	b.ensureCommitVisible()
	availableRows := b.availableRows()
	start := b.historyOffset
	end := start + availableRows
	if end > len(b.history) {
		end = len(b.history)
	}
	for i := start; i < end; i++ {
		c := b.history[i]
		cursor := "  "
		if i == b.selectedCommit {
			cursor = "> "
		}
		row := fmt.Sprintf("%s%s%s  %s  %s", reportIndent+cursor, shortHash(c.Hash), " ", c.Subject, c.AuthorDate)
		// Simplified row; horizontal viewport reveals overflow.
		sb.WriteString(row + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString(reportIndent + "↑/↓ select    Enter detail    ←/Esc back    q quit\n")
	return sb.String()
}

func renderCommitDetail(repo Repository, c Commit) string {
	var sb strings.Builder
	sb.WriteString(reportIndent + "Repository: " + repo.Name + "\n")
	sb.WriteString(reportIndent + "Commit:     " + c.Hash + "\n")
	sb.WriteString(reportIndent + "Author:     " + c.Author + " <" + c.AuthorDate + ">\n")
	sb.WriteString(reportIndent + "Committer:  " + c.Committer + " <" + c.CommitterDate + ">\n")
	if len(c.Parents) == 0 {
		sb.WriteString(reportIndent + "Parents:    (none - root commit)\n")
	} else {
		sb.WriteString(reportIndent + "Parents:    " + strings.Join(c.Parents, " ") + "\n")
	}
	sb.WriteString(reportIndent + "Subject:    " + c.Subject + "\n")
	if strings.TrimSpace(c.Body) != "" {
		sb.WriteString(reportIndent + "Body:\n")
		for _, line := range strings.Split(c.Body, "\n") {
			sb.WriteString(reportIndent + "  " + line + "\n")
		}
	}
	return sb.String()
}

func renderChangedFilesForCommit(repo Repository, c Commit, files []ChangedFile, filesErr string) string {
	var sb strings.Builder
	sb.WriteString(reportIndent + "Repository: " + repo.Name + "\n")
	sb.WriteString(reportIndent + "Commit:     " + shortHash(c.Hash) + "  " + c.Subject + "\n\n")
	if filesErr != "" {
		sb.WriteString(reportIndent + "Failed to load changed files: " + filesErr + "\n")
		return sb.String()
	}
	if len(files) == 0 {
		sb.WriteString(reportIndent + "No files changed.\n")
		return sb.String()
	}
	for _, f := range files {
		if f.OldPath != "" {
			sb.WriteString(reportIndent + f.Status + "  " + f.OldPath + " -> " + f.Path + "\n")
		} else {
			sb.WriteString(reportIndent + f.Status + "  " + f.Path + "\n")
		}
	}
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
			switch b.page {
			case BrowseList:
				b.moveSelection(-1)
				b.render()
			case BrowseCommitHistory:
				b.moveCommitSelection(-1)
				b.render()
			}
		case keyDown:
			switch b.page {
			case BrowseList:
				b.moveSelection(1)
				b.render()
			case BrowseCommitHistory:
				b.moveCommitSelection(1)
				b.render()
			}
		case keyEnter:
			switch b.page {
			case BrowseList:
				if b.selected >= 0 {
					b.page = BrowseDetail
					b.offset = 0
					b.render()
				}
			case BrowseDetail:
				b.page = BrowseActivity
				b.offset = 0
				b.render()
			case BrowseActivity:
				b.loadHistoryForSelected()
				b.page = BrowseCommitHistory
				b.offset = 0
				b.render()
			case BrowseCommitHistory:
				if b.selectedCommit >= 0 && b.selectedCommit < len(b.history) {
					b.page = BrowseCommitDetail
					b.offset = 0
					b.render()
				}
			case BrowseCommitDetail:
				b.loadChangedFilesForSelectedCommit()
				b.page = BrowseChangedFiles
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
			case BrowseCommitHistory:
				b.page = BrowseActivity
				b.offset = 0
				b.render()
			case BrowseCommitDetail:
				b.page = BrowseCommitHistory
				b.offset = 0
				b.render()
			case BrowseChangedFiles:
				b.page = BrowseCommitDetail
				b.offset = 0
				b.render()
			case BrowseList:
				// Left/Esc are not bound from the list; q or Ctrl+C quit.
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
		// Unknown keys (keyNone) are ignored.
	}
}

// isTerminal reports whether stdin is attached to a character device (a real
// terminal) rather than a pipe, file, or other redirected stream. It is the
// portable gate between interactive rendering and the deterministic one-shot
// overview: keeping control sequences out of redirected output relies on this
// check on every platform, including those where makeRaw is a no-op.
func isTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// Browse opens the interactive investigation surface over the already
// discovered, enriched, sorted, and filtered repository set, entered via
// `keen -i`. The hierarchy is List → Detail → Activity: Enter advances to a
// child surface, ←/Esc return to the parent, and q/Ctrl+C quit from any level.
// List is an addressable index; Detail shows one repository's full state;
// Activity is contextual to the selected repository.
//
// Terminal handling uses the standard library only (no added dependencies).
// When the input is not a terminal, Browse degrades to a single static List
// render so the command still produces useful, portable output.
func Browse(repos []Repository, totalDiscovered int) {
	width := terminalWidth()
	if !isTerminal() {
		fmt.Print(renderBrowse(repos, totalDiscovered, BrowseList, width))
		return
	}
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

// renderBannerAndHeader emits the shared screen chrome: the KEEN banner, a
// blank separator, the active-view line carrying the view label and page
// counter, and a trailing blank separator. It is the single canonical source
// for this chrome so every page renders an identical header.
func renderBannerAndHeader(page BrowsePage, width int) string {
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
	return sb.String()
}

// renderBrowse assembles the full (untruncated) screen for a page. Width
// controls only the header padding and the decorative rules; body lines are
// rendered at full content width and revealed through the horizontal viewport
// by the browserState renderer. This keeps the renderers independent of
// terminal state and directly assertable in tests.
func renderBrowse(repos []Repository, totalDiscovered int, page BrowsePage, width int) string {
	var sb strings.Builder
	sb.WriteString(renderBannerAndHeader(page, width))

	switch page {
	case BrowseList:
		sb.WriteString(renderOverview(repos, totalDiscovered, width))
	case BrowseDetail:
		// Detail via browse's contextual renderer; fallback to overview for tests.
		if len(repos) > 0 {
			sb.WriteString(renderDetail(repos[0]))
		} else {
			sb.WriteString(renderOverview(repos, totalDiscovered, width))
		}
	case BrowseActivity:
		sb.WriteString(renderActivity(repos, totalDiscovered, width))
	case BrowseCommitHistory:
		sb.WriteString(reportIndent + "Commit history requires interactive selection.\n")
	case BrowseCommitDetail:
		sb.WriteString(reportIndent + "Commit detail requires interactive selection.\n")
	case BrowseChangedFiles:
		sb.WriteString(reportIndent + "Changed files require interactive selection.\n")
	default:
		sb.WriteString(renderOverview(repos, totalDiscovered, width))
	}

	var hint string
	switch page {
	case BrowseList:
		hint = "↑/↓ select    Enter inspect    q quit"
	case BrowseDetail, BrowseActivity:
		hint = "←/Esc back    q quit"
	case BrowseCommitHistory:
		hint = "↑/↓ select    Enter detail    ←/Esc back    q quit"
	case BrowseCommitDetail:
		hint = "Enter files    ←/Esc back    q quit"
	case BrowseChangedFiles:
		hint = "←/Esc back    q quit"
	}
	sb.WriteString(reportIndent + hint + "\n")
	return sb.String()
}

func renderOverview(repos []Repository, totalDiscovered int, width int) string {
	if len(repos) == 0 {
		return reportIndent + emptyMessage(totalDiscovered) + "\n"
	}

	nameW, branchW, upstreamW := overviewWidths(width)
	var sb strings.Builder

	writeSection := func(title string, dirty bool) {
		var rows []string
		for _, r := range repos {
			if r.Dirty == dirty {
				rows = append(rows, renderOverviewRow(r, nameW, branchW, upstreamW))
			}
		}
		if len(rows) == 0 {
			return
		}
		sb.WriteString(reportIndent + title + "\n")
		sb.WriteString(reportIndent + dashes(width) + "\n\n")
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
		return reportIndent + emptyMessage(totalDiscovered) + "\n"
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
		sb.WriteString(reportIndent + title + "\n")
		sb.WriteString(reportIndent + dashes(width) + "\n\n")
		for _, e := range entries {
			sb.WriteString(e)
		}
		sb.WriteString("\n")
	}

	writeSection("CLEAN", false)
	writeSection("DIRTY", true)
	return sb.String()
}

// renderOverviewRow presents the Overview contract: status, name, branch,
// upstream, ahead, behind. The row is rendered at full content width (no field
// truncation); the horizontal viewport reveals any content wider than the
// terminal.
func renderOverviewRow(r Repository, nameW, branchW, upstreamW int) string {
	return fmt.Sprintf("%s[%-5s] %-*s %-*s %-*s %s",
		reportIndent, repoStatus(r),
		nameW, r.Name,
		branchW, branchLabel(r),
		upstreamW, upstreamLabel(r),
		aheadBehindLabel(r))
}

// activityEntry presents the Activity contract: name, short hash, subject,
// relative commit time. When a repository has no commits it shows "No commits".
// The subject is shown in full; the horizontal viewport handles width.
func activityEntry(r Repository) string {
	var sb strings.Builder
	sb.WriteString(reportIndent + r.Name + "\n")

	if r.LastCommitHash == "" {
		sb.WriteString(reportIndent + "  No commits\n")
		return sb.String()
	}

	sb.WriteString(reportIndent + "  " + shortHash(r.LastCommitHash) + "  " + r.LastCommitSubject + "\n")
	sb.WriteString(reportIndent + "  " + r.LastCommitTime + "\n")
	return sb.String()
}

// renderDetail renders the bounded repository detail view for one selected
// repository. It consumes only existing Repository facts.
func renderDetail(repo Repository) string {
	var sb strings.Builder
	sb.WriteString(reportIndent + "Repository: " + repo.Name + "\n")
	sb.WriteString(reportIndent + "Path:       " + repo.Path + "\n")
	status := repoStatus(repo)
	sb.WriteString(reportIndent + "Status:     " + status + "\n")
	branch := branchLabel(repo)
	sb.WriteString(reportIndent + "Branch:     " + branch + "\n")
	upstream := upstreamLabel(repo)
	sb.WriteString(reportIndent + "Upstream:   " + upstream + "\n")
	// Ahead/Behind honest handling.
	ahead, behind := "–", "–"
	if repo.Upstream != "" {
		ahead = fmt.Sprintf("%d", repo.Ahead)
		behind = fmt.Sprintf("%d", repo.Behind)
	}
	sb.WriteString(reportIndent + "Ahead:      " + ahead + "\n")
	sb.WriteString(reportIndent + "Behind:     " + behind + "\n")
	sb.WriteString(reportIndent + "Last commit:\n")
	if repo.LastCommitHash == "" {
		sb.WriteString(reportIndent + "  No commits\n")
	} else {
		sb.WriteString(reportIndent + "  " + shortHash(repo.LastCommitHash) + "  " + repo.LastCommitSubject + "\n")
		sb.WriteString(reportIndent + "  " + repo.LastCommitTime + "\n")
	}
	return sb.String()
}

// renderActivityForSelected renders Activity contextual to the selected repository.
func renderActivityForSelected(repo Repository, width int) string {
	var sb strings.Builder
	sb.WriteString(reportIndent + "Repository: " + repo.Name + "\n\n")
	if repo.LastCommitHash == "" {
		sb.WriteString(reportIndent + "  No commits\n")
		return sb.String()
	}
	sb.WriteString(reportIndent + "  " + shortHash(repo.LastCommitHash) + "  " + repo.LastCommitSubject + "\n")
	sb.WriteString(reportIndent + "  " + repo.LastCommitTime + "\n")
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
	n := width - len([]rune(reportIndent))
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

// truncate shortens s to at most max runes, appending an ellipsis when the
// content is longer. It is rune-aware so multi-byte characters are never
// split mid-codepoint. Used by the static rich report; the interactive
// investigation surface relies on the horizontal viewport instead.
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
