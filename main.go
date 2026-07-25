package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

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
	cmd := exec.Command("git", "-C", repo.Path, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	repo.Dirty = len(output) > 0
	return nil
}

func printRepositories(repositories []Repository) {
	for _, repository := range repositories {
		status := "clean"
		if repository.Dirty {
			status = "dirty"
		}
		fmt.Printf("[%s] %s\n", status, repository.Path)
	}
}

type Repository struct {
	Path  string
	Dirty bool
}

func isGitRepo(path string) bool {
	gitPath := filepath.Join(path, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}
