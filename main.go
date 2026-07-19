package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("===KEEN===")
	workingDir, err := os.Getwd()
	if err != nil {
		fmt.Println("Invocation or runtime error: ", err)
		return
	}
	fmt.Println(workingDir)
}
