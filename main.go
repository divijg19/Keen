package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charlievieth/fastwalk"
)

func main() {
	fmt.Println("===KEEN===")
	workingDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Invocation or runtime failed: %v\n", err)
		return
	}
	repositories, err := discoverRepositories(workingDir)
	if err != nil {
		fmt.Printf("Filesystem traversal failed: %v\n", err)
	}
	for i := range repositories {
		err := enrichRepository(&repositories[i])
		if err != nil {
			fmt.Printf("Failed to inspect %s: %v\n", repositories[i].Path, err)
		}
	}
	printRepositories(repositories)
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

func printRepositories(repositories []Repository) {
	for _, repository := range repositories {
		status := "clean"
		if repository.Dirty {
			status = "dirty"
		}
		fmt.Printf("[%-5s] %-15s (%-12s) ↑%-2d ↓%-2d", status, repository.Name, repository.Branch, repository.Ahead, repository.Behind)
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

func isGitRepo(path string) bool {
	gitPath := filepath.Join(path, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}
