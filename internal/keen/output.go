package keen

import (
	"fmt"
	"strconv"
	"strings"
)

// branchLabel labels a repository's branch: a detached HEAD shows "detached"
// rather than a fabricated branch name.
func branchLabel(repo Repository) string {
	if repo.Branch == "" {
		return "detached"
	}
	return repo.Branch
}

// upstreamLabel labels a repository's upstream; an absent upstream is shown as
// an explicit em-dash rather than being folded into another column.
func upstreamLabel(repo Repository) string {
	if repo.Upstream == "" {
		return "—"
	}
	return repo.Upstream
}

// PrintRich renders a dense, columnar textual report. It presents the same
// information semantics used by the interactive views (name, branch, upstream,
// ahead/behind, short hash, subject, relative commit time) as aligned columns
// rather than prose. The layout adapts deterministically to the terminal
// width: below the point where a coherent table is impossible, it degrades
// silently to the canonical grouped report.
func PrintRich(repositories []Repository, totalDiscovered int) {
	width := terminalWidth()
	fmt.Print(renderRich(repositories, totalDiscovered, width))
}

// richColumn describes one column of the rich table: its header label, its
// rendered width, whether its values are right-aligned, and whether it may
// absorb leftover terminal width. Columns are always held in priority order
// (NAME through TIME) and width adaptation only ever removes trailing
// columns, so the number of separators between columns is always
// len(columns) - 1.
type richColumn struct {
	header string
	width  int
	right  bool
	flex   bool
}

const (
	// Fixed widths for facts that do not benefit from stretching.
	aheadColWidth  = 5
	behindColWidth = 6
	hashColWidth   = 7
	timeColWidth   = 12

	// Generous variable floors used by the wide tier.
	wideNameFloor     = 12
	wideBranchFloor   = 10
	wideUpstreamFloor = 12
	wideSubjectFloor  = 20

	// Tightened variable floors that keep all eight columns viable below the
	// wide threshold.
	tightNameFloor     = 6
	tightBranchFloor   = 4 // e.g. "main"
	tightUpstreamFloor = 5
	tightSubjectFloor  = 6

	// richWideMinWidth is the smallest CONTENT budget where every variable
	// column receives at least its generous floor, measured from this
	// renderer's own arithmetic: 30 fixed + 7 separators + 54 floors = 91.
	// Callers pass width minus the four-space report indent.
	richWideMinWidth = 91

	// richTableMinWidth is the smallest CONTENT budget where any coherent
	// rich table remains possible. Below it keen -r renders the canonical
	// grouped report instead: a successful, silent degradation — stdout stays
	// clean and stderr stays reserved for genuine diagnostics. Callers pass
	// width minus the four-space report indent.
	richTableMinWidth = 46
)

func renderRich(repositories []Repository, totalDiscovered, width int) string {
	if len(repositories) == 0 {
		return "\n" + emptyMessage(totalDiscovered) + "\n"
	}
	cols := richLayout(width - stringCellWidth(reportIndent))
	if cols == nil {
		return renderGrouped(repositories, width)
	}
	return "\n" + workspaceSummary(repositories) + "\n\n" + richTable(repositories, cols)
}

// richLayout selects the active column set for the requested width. It
// returns nil when no rich table can fit, which sends the caller to the
// canonical fallback. Thresholds emerge from the renderer's arithmetic and
// are pinned by the width-matrix tests rather than frozen arbitrarily.
func richLayout(width int) []richColumn {
	if width >= richWideMinWidth {
		cols := richWideLayout(width)
		richDistributeSlack(cols, width)
		return cols
	}
	if width < richTableMinWidth {
		return nil
	}
	cols := richTightLayout()
	for richTableWidth(cols) > width {
		next, ok := richDropExpendable(cols)
		if !ok {
			break
		}
		cols = next
	}
	if richTableWidth(cols) > width {
		return nil
	}
	richDistributeSlack(cols, width)
	return cols
}

