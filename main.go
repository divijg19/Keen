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
		fmt.Println("Invocation or runtime error: ", err)
		return
	}
	fmt.Println(workingDir)
	conf := &fastwalk.Config{}
	err = fastwalk.Walk(conf, workingDir, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			if isGitRepo(path) {
				fmt.Println(path)
			}
		}
		return nil
	})
	if err != nil {
		fmt.Println("Ran into an error, are you sure path is correct?")
	}
}

func isGitRepo(path string) bool {
	gitPath := filepath.Join(path, ".git")
	_, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	return true
}
