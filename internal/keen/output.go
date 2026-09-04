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
// information semantics used by the interactive views (status, name, branch,
// upstream, ahead/behind, short hash, subject, relative commit time) as aligned
// columns rather than prose. The layout adapts deterministically to the
// terminal width: below the point where a coherent table is impossible, it
// degrades silently to the canonical grouped report.
func PrintRich(repositories []Repository, totalDiscovered int) {
	width := terminalWidth()
	fmt.Print(renderRich(repositories, totalDiscovered, width))
}

// richColumn describes one column of the rich table: its header label, its
// rendered width, whether its values are right-aligned, and whether it may
// absorb leftover terminal width. Columns are always held in priority order
// (STATUS through TIME) and width adaptation only ever removes trailing
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
	statusColWidth = 7
	aheadColWidth  = 5
	behindColWidth = 6
	hashColWidth   = 7
	timeColWidth   = 12

	// Generous variable floors used by the wide tier.
	wideNameFloor     = 12
	wideBranchFloor   = 10
	wideUpstreamFloor = 12
	wideSubjectFloor  = 20

	// Tightened variable floors that keep all nine columns viable below the
	// wide threshold.
	tightNameFloor     = 6
	tightBranchFloor   = 4 // e.g. "main"
	tightUpstreamFloor = 5
	tightSubjectFloor  = 6

	// richWideMinWidth is the smallest CONTENT budget where every variable
	// column receives at least its generous floor, measured from this
	// renderer's own arithmetic: 37 fixed + 8 separators + 54 floors = 99.
	// Callers pass width minus the four-space report indent.
	richWideMinWidth = 99

	// richTableMinWidth is the smallest CONTENT budget where any coherent
	// rich table remains possible (identity, branch, and the atomic sync
	// pair). Below it keen -r renders the canonical grouped report instead:
	// a successful, silent degradation — stdout stays clean and stderr stays
	// reserved for genuine diagnostics. Callers pass width minus the
	// four-space report indent.
	richTableMinWidth = 46
)

func renderRich(repositories []Repository, totalDiscovered, width int) string {
	if len(repositories) == 0 {
		return "\n" + emptyMessage(totalDiscovered) + "\n"
	}
	cols := richLayout(width - len([]rune(reportIndent)))
	if cols == nil {
		return renderGrouped(repositories)
	}
	return "\n" + richTable(repositories, cols)
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
	variables := width - (statusColWidth + aheadColWidth + behindColWidth + hashColWidth + timeColWidth + 8)
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
		{"STATUS", statusColWidth, false, false},
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

// richTightLayout is the full nine-column table using tightened floors; it
// is the starting point for width adaptation below the wide threshold
// (measured requirement: 58 cells + 8 separators = 66 columns).
func richTightLayout() []richColumn {
	return []richColumn{
		{"STATUS", statusColWidth, false, false},
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

// richDropExpendable removes the lowest-priority trailing column according to
// the information hierarchy: TIME first, then SUBJECT, then the AHEAD/BEHIND
// synchronization pair atomically (never half a divergence fact), then HASH,
// UPSTREAM, and BRANCH. STATUS and NAME are never dropped. The boolean
// reports whether anything was removed.
func richDropExpendable(cols []richColumn) ([]richColumn, bool) {
	n := len(cols)
	switch {
	case n > 8: // TIME
		return cols[:n-1], true
	case n > 7: // SUBJECT
		return cols[:n-1], true
	case n > 5: // AHEAD+BEHIND atomically
		return cols[:n-2], true
	case n > 3: // HASH, then UPSTREAM, then BRANCH
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

// richValues returns a repository's nine display values in canonical column
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
		repoStatus(r),
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
	text = truncate(text, c.width)
	if c.right {
		return fmt.Sprintf("%*s", c.width, text)
	}
	return fmt.Sprintf("%-*s", c.width, text)
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
		sb.WriteString(reportIndent + "Git Status: " + title + "\n")
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
// followed by a width-bounded subject. The canonical subject on the model is
// never altered; only its presentation is truncated.
func commitLabel(repo Repository) string {
	if repo.LastCommitHash == "" {
		return ""
	}
	short := shortHash(repo.LastCommitHash)
	subject := repo.LastCommitSubject
	const maxSubject = 40
	if len([]rune(subject)) > maxSubject {
		subject = string([]rune(subject)[:maxSubject]) + "…"
	}
	return short + " " + subject
}

// Print renders repositories according to the selected output mode. An empty
// result is reported with a minimal message that distinguishes between zero
// discovered repositories and zero filter matches.
func Print(repositories []Repository, mode OutputMode, totalDiscovered int) {
	if len(repositories) == 0 {
		fmt.Println()
		if totalDiscovered == 0 {
			fmt.Println("No repositories found.")
		} else {
			fmt.Println("No repositories match the selected filters.")
		}
		return
	}
	switch mode {
	case OutputCompact:
		printCompact(repositories)
	default:
		printGrouped(repositories)
	}
}

// repositoryRow renders one canonical row as a string.
func repositoryRow(repository Repository) string {
	status := repoStatus(repository)
	row := fmt.Sprintf("[%-5s] %-15s (%-22s) %s", status, repository.Name, branchUpstreamLabel(repository), aheadBehindLabel(repository))
	if cl := commitLabel(repository); cl != "" {
		row += " | " + cl
	}
	if repository.LastCommitTime != "" {
		row += " | " + repository.LastCommitTime
	}
	return row + "\n"
}

// renderGrouped renders the canonical grouped report as a string. Both the
// default presentation and the very-narrow rich fallback use exactly this
// path, so identical repository sets always produce byte-identical output.
func renderGrouped(repositories []Repository) string {
	var sb strings.Builder
	sb.WriteString("\n")
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
		sb.WriteString("    Git Status: CLEAN\n")
		sb.WriteString("---------------------------\n")
		for _, repository := range repositories {
			if !repository.Dirty {
				sb.WriteString(repositoryRow(repository))
			}
		}
	}
	if hasDirty {
		if hasClean {
			sb.WriteString("\n")
		}
		sb.WriteString("    Git Status: DIRTY\n")
		sb.WriteString("---------------------------\n")
		for _, repository := range repositories {
			if repository.Dirty {
				sb.WriteString(repositoryRow(repository))
			}
		}
	}
	return sb.String()
}

func printGrouped(repositories []Repository) {
	fmt.Print(renderGrouped(repositories))
}

func printCompact(repositories []Repository) {
	fmt.Println()
	for _, repository := range repositories {
		status := repoStatus(repository)
		fmt.Printf("[%s] %s (%s) %s", status, repository.Name, branchUpstreamLabel(repository), aheadBehindLabel(repository))
		if cl := commitLabel(repository); cl != "" {
			fmt.Printf(" | %s", cl)
		}
		if repository.LastCommitTime != "" {
			fmt.Printf(" | %s", repository.LastCommitTime)
		}
		fmt.Println()
	}
}