// richWideLayout allocates variable columns proportionally for the wide tier,
// where every variable column receives at least its generous floor.
func richWideLayout(width int) []richColumn {
	variables := width - (aheadColWidth + behindColWidth + hashColWidth + timeColWidth + 7)
	name := max(wideNameFloor, variables*25/100)
	branch := max(wideBranchFloor, variables*20/100)
	upstream := max(wideUpstreamFloor, variables*25/100)
	subject := max(wideSubjectFloor, variables*30/100)
	// In the band just above the wide threshold the generous subject floor
	// exceeds its proportional share; SUBJECT is the lowest-priority
	// flexible column, so it absorbs the difference and keeps the row within
	// budget. richDistributeSlack widens columns back out to the exact
	// terminal width afterwards.
	if overflow := name + branch + upstream + subject - variables; overflow > 0 {
		subject -= overflow
	}
	return []richColumn{
		{"NAME", name, false, true},
		{"BRANCH", branch, false, true},
		{"UPSTREAM", upstream, false, true},
		{"AHEAD", aheadColWidth, true, false},
		{"BEHIND", behindColWidth, true, false},
		{"HASH", hashColWidth, false, false},
		{"SUBJECT", subject, false, true},
		{"TIME", timeColWidth, false, false},
	}
}

// richTightLayout is the full eight-column table using tightened floors; it
// is the starting point for width adaptation below the wide threshold
// (measured requirement: 51 cells + 7 separators = 58 columns).
func richTightLayout() []richColumn {
	return []richColumn{
		{"NAME", tightNameFloor, false, true},
		{"BRANCH", tightBranchFloor, false, true},
		{"UPSTREAM", tightUpstreamFloor, false, true},
		{"AHEAD", aheadColWidth, true, false},
		{"BEHIND", behindColWidth, true, false},
		{"HASH", hashColWidth, false, false},
		{"SUBJECT", tightSubjectFloor, false, true},
		{"TIME", timeColWidth, false, false},
	}
}

// richDropExpendable removes the lowest-priority column according to the
// information hierarchy: TIME first, then SUBJECT, then the AHEAD/BEHIND
// synchronization pair atomically (never half a divergence fact, and HASH is
// preserved because it outranks the pair for removal), then HASH, UPSTREAM,
// and BRANCH. NAME is never dropped. The boolean reports whether anything was
// removed.
func richDropExpendable(cols []richColumn) ([]richColumn, bool) {
	n := len(cols)
	switch {
	case n > 7: // TIME
		return cols[:n-1], true
	case n > 6: // SUBJECT
		return cols[:n-1], true
	case n > 4: // AHEAD+BEHIND atomically, splicing around HASH
		out := make([]richColumn, 0, n-2)
		out = append(out, cols[:3]...)
		out = append(out, cols[5:]...)
		return out, true
	case n > 1: // HASH, then UPSTREAM, then BRANCH
		return cols[:n-1], true
	}
	return cols, false
}

// richTableWidth computes the rendered line length of a table laid out with
// cols: one character of separation between adjacent columns. Deriving the
// gap count from the column count keeps reduced tables honest after column
// removal.
func richTableWidth(cols []richColumn) int {
	width := len(cols) - 1
	for _, c := range cols {
		width += c.width
	}
	return width
}

// richDistributeSlack widens flexible columns until the table spans exactly
// width, cycling left-to-right so the result is deterministic. Fixed-width
// facts do not stretch.
func richDistributeSlack(cols []richColumn, width int) {
	extra := width - richTableWidth(cols)
	for extra > 0 {
		progressed := false
		for i := range cols {
			if extra == 0 {
				break
			}
			if cols[i].flex {
				cols[i].width++
				extra--
				progressed = true
			}
		}
		if !progressed {
			return
		}
	}
}

