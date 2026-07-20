package main

import (
	"fmt"
	"io/fs"
	"os"

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
			fmt.Println(path)
		}
		return nil
	})
	if err != nil {
		fmt.Println("Ran into an error, are you sure path is correct?")
	}
}
