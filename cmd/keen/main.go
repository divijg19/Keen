package main

import (
	"flag"
	"fmt"
	"os"
	"time"

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
	var recentFlag string
	flag.BoolVar(&opts.ShowClean, "clean", false, "show only clean repositories")
	flag.BoolVar(&opts.ShowDirty, "dirty", false, "show only dirty repositories")
	flag.StringVar(&recentFlag, "recent", "", "show only repositories with commits within duration (e.g. 30m, 24h, 7d, 2w)")
	flag.BoolVar(&opts.Compact, "compact", false, "use compact output")
	flag.Parse()

	if recentFlag != "" {
		dur, err := keen.ParseRecentDuration(recentFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --recent duration: %q\n", recentFlag)
			os.Exit(1)
		}
		opts.HasRecent = true
		opts.RecentAfter = time.Now().Add(-dur)
	}

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
	keen.Print(filtered, mode, len(repositories))
}
