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
	} else {
		fields := strings.Fields(output)
		if len(fields) == 2 {
			repo.Ahead, _ = strconv.Atoi(fields[0])
			repo.Behind, _ = strconv.Atoi(fields[1])
		}
	}

	// Last commit timestamp (machine-readable ISO-8601).
	output, err = runGit(repo.Path, "log", "-1", "--format=%cI")
	if err != nil || strings.TrimSpace(output) == "" {
		// Repository legitimately has no commits.
		repo.LastCommitAt = time.Time{}
		repo.LastCommitTime = "No commits"
		return nil
	}
	parsedTime, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(output))
	if parseErr != nil {
		return fmt.Errorf("parse last commit time: %w", parseErr)
	}
	repo.LastCommitAt = parsedTime

	// Last commit relative date (human-readable presentation).
	output, err = runGit(repo.Path, "log", "-1", "--date=relative", "--pretty=%cd")
	if err != nil {
		return fmt.Errorf("read last commit date: %w", err)
	}
	repo.LastCommitTime = strings.TrimSpace(output)

	return nil
}
