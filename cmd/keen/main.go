package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/divijg19/Keen/internal/keen"
)

func main() {
	workingDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Invocation or runtime failed: %v\n", err)
		return
	}

	repositories, err := keen.Discover(workingDir)
	if err != nil {
		fmt.Printf("Filesystem traversal failed: %v\n", err)
		return
	}

	var opts keen.Options
	flag.BoolVar(&opts.ShowClean, "clean", false, "show only clean repositories")
	flag.BoolVar(&opts.ShowDirty, "dirty", false, "show only dirty repositories")
	flag.BoolVar(&opts.Compact, "compact", false, "use compact output")
	flag.Parse()

	fmt.Println("===KEEN===")

	for i := range repositories {
		if err := keen.Enrich(&repositories[i]); err != nil {
			fmt.Printf("Failed to inspect %s: %v\n", repositories[i].Path, err)
		}
	}

	keen.Sort(repositories)
	filtered := keen.Filter(repositories, opts)

	mode := keen.OutputGrouped
	if opts.Compact {
		mode = keen.OutputCompact
	}
	keen.Print(filtered, mode)
}