// richValues returns a repository's eight display values in canonical column
// order. Positional pairing with a prefix of that order is what allows
// trailing-column removal without any lookup structure.
func richValues(r Repository) []string {
	var ahead, behind string
	if r.Upstream == "" {
		ahead, behind = "–", "–"
	} else {
		ahead, behind = strconv.Itoa(r.Ahead), strconv.Itoa(r.Behind)
	}
	hash := shortHash(r.LastCommitHash)
	if hash == "" {
		hash = "—"
	}
	subject := r.LastCommitSubject
	if r.LastCommitHash == "" {
		subject = "—"
	}
	time := r.LastCommitTime
	if time == "" {
		time = "—"
	}
	return []string{
		r.Name,
		branchLabel(r),
		upstreamLabel(r),
		ahead,
		behind,
		hash,
		subject,
		time,
	}
}

// fit truncates text to the column width and pads it to exactly that width.
func (c richColumn) fit(text string) string {
	return padCells(text, c.width, c.right)
}

func richTable(repositories []Repository, cols []richColumn) string {
	headers := make([]string, len(cols))
	for i := range cols {
		headers[i] = cols[i].fit(cols[i].header)
	}
	headerLine := strings.Join(headers, " ")
	rule := strings.Repeat("-", richTableWidth(cols))

	var sb strings.Builder
	writeGroup := func(title string, dirty bool) {
		var rows []string
		for _, r := range repositories {
			if r.Dirty == dirty {
				rows = append(rows, richRenderRow(richValues(r), cols))
			}
		}
		if len(rows) == 0 {
			return
		}
		sb.WriteString(reportIndent + title + "\n")
		sb.WriteString(reportIndent + rule + "\n")
		sb.WriteString(reportIndent + headerLine + "\n")
		for _, row := range rows {
			sb.WriteString(reportIndent + row + "\n")
		}
		sb.WriteString("\n")
	}
	writeGroup("CLEAN", false)
	writeGroup("DIRTY", true)
	return sb.String()
}

func richRenderRow(values []string, cols []richColumn) string {
	parts := make([]string, len(cols))
	for i := range cols {
		parts[i] = cols[i].fit(values[i])
	}
	return strings.Join(parts, " ")
}

func repoStatus(repository Repository) string {
	if repository.Dirty {
		return "dirty"
	}
	return "clean"
}

// branchUpstreamLabel renders the branch and, when configured, the upstream
// relationship. A detached HEAD is shown as "detached" rather than a
// fabricated branch name.
func branchUpstreamLabel(repo Repository) string {
	if repo.Upstream == "" {
		if repo.Branch == "" {
			return "detached"
		}
		return repo.Branch
	}
	if repo.Branch == "" {
		return "detached → " + repo.Upstream
	}
	return repo.Branch + " → " + repo.Upstream
}

// aheadBehindLabel renders divergence from upstream. When no upstream is
// configured, an explicit non-numeric marker is used so that "0/0" is not
// mistaken for a synchronized state.
func aheadBehindLabel(repo Repository) string {
	if repo.Upstream == "" {
		return "↑– ↓–"
	}
	return fmt.Sprintf("↑%d ↓%d", repo.Ahead, repo.Behind)
}

// commitLabel renders the latest commit as a seven-character short hash
// followed by a width-bounded subject. The subject budget never exceeds the
// canonical 40-rune cap, so unconstrained rendering stays bounded; in narrow
// terminals the subject is additionally trimmed to the remaining line budget,
// and only the bare short hash survives when even one subject rune cannot fit.
// The canonical subject on the model is never altered; only its presentation
// is truncated.
func commitLabel(repo Repository, budget int) string {
	if repo.LastCommitHash == "" {
		return ""
	}
	short := shortHash(repo.LastCommitHash)
	avail := budget - stringCellWidth(short) - 1
	if maxSubject := 40; avail > maxSubject {
		avail = maxSubject
	}
	if avail < 1 {
		return short
	}
	subject := repo.LastCommitSubject
	if stringCellWidth(subject) > avail {
		subject = truncateCells(subject, avail) + "…"
	}
	return short + " " + subject
}

