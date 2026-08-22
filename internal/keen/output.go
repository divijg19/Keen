package keen

import (
	"fmt"
	"strconv"
	"strings"
)

// richBranch labels the branch column: a detached HEAD shows "detached" rather
// than a fabricated branch name.
func richBranch(repo Repository) string {
	if repo.Branch == "" {
		return "detached"
	}
	return repo.Branch
}

// richUpstream labels the upstream column; an absent upstream is shown as an
// explicit em-dash rather than being folded into another column.
func richUpstream(repo Repository) string {
	if repo.Upstream == "" {
		return "—"
	}
	return repo.Upstream
}

// PrintRich renders a dense, columnar textual report. It presents the same
// information semantics used by the interactive views (status, name, branch,
// upstream, ahead/behind, short hash, subject, relative commit time) as aligned
// columns rather than prose. Column widths adapt to the terminal width, but the
// columns themselves never change meaning; over-wide cells are deterministically
// truncated instead of re-flowed.
func PrintRich(repositories []Repository, totalDiscovered int) {
	width := terminalWidth()
	fmt.Print(renderRich(repositories, totalDiscovered, width))
}

func renderRich(repositories []Repository, totalDiscovered, width int) string {
	if len(repositories) == 0 {
		return "\n" + emptyMessage(totalDiscovered) + "\n"
	}

	const (
		statusW = 7
		aheadW  = 5
		behindW = 6
		hashW   = 7
		timeW   = 12
	)
	const gaps = 8
	reserved := statusW + aheadW + behindW + hashW + timeW + gaps
	avail := width - reserved
	nameMin, branchMin, upstreamMin, subjectMin := 12, 10, 12, 20
	minTotal := nameMin + branchMin + upstreamMin + subjectMin

	var nameW, branchW, upstreamW, subjectW int
	if avail < minTotal {
		nameW, branchW, upstreamW, subjectW = nameMin, branchMin, upstreamMin, subjectMin
	} else {
		nameW = max(nameMin, avail*25/100)
		branchW = max(branchMin, avail*20/100)
		upstreamW = max(upstreamMin, avail*25/100)
		subjectW = max(subjectMin, avail*30/100)
	}

	var sb strings.Builder
	sb.WriteString("\n")
	writeGroup := func(title string, dirty bool) {
		var rows []string
		for _, r := range repositories {
			if r.Dirty == dirty {
				rows = append(rows, richRow(r, statusW, nameW, branchW, upstreamW, aheadW, behindW, hashW, subjectW, timeW))
			}
		}
		if len(rows) == 0 {
			return
		}
		rule := strings.Repeat("-", statusW+nameW+branchW+upstreamW+aheadW+behindW+hashW+subjectW+timeW+gaps)
		sb.WriteString("    Git Status: " + title + "\n")
		sb.WriteString("    " + rule + "\n")
		sb.WriteString("    " + richHeader(statusW, nameW, branchW, upstreamW, aheadW, behindW, hashW, subjectW, timeW) + "\n")
		for _, row := range rows {
			sb.WriteString("    " + row + "\n")
		}
		sb.WriteString("\n")
	}
	writeGroup("CLEAN", false)
	writeGroup("DIRTY", true)
	return sb.String()
}

func richRow(r Repository, statusW, nameW, branchW, upstreamW, aheadW, behindW, hashW, subjectW, timeW int) string {
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
	subject := truncate(r.LastCommitSubject, subjectW)
	if r.LastCommitHash == "" {
		subject = "—"
	}
	time := r.LastCommitTime
	if time == "" {
		time = "—"
	}
	return fmt.Sprintf("%-*s %-*s %-*s %-*s %*s %*s %-*s %-*s %-*s",
		statusW, repoStatus(r),
		nameW, truncate(r.Name, nameW),
		branchW, truncate(richBranch(r), branchW),
		upstreamW, truncate(richUpstream(r), upstreamW),
		aheadW, ahead,
		behindW, behind,
		hashW, hash,
		subjectW, subject,
		timeW, time,
	)
}

func richHeader(statusW, nameW, branchW, upstreamW, aheadW, behindW, hashW, subjectW, timeW int) string {
	return fmt.Sprintf("%-*s %-*s %-*s %-*s %*s %*s %-*s %-*s %-*s",
		statusW, "STATUS",
		nameW, "NAME",
		branchW, "BRANCH",
		upstreamW, "UPSTREAM",
		aheadW, "AHEAD",
		behindW, "BEHIND",
		hashW, "HASH",
		subjectW, "SUBJECT",
		timeW, "TIME",
	)
}

func repoStatus(repository Repository) string {
	if repository.Dirty {
		return "dirty"
	}
	return "clean"
}

// branchLabel renders the branch and, when configured, the upstream
// relationship. A detached HEAD is shown as "detached" rather than a
// fabricated branch name.
func branchLabel(repo Repository) string {
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
	short := repo.LastCommitHash
	if len(short) > 7 {
		short = short[:7]
	}
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

func printRepositoryRow(repository Repository) {
	status := repoStatus(repository)
	fmt.Printf("[%-5s] %-15s (%-22s) %s", status, repository.Name, branchLabel(repository), aheadBehindLabel(repository))
	if cl := commitLabel(repository); cl != "" {
		fmt.Printf(" | %s", cl)
	}
	if repository.LastCommitTime != "" {
		fmt.Printf(" | %s", repository.LastCommitTime)
	}
	fmt.Println()
}

func printGrouped(repositories []Repository) {
	fmt.Println()
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
		fmt.Println("    Git Status: CLEAN")
		fmt.Println("---------------------------")
		for _, repository := range repositories {
			if !repository.Dirty {
				printRepositoryRow(repository)
			}
		}
	}
	if hasDirty {
		if hasClean {
			fmt.Println()
		}
		fmt.Println("    Git Status: DIRTY")
		fmt.Println("---------------------------")
		for _, repository := range repositories {
			if repository.Dirty {
				printRepositoryRow(repository)
			}
		}
	}
}

func printCompact(repositories []Repository) {
	fmt.Println()
	for _, repository := range repositories {
		status := repoStatus(repository)
		fmt.Printf("[%s] %s (%s) %s", status, repository.Name, branchLabel(repository), aheadBehindLabel(repository))
		if cl := commitLabel(repository); cl != "" {
			fmt.Printf(" | %s", cl)
		}
		if repository.LastCommitTime != "" {
			fmt.Printf(" | %s", repository.LastCommitTime)
		}
		fmt.Println()
	}
}
