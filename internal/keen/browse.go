package keen

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
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
	keyBackspace
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

func (p BrowsePage) label() string {
	switch p {
	case BrowseList:
		return "REPOSITORIES"
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

// browsePageCount is the number of interactive surfaces in the investigation
// hierarchy (List → Detail → History → Commit → Files). The header shows the
// current surface's position ("n / 5") right-aligned on the breadcrumb line.
const browsePageCount = 5

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
	case "\x7f", "\x08": // Backspace: DEL and BS control forms
		return keyBackspace
	}
	return keyNone
}

// isFilterChar reports whether raw is a single printable rune suitable for
// appending to the interactive filter query. Control sequences, escape
// sequences, and multi-rune inputs are never query text.
func isFilterChar(raw string) bool {
	if !utf8.ValidString(raw) {
		return false
	}
	r := []rune(raw)
	if len(r) != 1 {
		return false
	}
	return unicode.IsPrint(r[0])
}

// appendFilterChar appends one printable character to the filter query.
func (b *browserState) appendFilterChar(raw string) {
	b.filterQuery += raw
	b.applyFilter()
}

// backspaceFilter removes the final rune from the filter query safely,
// handling multi-byte characters without splitting them.
func (b *browserState) backspaceFilter() {
	if b.filterQuery == "" {
		return
	}
	r := []rune(b.filterQuery)
	b.filterQuery = string(r[:len(r)-1])
	b.applyFilter()
}

// exitFilterEditing leaves filter editing mode. When keep is true the current
// query stays active; otherwise the query is cleared and the full set
// restored. Selection and viewport invariants are preserved via applyFilter.
func (b *browserState) exitFilterEditing(keep bool) {
	if !keep {
		b.filterQuery = ""
		b.applyFilter()
	}
	b.filtering = false
}

// browseHeaderLine is the index of the view-indicator header within the screen
// produced by renderBrowse. The layout contract is fixed: renderBrowse emits
// the breadcrumb header carrying the view hierarchy (line 0), followed by a
// blank separator (line 1). browserState.render keeps exactly this line
// exempt from horizontal slicing so the active view remains identifiable at
// every offset; only body content scrolls. No application banner is drawn:
// the header alone orients the user on every repaint.
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
	allRepos        []Repository
	repos           []Repository
	total           int
	page            BrowsePage
	selected        int // index of selected repo; -1 if none
	listOffset      int // vertical viewport offset
	offset          int // horizontal viewport offset
	viewport        int // terminal width
	viewportHeight  int // terminal height
	history         []Commit
	selectedCommit  int // index of selected commit; -1 if none
	historyOffset   int // vertical viewport offset for commit history
	changedFiles    []ChangedFile
	detailOffset    int // vertical viewport offset for Detail/Commit/Files content
	historyErr      string
	changedFilesErr string
	filtering       bool
	filterQuery     string
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
		// Identities are re-resolved against the visible subset, mirroring
		// the pipeline's Discover → … → Filter → ResolveDisplayIdentities
		// order: when filtering removes one member of a collision group,
		// the survivor sheds its parent qualification instead of showing a
		// stale one. The display identity is derived, never stored, so
		// re-resolution restarts from basenames (Enrich sets Name to the
		// path basename) exactly as the pipeline feeds it; entries without
		// a path keep their existing name since no basename is derivable.
		// Clearing the query restores the full-set identities.
		subset := make([]Repository, len(matched))
		for i, r := range matched {
			if r.Path != "" {
				r.Name = filepath.Base(r.Path)
			}
			subset[i] = r
		}
		b.repos = ResolveDisplayIdentities(subset)
	}
	b.clampSelection()
	b.ensureVisible()
}

// clampIndex keeps a selection address valid for a list of n items: an
// empty list selects nothing (-1), otherwise the index is pinned into
// [0, n). It is the single rule behind repository and commit selection.
func clampIndex(selected, n int) int {
	if n == 0 {
		return -1
	}
	if selected < 0 {
		return 0
	}
	if selected >= n {
		return n - 1
	}
	return selected
}

