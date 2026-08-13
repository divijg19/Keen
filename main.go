package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/charlievieth/fastwalk"
)

func main() {
	workingDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Invocation or runtime failed: %v\n", err)
		return
	}
	repositories, err := discoverRepositories(workingDir)
	if err != nil {
		fmt.Printf("Filesystem traversal failed: %v\n", err)
	}
	var showClean, showDirty bool
	var compact bool
	flag.BoolVar(&showClean, "clean", false, "show only clean repositories")
	flag.BoolVar(&showDirty, "dirty", false, "show only dirty repositories")
	flag.BoolVar(&compact, "compact", false, "use compact output")
	flag.Parse()
	fmt.Println("===KEEN===")
	for i := range repositories {
		err := enrichRepository(&repositories[i])
		if err != nil {
			fmt.Printf("Failed to inspect %s: %v\n", repositories[i].Path, err)
		}
	}
	slices.SortStableFunc(repositories, func(a, b Repository) int {
		if !a.Dirty && b.Dirty {
			return -1
		}
		if a.Dirty && !b.Dirty {
			return 1
		}
		return 0
	})

	var filtered []Repository
	for _, r := range repositories {
		if showClean && !r.Dirty {
			filtered = append(filtered, r)
		}
		if showDirty && r.Dirty {
			filtered = append(filtered, r)
		}
	}
	if !showClean && !showDirty {
		filtered = repositories
	}
	mode := OutputGrouped
	if compact {
		mode = OutputCompact
	}
	printRepositories(filtered, mode)
}

func traversalEntry(path string, d fs.DirEntry, err error) (*Repository, error) {
	if err != nil {
		fmt.Printf("Skipping: %s | %v\n", path, err)
		return nil, nil
	}
	if !d.IsDir() {
		return nil, nil
	}
	if !isGitRepo(path) {
		return nil, nil
	}
	return &Repository{
		Path: path,
	}, nil
}

func discoverRepositories(root string) ([]Repository, error) {
	var repositories []Repository
	conf := &fastwalk.Config{}
	err := fastwalk.Walk(conf, root, func(path string, d fs.DirEntry, err error) error {
		repo, err := traversalEntry(path, d, err)
		if err != nil {
			return err
		}
		if repo != nil {
			repositories = append(repositories, *repo)
			return filepath.SkipDir
		}
		return nil
	})
	return repositories, err
}

func enrichRepository(repo *Repository) error {
	repo.Name = filepath.Base(repo.Path)
	cmd := exec.Command("git", "-C", repo.Path, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	repo.Dirty = len(output) > 0

	cmd = exec.Command(
		"git",
		"-C",
		repo.Path,
		"branch",
		"--show-current",
	)
	output, err = cmd.Output()
	if err != nil {
		return err
	}
	repo.Branch = strings.TrimSpace(string(output))

	cmd = exec.Command(
		"git",
		"-C",
		repo.Path,
		"rev-list",
		"--left-right",
		"--count",
		"HEAD...@{upstream}",
	)
	output, err = cmd.Output()
	if err != nil {
		// Repository may not have an upstream configured.
		repo.Ahead = 0
		repo.Behind = 0
	} else {
		fields := strings.Fields(string(output))
		if len(fields) == 2 {
			repo.Ahead, _ = strconv.Atoi(fields[0])
			repo.Behind, _ = strconv.Atoi(fields[1])
		}
	}

	cmd = exec.Command("git", "-C", repo.Path, "log", "-1", "--date=relative", "--pretty=%cd")
	output, err = cmd.Output()
	if err != nil {
		repo.LastCommitTime = "No commits"
		return nil
	}
	repo.LastCommitTime = strings.TrimSpace(string(output))

	return nil
}

func printRepositories(repositories []Repository, mode OutputMode) {
	if len(repositories) == 0 {
		fmt.Println()
		fmt.Println("No repositories found.")
		return
	}

	switch mode {
	case OutputCompact:
		printCompact(repositories)
	default:
		printGrouped(repositories)
	}
}

func repoStatus(repository Repository) string {
	if repository.Dirty {
		return "dirty"
	}
	return "clean"
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

type Repository struct {
	Path           string
	Name           string
	Branch         string
	Dirty          bool
	Ahead          int
	Behind         int
	LastCommitTime string
}

type OutputMode int

const (
	OutputGrouped OutputMode = iota
	OutputCompact
)

func isGitRepo(path string) bool {
	gitPath := filepath.Join(path, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}
