package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/divijg19/Keen/internal/keen"
)

// parseArgs converts CLI arguments into Keen's Options. Selection flags
// (--clean/--dirty/--recent) are independent of the presentation mode; the
// three presentation modes (canonical, rich via -r, interactive via -i) are
// mutually exclusive.
func parseArgs(args []string) (keen.Options, error) {
	fs := flag.NewFlagSet("keen", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	var opts keen.Options
	var recentFlag string
	fs.BoolVar(&opts.ShowClean, "clean", false, "show only clean repositories")
	fs.BoolVar(&opts.ShowDirty, "dirty", false, "show only dirty repositories")
	fs.StringVar(&recentFlag, "recent", "", "show only repositories with commits within duration (e.g. 30m, 24h, 7d)")
	fs.BoolVar(&opts.Compact, "compact", false, "use compact output (canonical report only)")
	fs.BoolVar(&opts.Rich, "r", false, "use the rich textual report")
	fs.BoolVar(&opts.Interactive, "i", false, "open the interactive investigation (prints a one-shot overview when stdin is not a terminal)")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}

	// Mode flags select the presentation; modifiers refine the canonical
	// report unless explicitly supported by another mode. --compact belongs
	// to the canonical renderer only, so these combinations are invalid.
	if opts.Rich && opts.Interactive {
		err := fmt.Errorf("cannot combine -r (rich) and -i (interactive)")
		fmt.Fprintf(os.Stderr, "keen: %v\n", err)
		return opts, err
	}
	if opts.Compact && (opts.Rich || opts.Interactive) {
		err := fmt.Errorf("--compact applies only to the canonical report (not valid with -r or -i)")
		fmt.Fprintf(os.Stderr, "keen: %v\n", err)
		return opts, err
	}

	if recentFlag != "" {
		dur, err := keen.ParseRecentDuration(recentFlag)
		if err != nil {
			verr := fmt.Errorf("invalid --recent duration: %q", recentFlag)
			fmt.Fprintf(os.Stderr, "keen: %v\n", verr)
			return opts, verr
		}
		opts.HasRecent = true
		opts.RecentAfter = time.Now().Add(-dur)
	}

	return opts, nil
}

func main() {
	workingDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invocation or runtime failed: %v\n", err)
		return
	}

	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		// Flag parse errors already printed usage to stdout; custom validation
		// errors were printed to stderr above. Either way, exit non-zero.
		os.Exit(1)
	}

	repositories, err := keen.Discover(workingDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Filesystem traversal failed: %v\n", err)
		return
	}

	for i := range repositories {
		if err := keen.Enrich(&repositories[i]); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to inspect %s: %v\n", repositories[i].Path, err)
		}
	}

	keen.Sort(repositories)
	filtered := keen.Filter(repositories, opts)
	// Presentation boundary: display identities are resolved once, against
	// the filtered set, so all three modes consume identical identities.
	presented := keen.ResolveDisplayIdentities(filtered)

	if opts.Interactive {
		keen.Browse(presented, len(repositories))
		return
	}

	fmt.Println("===KEEN===")

	if opts.Rich {
		keen.PrintRich(presented, len(repositories))
		return
	}

	mode := keen.OutputGrouped
	if opts.Compact {
		mode = keen.OutputCompact
	}
	keen.Print(presented, mode, len(repositories))
}
