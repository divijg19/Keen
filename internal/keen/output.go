package keen

import "fmt"

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