// windowOffset keeps offset a valid window origin showing selected within a
// rows-tall window over n items: the window follows the selection first,
// then the origin is pinned into [0, max(0, n-rows)]. It is the single rule
// behind list and history viewport tracking.
func windowOffset(selected, offset, n, rows int) int {
	if rows <= 0 {
		rows = 1
	}
	if selected < offset {
		offset = selected
	}
	if selected >= offset+rows {
		offset = selected - rows + 1
	}
	maxOffset := n - rows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	return offset
}

func (b *browserState) clampSelection() {
	if len(b.repos) == 0 {
		b.selected = -1
		b.listOffset = 0
		return
	}
	b.selected = clampIndex(b.selected, len(b.repos))
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
	b.listOffset = windowOffset(b.selected, b.listOffset, len(b.repos), b.listBodyRows())
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

// bodyRows reserves the shared chrome budget plus extra fixed preamble rows
// (page lines beyond the header and footer) from the terminal height,
// floored so at least one body row always renders. The single floor keeps
// every surface's window consistent.
func (b *browserState) bodyRows(extraChrome int) int {
	rows := b.availableRows() - extraChrome
	if rows < 1 {
		rows = 1
	}
	return rows
}

// listBodyRows is the selectable-row budget for the List surface. When the
// filter query line is displayed it consumes two fixed rows ("filter:
// <query>" plus its blank separator) beyond the shared chrome budget, so the
// window shrinks accordingly; otherwise the full shared budget applies.
// Keeping this budget identical between ensureVisible and renderListContent
// guarantees the selected row is always inside the rendered window.
func (b *browserState) listBodyRows() int {
	extra := 0
	if b.filterQuery != "" {
		extra = 2
	}
	return b.bodyRows(extra)
}

// historyBodyRows is the selectable-row budget for the History surface. The
// "Repository:" line and its blank separator consume two fixed rows beyond
// the shared chrome budget. Shared with ensureCommitVisible and
// renderCommitHistoryContent so the selected commit is always rendered.
func (b *browserState) historyBodyRows() int {
	return b.bodyRows(2)
}

func (b *browserState) clampCommitSelection() {
	if len(b.history) == 0 {
		b.selectedCommit = -1
		b.historyOffset = 0
		return
	}
	b.selectedCommit = clampIndex(b.selectedCommit, len(b.history))
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
	b.historyOffset = windowOffset(b.selectedCommit, b.historyOffset, len(b.history), b.historyBodyRows())
}

func (b *browserState) selectedCommitObj() *Commit {
	if b.selectedCommit < 0 || b.selectedCommit >= len(b.history) {
		return nil
	}
	return &b.history[b.selectedCommit]
}

// windowLines returns at most maxRows lines starting at offset, with offset
// clamped into range. It is the pure primitive behind bounded detail content:
// long Detail/Commit/Files bodies are windowed so the footer remains visible
// instead of being pushed below the terminal.
func windowLines(lines []string, offset, maxRows int) ([]string, int) {
	if maxRows < 1 {
		maxRows = 1
	}
	if offset < 0 {
		offset = 0
	}
	maxOffset := len(lines) - maxRows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	end := offset + maxRows
	if end > len(lines) {
		end = len(lines)
	}
	return lines[offset:end], offset
}

// splitContentLines splits rendered content into lines, dropping the single
// trailing empty element produced by a trailing newline so windowing counts
// real content rows.
func splitContentLines(content string) []string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// boundedDetailContent windows detail-page content to the available rows,
// clamping and persisting the shared detailOffset so Detail, Commit Detail,
// and Changed Files all keep the footer visible.
func (b *browserState) boundedDetailContent(content string) string {
	lines := splitContentLines(content)
	windowed, offset := windowLines(lines, b.detailOffset, b.availableRows())
	b.detailOffset = offset
	if len(windowed) == 0 {
		return ""
	}
	return strings.Join(windowed, "\n") + "\n"
}

// scrollDetail moves the shared detail viewport by delta rows, clamped on
// the next render against the actual content height.
func (b *browserState) scrollDetail(delta int) {
	b.detailOffset += delta
	if b.detailOffset < 0 {
		b.detailOffset = 0
	}
}

// clearChangedFiles drops the per-commit file state; called whenever the
// selected repository or commit changes.
func (b *browserState) clearChangedFiles() {
	b.changedFiles = nil
	b.changedFilesErr = ""
}

func (b *browserState) loadHistoryForSelected() {
	b.history = nil
	b.selectedCommit = -1
	b.historyOffset = 0
	b.historyErr = ""
	b.clearChangedFiles()
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
	b.clearChangedFiles()
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
			full = b.renderDetailPage(renderDetail(*repo))
		} else {
			full = renderBrowse(b.repos, b.total, b.page, b.viewport)
		}
	case BrowseCommitHistory:
		full = b.renderCommitHistoryContent()
	case BrowseCommitDetail:
		if c := b.selectedCommitObj(); c != nil {
			if repo := b.selectedRepo(); repo != nil {
				full = b.renderDetailPage(renderCommitDetail(*repo, *c))
			} else {
				full = renderBrowse(b.repos, b.total, b.page, b.viewport)
			}
		} else {
			full = renderBrowse(b.repos, b.total, b.page, b.viewport)
		}
	case BrowseChangedFiles:
		if c := b.selectedCommitObj(); c != nil {
			if repo := b.selectedRepo(); repo != nil {
				full = b.renderDetailPage(renderChangedFilesForCommit(*repo, *c, b.changedFiles, b.changedFilesErr))
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
	// The footer is always the final content line: every render path ends
	// with the keymap footer's trailing newline (renderFooter, or the
	// filter-editing footer on a filtering List), so the hint sits at
	// len-2 with a single empty element after it. Like the header, the
	// footer is chrome, not content: it is never horizontally scrolled and
	// never carries the selection highlight.
	footerLine := -1
	if len(lines) >= 2 {
		footerLine = len(lines) - 2
	}
	var out strings.Builder
	selLine := b.selectedRowLine()
	for i, line := range lines {
		if i == browseHeaderLine || i == footerLine {
			// The view indicator and the keymap footer are navigation
			// identity, not content: they remain visible and stable at
			// every horizontal offset. Only body lines scroll.
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
// header on line 0 and a blank separator on line 1); the List page inserts
// the two-line filter readout above its rows while a query is displayed, and
// the commit-history page renders a Repository line and a blank line before
// its rows.
func (b *browserState) selectedRowLine() int {
	switch b.page {
	case BrowseList:
		if b.selected < 0 {
			return -1
		}
		base := 2
		if b.filterQuery != "" {
			base = 4
		}
		return base + (b.selected - b.listOffset)
	case BrowseCommitHistory:
		if b.selectedCommit < 0 {
			return -1
		}
		return 4 + (b.selectedCommit - b.historyOffset)
	}
	return -1
}

func (b *browserState) renderHeader() string {
	repoName := ""
	if r := b.selectedRepo(); r != nil {
		repoName = r.Name
	}
	return renderBannerAndHeader(b.page, repoName, b.viewport)
}

// renderDetailPage assembles a detail-family page (Detail, Commit Detail,
// Changed Files): shared header, viewport-bounded body, and footer. The body
// is windowed so long content never displaces the footer.
func (b *browserState) renderDetailPage(body string) string {
	return b.renderHeader() + b.boundedDetailContent(body) + "\n" + renderFooter(b.page)
}

// transition moves to the given page, resetting both viewports so every
// surface starts at its origin, and repaints. All forward and back
// navigation shares this reset; page-specific loading happens before it.
func (b *browserState) transition(page BrowsePage) {
	b.page = page
	b.offset = 0
	b.detailOffset = 0
	b.render()
}

// renderListContent renders the List surface, prefixed by the shared
// active-view header.
func (b *browserState) renderListContent() string {
	var sb strings.Builder
	sb.WriteString(b.renderHeader())

	if b.filterQuery != "" {
		sb.WriteString(reportIndent + "filter: " + b.filterQuery + "\n\n")
	}

	if len(b.repos) == 0 {
		sb.WriteString(reportIndent + emptyMessage(b.total) + "\n\n")
		sb.WriteString(b.listFooter())
		return sb.String()
	}

	// Ensure selected visible before rendering.
	b.ensureVisible()
	availableRows := b.listBodyRows()
	start := b.listOffset
	end := start + availableRows
	if end > len(b.repos) {
		end = len(b.repos)
	}

	// The List is an addressable index, not a full repository report: it
	// shows identity, the single status signal, branch, and synchronization.
	// Secondary facts (upstream path, commit identity) live in Detail and
	// the commit surfaces. The selected row is identified solely by the reverse-video
	// highlight applied by the renderer on a terminal; no pointer glyph
	// competes with it, keeping the row visually quiet.
	nameW, branchW, _ := overviewWidths(b.viewport)
	for i := start; i < end; i++ {
		r := b.repos[i]
		row := reportIndent + "  [" + padStartCells(repoStatus(r), 5) + "] " +
			padStartCells(r.Name, nameW) + " " +
			padStartCells(branchLabel(r), branchW) + " " +
			aheadBehindLabel(r)
		sb.WriteString(row + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString(b.listFooter())
	return sb.String()
}

// listFooter selects the List keymap footer for the current input state.
// While filter editing is active the normal List footer would lie — ↑/↓ do
// not move, "/" types, "q" is query text, Enter does not inspect — so the
// footer swaps to the editing keymap describing exactly what each key does,
// including the Ctrl+C quit path. Every other state keeps renderFooter.
func (b *browserState) listFooter() string {
	if b.filtering {
		return renderFilterFooter()
	}
	return renderFooter(b.page)
}

// renderFilterFooter is the keymap footer shown while filter editing is
// active. Typing is the mode's primary action so it leads; the remaining
// entries follow the canonical page-footer order — Enter accepts, Backspace
// deletes rune-safely (including "q", "/", "H", "L" as typed text), Esc
// clears, and Ctrl+C quits from anywhere, even mid-query, with quit last.
func renderFilterFooter() string {
	return reportIndent + "type to filter    Enter accept    Backspace delete    Esc clear    Ctrl+C quit\n"
}

func (b *browserState) renderCommitHistoryContent() string {
	var sb strings.Builder
	sb.WriteString(b.renderHeader())

	repo := b.selectedRepo()
	if repo == nil {
		sb.WriteString(reportIndent + "No repository selected.\n\n")
		sb.WriteString(renderFooter(b.page))
		return sb.String()
	}
	sb.WriteString(reportIndent + "Repository: " + repo.Name + "\n\n")
	if b.historyErr != "" {
		sb.WriteString(reportIndent + "Failed to load history: " + b.historyErr + "\n\n")
		sb.WriteString(renderFooter(b.page))
		return sb.String()
	}
	if len(b.history) == 0 {
		sb.WriteString(reportIndent + "No commits\n\n")
		sb.WriteString(renderFooter(b.page))
		return sb.String()
	}
	b.ensureCommitVisible()
	availableRows := b.historyBodyRows()
	start := b.historyOffset
	end := start + availableRows
	if end > len(b.history) {
		end = len(b.history)
	}
	// The selected commit is identified solely by the reverse-video highlight
	// applied by the renderer; no pointer glyph competes with it.
	for i := start; i < end; i++ {
		c := b.history[i]
		row := fmt.Sprintf("%s  %s   %s  %s", reportIndent, shortHash(c.Hash), c.Subject, c.AuthorDate)
		// Simplified row; horizontal viewport reveals overflow.
		sb.WriteString(row + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString(renderFooter(b.page))
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
// without a terminal; production Browse passes readKey. The raw sequence
// accompanies the normalized action so filter editing can consume printable
// characters that interpretSequence otherwise reports as keyNone.
func (b *browserState) run(read func() (keyAction, string, bool)) {
	for {
		action, raw, ok := read()
		if !ok {
			// The stream ended or failed permanently (EOF, closed or hung-up
			// tty, I/O error). Quit cleanly so the caller's deferred
			// restoreRaw executes; never busy-loop on a dead stream.
			return
		}
		if b.filtering && b.page == BrowseList {
			// Filter editing is a genuine text-entry state: printable input
			// extends the query, including characters that are bound to
			// actions outside editing ("q", "/", "H", "L"). Structural keys
			// keep their meaning — Backspace edits, Enter accepts, Esc
			// clears — and navigation/scrolling keys are ignored so
			// selection cannot move accidentally. Ctrl+C still quits: it is
			// not printable, so it never becomes query text.
			if isFilterChar(raw) {
				b.appendFilterChar(raw)
				b.render()
				continue
			}
			switch action {
			case keyQuit:
				return
			case keyEsc:
				b.exitFilterEditing(false)
				b.render()
			case keyEnter:
				b.exitFilterEditing(true)
				b.render()
			case keyBackspace:
				b.backspaceFilter()
				b.render()
			}
			continue
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
			case BrowseDetail, BrowseCommitDetail, BrowseChangedFiles:
				b.scrollDetail(-1)
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
			case BrowseDetail, BrowseCommitDetail, BrowseChangedFiles:
				b.scrollDetail(1)
				b.render()
			}
		case keyEnter:
			switch b.page {
			case BrowseList:
				if b.selected >= 0 {
					b.transition(BrowseDetail)
				}
			case BrowseDetail:
				b.loadHistoryForSelected()
				b.transition(BrowseCommitHistory)
			case BrowseCommitHistory:
				if b.selectedCommit >= 0 && b.selectedCommit < len(b.history) {
					b.transition(BrowseCommitDetail)
				}
			case BrowseCommitDetail:
				b.loadChangedFilesForSelectedCommit()
				b.transition(BrowseChangedFiles)
			}
		case keyLeft, keyEsc:
			switch b.page {
			case BrowseDetail:
				b.transition(BrowseList)
			case BrowseCommitHistory:
				b.transition(BrowseDetail)
			case BrowseCommitDetail:
				b.transition(BrowseCommitHistory)
			case BrowseChangedFiles:
				b.transition(BrowseCommitDetail)
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
// `keen -i`. The hierarchy is List → Detail → History → Commit → Files:
// Enter advances to a child surface, ←/Esc return to the parent, and
// q/Ctrl+C quit from any level. List is an addressable index; Detail shows
// one repository's full state; History, Commit, and Files investigate the
// selected repository's recent commits.
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

	// Minimal signal protection for raw terminal: restore on TERM/HUP/INT/QUIT.
	// Keyboard Ctrl-C never arrives as SIGINT (makeRaw clears ISIG, so it
	// reads as a 0x03 byte handled by the event loop); these handlers cover
	// only externally delivered signals that would otherwise leave raw mode.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT)
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

// sliceViewport returns the terminal-cell window [offset, offset+width) of s.
// Offsets and widths are display cells, matching contentWidth and the
// terminal width, so CJK and other wide content scrolls coherently instead of
// drifting. Wide characters straddling a window edge are skipped rather than
// split; zero-width characters ride with their cell position. ASCII content
// behaves exactly as a rune window.
func sliceViewport(s string, offset, width int) string {
	if width <= 0 {
		return ""
	}
	if offset < 0 {
		offset = 0
	}
	var sb strings.Builder
	cur := 0 // cells consumed so far
	for _, r := range s {
		rw := runeCellWidth(r)
		if rw <= 0 {
			if cur >= offset && cur < offset+width {
				sb.WriteRune(r)
			}
			continue
		}
		if cur+rw <= offset {
			cur += rw
			continue
		}
		if cur >= offset+width {
			break
		}
		if cur < offset || cur+rw > offset+width {
			cur += rw
			continue
		}
		sb.WriteRune(r)
		cur += rw
	}
	return sb.String()
}

// renderBannerAndHeader emits the shared screen chrome: the breadcrumb line
// carrying the view hierarchy with the current surface's position ("n / 5")
// right-aligned, followed by a blank separator. It is the single canonical
// source for this chrome so every page renders an identical header. The
// title keeps its cell-aware truncation budget minus the indicator width, so
// the indicator never wraps the header onto a second line; over-wide titles
// truncate at a cell boundary instead of colliding. Repository identity in
// the header is orientation, not the canonical record: the full path remains
// on Detail.
func renderBannerAndHeader(page BrowsePage, repoName string, width int) string {
	var sb strings.Builder
	var title string
	if repoName == "" || page == BrowseList {
		title = "KEEN › " + page.label()
	} else {
		title = fmt.Sprintf("KEEN › %s › %s", repoName, page.label())
	}
	indicator := fmt.Sprintf("%d / %d", int(page)+1, browsePageCount)
	if width > 0 {
		budget := width - stringCellWidth(indicator) - 1
		if budget < 1 {
			budget = 1
		}
		if stringCellWidth(title) > budget {
			title = truncateCells(title, budget)
		}
	}
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

// renderFooter is the single source for the contextual keymap footer. Each
// page advertises exactly the actions its event handling supports, using the
// target-surface name for Enter ("Enter detail/commit/files/history") so the
// footer always says what the key will do. Bindings follow one canonical
// order — Enter, ↑/↓, /, ←/Esc, quit — and every page's footer is a
// subsequence of it: Enter leads wherever it acts, quit always closes, and a
// missing entry (e.g. no Enter on Files) means the action is a no-op there
// rather than a reordering.
//
//   - ↑/↓ appear wherever they act: selection on List/History, scrolling on
//     Detail/Commit/Files.
//   - H/L horizontal scrolling is intentionally undisclosed to keep the footer
//     within a narrow-terminal width budget; it is documented in
//     docs/interactive.md instead of consuming permanent footer space.
//
// Every render path ends with this footer, so the interactive renderer can
// rely on it occupying the final content line.
func renderFooter(page BrowsePage) string {
	var hint string
	switch page {
	case BrowseList:
		hint = "Enter detail    ↑/↓ select    / filter    q quit"
	case BrowseDetail:
		hint = "Enter history    ↑/↓ scroll    ←/Esc back    q quit"
	case BrowseCommitHistory:
		hint = "Enter commit    ↑/↓ select    ←/Esc back    q quit"
	case BrowseCommitDetail:
		hint = "Enter files    ↑/↓ scroll    ←/Esc back    q quit"
	case BrowseChangedFiles:
		hint = "↑/↓ scroll    ←/Esc back    q quit"
	}
	return reportIndent + hint + "\n"
}

// renderBrowse assembles the full (untruncated) screen for a page. Width
// controls only the header padding and the decorative rules; body lines are
// rendered at full content width and revealed through the horizontal viewport
// by the browserState renderer. This keeps the renderers independent of
// terminal state and directly assertable in tests.
func renderBrowse(repos []Repository, totalDiscovered int, page BrowsePage, width int) string {
	var sb strings.Builder
	repoName := ""
	if len(repos) > 0 && page != BrowseList {
		repoName = repos[0].Name
	}
	sb.WriteString(renderBannerAndHeader(page, repoName, width))

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

	sb.WriteString(renderFooter(page))
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
	return reportIndent +
		padStartCells(r.Name, nameW) + " " +
		padStartCells(branchLabel(r), branchW) + " " +
		padStartCells(upstreamLabel(r), upstreamW) + " " +
		aheadBehindLabel(r)
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
	// Ahead/Behind honest handling: unavailable without an upstream.
	ahead, behind := noSyncMark, noSyncMark
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
