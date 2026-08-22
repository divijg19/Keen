package keen

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// runGit centralizes Git command execution through a single narrow abstraction
// over exec.Command. It returns combined standard output as a string and any
// error from the subprocess.
func runGit(dir string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", cmdArgs...)
	output, err := cmd.Output()
	return string(output), err
}

// Enrich populates a repository's Git metadata using separate, intentional Git
// commands. The call sites reflect distinct pieces of metadata and are kept
// separate to ease future metadata additions without expanding Enrich indefinitely.
//
// Invariant: repo.LastCommitAt is zero if and only if the repository has no
// commits. A malformed machine-readable timestamp is an enrichment error, never
// a silent downgrade to "no commit".
//
// Git fact taxonomy:
//
//	Required facts — failure is an enrichment error:
//	  working-tree status      (git status --porcelain)
//	  branch                   (git branch --show-current; detached HEAD → "")
//	  commit identity/timestamp (git log -1 --format=%H/%s/%cI/%cd)
//
//	Optional facts — failure degrades gracefully:
//	  upstream tracking        (git rev-list HEAD...@{upstream})
//	    absent upstream → Ahead = 0, Behind = 0, Upstream = ""
//	  upstream identity        (git rev-parse --abbrev-ref --symbolic-full-name @{upstream})
//	    absent upstream → Upstream = ""
func Enrich(repo *Repository) error {
	repo.Name = filepath.Base(repo.Path)

	output, err := runGit(repo.Path, "status", "--porcelain")
	if err != nil {
		return err
	}
	repo.Dirty = len(output) > 0

	output, err = runGit(repo.Path, "branch", "--show-current")
	if err != nil {
		return err
	}
	repo.Branch = strings.TrimSpace(output)

	output, err = runGit(repo.Path, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		// Repository may not have an upstream configured.
		repo.Ahead = 0
		repo.Behind = 0
		repo.Upstream = ""
	} else {
		fields := strings.Fields(output)
		if len(fields) == 2 {
			repo.Ahead, _ = strconv.Atoi(fields[0])
			repo.Behind, _ = strconv.Atoi(fields[1])
		}
		if upstream, uerr := runGit(repo.Path, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); uerr == nil {
			repo.Upstream = strings.TrimSpace(upstream)
		}
	}

	// Last commit identity and timestamps, consolidated into a single Git call.
	// The four logical fields are newline-separated; %s (subject) is a single
	// line, so splitting on "\n" is unambiguous.
	output, err = runGit(repo.Path, "log", "-1", "--date=relative", "--format=%H%x0a%s%x0a%cI%x0a%cd")
	if err != nil || strings.TrimSpace(output) == "" {
		// Repository legitimately has no commits.
		repo.LastCommitAt = time.Time{}
		repo.LastCommitTime = "No commits"
		repo.LastCommitHash = ""
		repo.LastCommitSubject = ""
		return nil
	}
	parts := strings.Split(strings.TrimSpace(output), "\n")
	if len(parts) < 4 {
		return fmt.Errorf("unexpected git log output: %q", output)
	}
	repo.LastCommitHash = parts[0]
	repo.LastCommitSubject = parts[1]
	parsedTime, parseErr := time.Parse(time.RFC3339, parts[2])
	if parseErr != nil {
		return fmt.Errorf("parse last commit time: %w", parseErr)
	}
	repo.LastCommitAt = parsedTime
	repo.LastCommitTime = parts[3]

	return nil
}
