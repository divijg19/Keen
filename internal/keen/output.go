package keen

import "fmt"

func repoStatus(repository Repository) string {
	if repository.Dirty {
		return "dirty"
	}
	return "clean"
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
	fmt.Printf("[%-5s] %-15s (%-12s) ↑%-2d ↓%-2d", status, repository.Name, repository.Branch, repository.Ahead, repository.Behind)
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
		fmt.Printf("[%s] %s (%s) ↑%d ↓%d", status, repository.Name, repository.Branch, repository.Ahead, repository.Behind)
		if repository.LastCommitTime != "" {
			fmt.Printf(" | %s", repository.LastCommitTime)
		}
		fmt.Println()
	}
}
