package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/charlievieth/fastwalk"
)

func main() {
	fmt.Println("===KEEN===")
	workingDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Invocation or runtime error: %v\n", err)
		return
	}
	conf := &fastwalk.Config{}
	err = fastwalk.Walk(conf, workingDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Printf("Skipping: %s | %v\n", path, err)
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if !isGitRepo(path) {
			return nil
		}
		fmt.Println(path)
		return nil
	})
	if err != nil {
		fmt.Printf("Filesystem traversal failed: %v\n", err)
	}
}

func isGitRepo(path string) bool {
	gitPath := filepath.Join(path, ".git")
	_, err := os.Stat(gitPath)
	return err == nil
}