// Print renders repositories according to the selected output mode. An empty
// result is reported with a minimal message that distinguishes between zero
// discovered repositories and zero filter matches.
func Print(repositories []Repository, mode OutputMode, totalDiscovered int) {
	if len(repositories) == 0 {
		fmt.Print("\n" + emptyMessage(totalDiscovered) + "\n")
		return
	}
	switch mode {
	case OutputCompact:
		printCompact(repositories)
	default:
		printGrouped(repositories)
	}
}

// repositoryRow renders one canonical row as a string. showTag prefixes the
// explicit [clean]/[dirty] marker for surfaces without a structural status
// signal (compact mode); grouped surfaces carry the status in their section
// heading instead and pass showTag=false. width is the terminal width (0 means
// unconstrained); commit identity and relative time are appended only when
// they fit, so lower-priority facts drop before the identity, branch, and
// synchronization facts become unreadable.
func repositoryRow(repository Repository, width int, showTag bool) string {
	var line strings.Builder
	if showTag {
		line.WriteString("[" + repoStatus(repository) + "] ")
	}
	fmt.Fprintf(&line, "%-15s (%-22s) %s", repository.Name, branchUpstreamLabel(repository), aheadBehindLabel(repository))

	remaining := func() int {
		if width <= 0 {
			return 1 << 30
		}
		return width - stringCellWidth(reportIndent) - stringCellWidth(line.String())
	}
	if repository.LastCommitHash != "" {
		if rem := remaining(); rem >= 13 { // " | " + short hash + space + one subject rune
			line.WriteString(" | " + commitLabel(repository, rem-3))
		}
	}
	if repository.LastCommitTime != "" {
		if rem := remaining(); rem >= 3+stringCellWidth(repository.LastCommitTime) {
			line.WriteString(" | " + repository.LastCommitTime)
		}
	}
	return line.String() + "\n"
}

// workspaceSummary renders the one-line workspace orientation placed before
// the repository report: how many repositories are shown and how many need
// attention. It summarizes the presented (post-filter) set, so the counts
// always describe exactly what follows. The section headings below remain the
// scan anchors; the summary answers "what kind of workspace is this" first.
func workspaceSummary(repositories []Repository) string {
	clean := 0
	for _, repository := range repositories {
		if !repository.Dirty {
			clean++
		}
	}
	dirty := len(repositories) - clean
	noun := "repositories"
	if len(repositories) == 1 {
		noun = "repository"
	}
	return fmt.Sprintf("%d %s, %d clean, %d dirty", len(repositories), noun, clean, dirty)
}

// renderGrouped renders the canonical grouped report as a string. Both the
// default presentation and the very-narrow rich fallback use exactly this
// path, so identical repository sets and widths always produce byte-identical
// output. The CLEAN/DIRTY section headings are the scan anchors and the single
// status signal; rows carry no per-repo status tag beneath them.
func renderGrouped(repositories []Repository, width int) string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(workspaceSummary(repositories) + "\n\n")
	hasClean := false
	hasDirty := false
	for _, repository := range repositories {
		if repository.Dirty {
			hasDirty = true
		} else {
			hasClean = true
		}
	}
	if hasClean {
		sb.WriteString(reportIndent + "CLEAN\n")
		for _, repository := range repositories {
			if !repository.Dirty {
				sb.WriteString(reportIndent + repositoryRow(repository, width, false))
			}
		}
	}
	if hasDirty {
		if hasClean {
			sb.WriteString("\n")
		}
		sb.WriteString(reportIndent + "DIRTY\n")
		for _, repository := range repositories {
			if repository.Dirty {
				sb.WriteString(reportIndent + repositoryRow(repository, width, false))
			}
		}
	}
	return sb.String()
}

func printGrouped(repositories []Repository) {
	fmt.Print(renderGrouped(repositories, terminalWidth()))
}

func printCompact(repositories []Repository) {
	fmt.Println()
	for _, repository := range repositories {
		fmt.Print(repositoryRow(repository, terminalWidth(), true))
	}
}
