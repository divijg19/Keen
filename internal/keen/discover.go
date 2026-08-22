package keen

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/charlievieth/fastwalk"
)

// isGitRepo reports whether path contains a .git directory.
func isGitRepo(path string) bool {
	gitPath := filepath.Join(path, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func traversalEntry(path string, d fs.DirEntry, err error) (*Repository, error) {
	if err != nil {
		// Diagnostics belong on stderr; stdout is reserved for product output.
		fmt.Fprintf(os.Stderr, "Skipping: %s | %v\n", path, err)
		return nil, nil
	}
	if !d.IsDir() {
		return nil, nil
	}
	if !isGitRepo(path) {
		return nil, nil
	}
	return &Repository{Path: path}, nil
}

// Discover recursively finds Git repositories under root. Once a repository is
// found, traversal does not descend into it.
func Discover(root string) ([]Repository, error) {
	var repositories []Repository
	var mu sync.Mutex
	conf := &fastwalk.Config{}
	err := fastwalk.Walk(conf, root, func(path string, d fs.DirEntry, err error) error {
		repo, err := traversalEntry(path, d, err)
		if err != nil {
			return err
		}
		if repo != nil {
			mu.Lock()
			repositories = append(repositories, *repo)
			mu.Unlock()
			return filepath.SkipDir
		}
		return nil
	})
	return repositories, err
}
