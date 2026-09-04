package keen

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// runGit centralizes Git command execution through a single narrow abstraction
// over exec.Command. It returns standard output as a string; on failure the
// error carries Git's stderr so diagnostics name the cause without polluting
// the parsed stdout contract.
func runGit(dir string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", cmdArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			return string(output), fmt.Errorf("%w: %s", err, detail)
		}
		return string(output), err
	}
	return string(output), nil
}

// parseAheadBehind interprets `git rev-list --left-right --count` output: two
// whitespace-separated integers, ahead then behind. Anything else — wrong
// field count or non-numeric fields — means the synchronization fact is
// unavailable, never a verified zero. Callers must leave the repository
// without an upstream on !ok so renderers show the explicit unavailable
// marker instead of a misleading synchronized state.
func parseAheadBehind(output string) (ahead, behind int, ok bool) {
	fields := strings.Fields(output)
	if len(fields) != 2 {
		return 0, 0, false
	}
	var err error
	ahead, err = strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, false
	}
	behind, err = strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, false
	}
	return ahead, behind, true
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
//	    absent upstream or uninterpretable output → Ahead = 0, Behind = 0,
//	    Upstream = "" (explicitly unavailable, never a verified zero)
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
	if ahead, behind, ok := parseAheadBehind(output); err != nil || !ok {
		// No upstream, or output that cannot be read as a sync fact:
		// explicitly unavailable, never a verified synchronized state.
		repo.Ahead = 0
		repo.Behind = 0
		repo.Upstream = ""
	} else {
		repo.Ahead = ahead
		repo.Behind = behind
		if upstream, uerr := runGit(repo.Path, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); uerr == nil {
			repo.Upstream = strings.TrimSpace(upstream)
		} else {
			// Counts without a resolvable upstream name are not trustworthy.
			repo.Ahead = 0
			repo.Behind = 0
			repo.Upstream = ""
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
