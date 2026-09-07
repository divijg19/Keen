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
	keyFilter
)

// BrowsePage identifies the active surface of the interactive investigation.
type BrowsePage int

const (
	BrowseList BrowsePage = iota
	BrowseDetail
	BrowseCommitHistory
	BrowseCommitDetail
	BrowseChangedFiles
)

const browsePageCount = 5

func (p BrowsePage) label() string {
	switch p {
	case BrowseList:
		return "LIST"
	case BrowseDetail:
		return "DETAIL"
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
	case "/":
		return keyFilter
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
// the header carrying the active view label and page counter (line 0),
// followed by a blank separator (line 1). browserState.render keeps exactly
// this line exempt from horizontal slicing so the active view remains
// identifiable at every offset; only body content scrolls. No application
// banner is drawn: the header already orients the user on every repaint.
const browseHeaderLine = 0

// browseChromeRows is the fixed count of vertical chrome rows occupied by the
// header, separators, and hint on every interactive page. availableRows
// reserves this many rows before laying out selectable body content so the
// chrome never overlaps the visible selection window.
const browseChromeRows = 5

// browserState holds the concrete, local view state for one interactive
// session. It deliberately models distinct values for selection, vertical
// viewport, and horizontal viewport.
type browserState struct {
	allRepos           []Repository
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
	filtering          bool
	filterQuery        string
}

func newBrowserState(repos []Repository, total int) *browserState {
	selected := -1
	if len(repos) > 0 {
		selected = 0
	}
	return &browserState{
		allRepos:       repos,
		repos:          repos,
		total:          total,
		page:           BrowseList,
		selected:       selected,
		selectedCommit: -1,
		viewport:       terminalWidth(),
		viewportHeight: terminalHeight(),
	}
}

func (b *browserState) applyFilter() {
	if b.filterQuery == "" {
		b.repos = b.allRepos
	} else {
		q := strings.ToLower(b.filterQuery)
		var matched []Repository
		for _, r := range b.allRepos {
			if strings.Contains(strings.ToLower(r.Name), q) || strings.Contains(strings.ToLower(r.Branch), q) {
				matched = append(matched, r)
			}
		}
		b.repos = matched
	}
	b.clampSelection()
	b.ensureVisible()
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
		if w := stringCellWidth(line); w > max {
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
			// Wrap detail with the view header for consistency.
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
	selLine := b.selectedRowLine()
	for i, line := range lines {
		if i == browseHeaderLine {
			// The view indicator is navigation identity, not content: it
			// must remain visible and stable at every horizontal offset.
			// Only body lines scroll.
			out.WriteString(line)
		} else {
			sliced := sliceViewport(line, b.offset, b.viewport)
			if i == selLine && isStdoutTerminal() {
				// Reverse-video + bold is the selected-row highlight: it
				// follows the sliced viewport, so horizontal scrolling keeps
				// the visible part of the selection emphasized, and it does
				// not rely on color as the sole distinction.
				out.WriteString("\x1b[7;1m" + sliced + "\x1b[0m")
			} else {
				out.WriteString(sliced)
			}
		}
		if i < len(lines)-1 {
			out.WriteString("\n")
		}
	}
	// The screen-control clear is an interactive-presentation concern only;
	// it is emitted on a real terminal and never on redirected stdout.
	if isStdoutTerminal() {
		fmt.Print("\x1b[2J\x1b[H")
	}
	fmt.Print(out.String())
}

// selectedRowLine returns the index, within the current page's full rendered
// content, of the currently selected row, or -1 when no selectable row is on
// the page. Every list surface shares a constant two-line preamble (the view
// header on line 0 and a blank separator on line 1); the commit-history page
// additionally renders a Repository line and a blank line before its rows.
func (b *browserState) selectedRowLine() int {
	switch b.page {
	case BrowseList:
		if b.selected < 0 {
			return -1
		}
		return 2 + (b.selected - b.listOffset)
	case BrowseCommitHistory:
		if b.selectedCommit < 0 {
			return -1
		}
		return 4 + (b.selectedCommit - b.historyOffset)
	}
	return -1
}

func (b *browserState) renderHeader() string {
	return renderBannerAndHeader(b.page, b.viewport)
}

// renderListContent renders the List surface, prefixed by the shared
// active-view header.
func (b *browserState) renderListContent() string {
	var sb strings.Builder
	sb.WriteString(renderBannerAndHeader(b.page, b.viewport))

	if b.filterQuery != "" {
		sb.WriteString(reportIndent + "filter: " + b.filterQuery + "\n\n")
	}

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

	// The List is an addressable index, not a full repository report: it
	// shows identity, the single status signal, branch, and synchronization.
	// Secondary facts (upstream path, commit identity) live in Detail and
	// Activity. The selected row is marked with a leading pointer-like glyph
	// and additionally highlighted by the renderer on a terminal.
	nameW, branchW, _ := overviewWidths(b.viewport)
	for i := start; i < end; i++ {
		r := b.repos[i]
		cursor := "  "
		if i == b.selected {
			cursor = "▸ "
		}
		row := fmt.Sprintf("%s%s[%-5s] %-*s %-*s %s",
			reportIndent, cursor, repoStatus(r),
			nameW, r.Name,
			branchW, branchLabel(r),
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
		sb.WriteString(reportIndent + "No commits\n\n")
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
			cursor = "▸ "
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
			case BrowseCommitHistory:
				b.page = BrowseDetail
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
		case keyFilter:
			if b.page == BrowseList {
				b.filtering = true
				b.render()
			}
		}
		// Unknown keys (keyNone) are ignored.
	}
}

// isTerminal reports whether stdin is attached to a character device (a real
// terminal) rather than a pipe, file, or other redirected stream. It is the
// portable gate between interactive browsing and the deterministic one-shot
// overview, checked on every platform including those where makeRaw is a
// no-op. Unlike isStdoutTerminal it inspects stdin, which is the stream the
// interactive session reads from.
func isTerminal() bool {
	return isCharDevice(os.Stdin)
}

// isStdoutTerminal reports whether stdout is attached to a character device (a
// real terminal) rather than a pipe, file, or other redirected stream. Control
// sequences (screen clearing, row highlighting) are interactive-presentation
// concerns and are emitted only when stdout is a terminal, so redirected and
// piped output stays pure text.
func isStdoutTerminal() bool {
	return isCharDevice(os.Stdout)
}

// isCharDevice reports whether the given stream is attached to a character
// device (a terminal) rather than a pipe, file, or other redirected stream.
func isCharDevice(f *os.File) bool {
	info, err := f.Stat()
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
// When stdin is not a terminal, Browse degrades to a single static List render
// so the command still produces useful, portable output. Control sequences
// (screen clearing, row highlighting) are gated on stdout being a terminal, so
// redirected output never carries ANSI escapes even when stdin is a terminal.
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

// renderBannerAndHeader emits the shared screen chrome: the active-view line
// carrying the view label and page counter, followed by a blank separator. It
// is the single canonical source for this chrome so every page renders an
// identical header. No application banner is drawn; the header alone orients
// the user on every repaint.
func renderBannerAndHeader(page BrowsePage, width int) string {
	var sb strings.Builder
	title := "‹ " + page.label() + " ›"
	indicator := fmt.Sprintf("%d / %d", int(page)+1, browsePageCount)
	pad := width - stringCellWidth(title) - stringCellWidth(indicator)
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
	case BrowseDetail:
		hint = "Enter history    ←/Esc back    q quit"
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
		// The section heading carries the structural status signal, so rows
		// beneath it need no per-repo status tag (mirroring the canonical
		// grouped report). No decorative rule is drawn.
		sb.WriteString(reportIndent + title + "\n")
		for _, row := range rows {
			sb.WriteString(row + "\n")
		}
		sb.WriteString("\n")
	}

	writeSection("CLEAN", false)
	writeSection("DIRTY", true)
	return sb.String()
}

// renderOverviewRow presents the Overview contract: name, branch, upstream,
// ahead, behind. Status is conveyed structurally by the section heading, so
// the row itself carries no status tag. The row is rendered at full content
// width (no field truncation); the horizontal viewport reveals any content
// wider than the terminal.
func renderOverviewRow(r Repository, nameW, branchW, upstreamW int) string {
	return fmt.Sprintf("%s%-*s %-*s %-*s %s",
		reportIndent,
		nameW, r.Name,
		branchW, branchLabel(r),
		upstreamW, upstreamLabel(r),
		aheadBehindLabel(r))
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

func overviewWidths(width int) (int, int, int) {
	// Reserve room for the report indent, column separators, and the widest
	// synchronization label; the per-row status tag was removed in v0.8.1.
	avail := width - 15
	if avail < 15 {
		avail = 15
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
