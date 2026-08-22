package keen

import (
	"fmt"
	"os"
	"strings"
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
)

// BrowsePage identifies one horizontal surface of the browser. The v0.5.6
// browser has exactly two pages; this is a closed enumeration, not an
// extensible pagination mechanism.
type BrowsePage int

const (
	BrowseOverview BrowsePage = iota
	BrowseActivity
)

const browsePageCount = 2

func (p BrowsePage) label() string {
	switch p {
	case BrowseOverview:
		return "OVERVIEW"
	case BrowseActivity:
		return "ACTIVITY"
	}
	return "?"
}

// cyclePage moves between surfaces with wraparound. Direction is -1 (left) or
// +1 (right); any other value is treated as +1.
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
// accepts literal key characters (h/l/q/H/L/Ctrl+C) and ANSI escape sequences
// (including modified arrows) so the function stays pure and unit-testable
// independent of terminal state.
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
	case "\x1b[A", "\x1b[B":
		return keyNone
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
// session. It deliberately models the horizontal viewport as distinct values:
// the active page (view), the full content width of that view, the terminal
// width (viewport), and the horizontal offset into the content.
type browserState struct {
	repos    []Repository
	total    int
	page     BrowsePage
	offset   int
	viewport int
}

func (b *browserState) contentWidth() int {
	full := renderBrowse(b.repos, b.total, b.page, b.viewport)
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
	full := renderBrowse(b.repos, b.total, b.page, b.viewport)
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
		case keyLeft:
			b.page = cyclePage(b.page, -1)
			b.render()
		case keyRight:
			b.page = cyclePage(b.page, 1)
			b.render()
		case keyScrollLeft:
			b.scroll(-max(1, b.viewport/2))
			b.render()
		case keyScrollRight:
			b.scroll(max(1, b.viewport/2))
			b.render()
		case keyQuit, keyEsc:
			return
		}
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
		fmt.Print(renderBrowse(repos, totalDiscovered, BrowseOverview, width))
		return
	}
	defer restoreRaw(fd)

	state := &browserState{
		repos:    repos,
		total:    totalDiscovered,
		page:     BrowseOverview,
		offset:   0,
		viewport: width,
	}
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
	case BrowseOverview:
		sb.WriteString(renderOverview(repos, totalDiscovered, width))
	case BrowseActivity:
		sb.WriteString(renderActivity(repos, totalDiscovered, width))
	}

	sb.WriteString("\n")
	sb.WriteString(browseIndent + "←/→ views    Shift+←/→ scroll    q quit\n")
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
