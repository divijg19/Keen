package keen

import (
	"fmt"
	"strings"
)

// Commit represents a single Git commit examined during investigation.
// It is intentionally minimal and lives only in the interactive history
// surface; Repository remains flat and pipeline-global history is never
// populated.
type Commit struct {
	Hash          string
	Subject       string
	Body          string
	Author        string
	AuthorDate    string
	Committer     string
	CommitterDate string
	Parents       []string
}

// ChangedFile describes one path touched by a commit.
type ChangedFile struct {
	Status  string // A, M, D, R, C, etc.
	Path    string // for renames/copies: "old -> new" or just new path
	OldPath string // only for renames/copies
}

const commitHistoryLimit = 20

// commitFieldSep and commitRecordSep are the delimiters used in git log
// --format. Unit Separator (0x1f) between fields and Record Separator (0x1e)
// between commits make field boundaries unambiguous even when subjects,
// bodies or author names contain spaces or punctuation.
const commitFieldSep = "\x1f"
const commitRecordSep = "\x1e"

// loadCommitHistory returns the recent commit history for the repository at
// repoPath, bounded to commitHistoryLimit entries. It is loaded on demand
// when the user descends from Activity and never during the global Enrich
// pipeline.
func loadCommitHistory(repoPath string) ([]Commit, error) {
	// %H hash, %s subject, %b body, %an author, %ai author date (ISO-like),
	// %cn committer, %ci committer date, %P parents, %x1f field sep, %x1e record sep.
	format := "%H%x1f%s%x1f%b%x1f%an%x1f%ai%x1f%cn%x1f%ci%x1f%P%x1e"
	out, err := runGit(repoPath, "log", fmt.Sprintf("-n%d", commitHistoryLimit), "--date=iso-strict", "--format="+format)
	if err != nil {
		// Empty repository or no commits: git log exits non-zero.
		if strings.TrimSpace(out) == "" {
			return nil, nil
		}
		return nil, err
	}
	return parseCommitHistory(out)
}

// parseCommitHistory parses the raw log output produced by loadCommitHistory.
func parseCommitHistory(raw string) ([]Commit, error) {
	raw = strings.TrimSuffix(raw, commitRecordSep)
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	records := strings.Split(raw, commitRecordSep)
	var commits []Commit
	for _, rec := range records {
		rec = strings.Trim(rec, "\n")
		if rec == "" {
			continue
		}
		fields := strings.Split(rec, commitFieldSep)
		if len(fields) != 8 {
			return nil, fmt.Errorf("unexpected commit record with %d fields: %q", len(fields), rec)
		}
		c := Commit{
			Hash:          strings.TrimSpace(fields[0]),
			Subject:       fields[1],
			Body:          strings.Trim(fields[2], "\n"),
			Author:        fields[3],
			AuthorDate:    strings.TrimSpace(fields[4]),
			Committer:     fields[5],
			CommitterDate: strings.TrimSpace(fields[6]),
		}
		parentsRaw := strings.TrimSpace(fields[7])
		if parentsRaw != "" {
			c.Parents = strings.Fields(parentsRaw)
		}
		commits = append(commits, c)
	}
	return commits, nil
}

// loadChangedFiles returns the files touched by the commit identified by hash
// in repoPath. It uses a machine-readable format with rename detection.
func loadChangedFiles(repoPath, hash string) ([]ChangedFile, error) {
	if hash == "" {
		return nil, nil
	}
	// --no-commit-id suppresses hash line, -r recurses, -M detects renames,
	// --name-status gives status + tab + path(s).
	// For root commit, diff-tree without --no-commit-id would show nothing;
	// use --root to include root diff.
	out, err := runGit(repoPath, "diff-tree", "--no-commit-id", "--name-status", "-r", "-M", "--root", hash)
	if err != nil {
		return nil, err
	}
	return parseChangedFiles(out)
}

// parseChangedFiles parses git diff-tree --name-status output.
func parseChangedFiles(raw string) ([]ChangedFile, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	lines := strings.Split(raw, "\n")
	var files []ChangedFile
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Format: <status>\t<path>  or  <status>\t<old>\t<new> for renames
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			return nil, fmt.Errorf("unexpected changed-file line: %q", line)
		}
		status := parts[0]
		// Status may be like "R100" for renames; keep first letter for display but preserve full?
		// Use first rune as status category.
		displayStatus := status
		if len(status) > 1 {
			// For renames/copies, status is e.g. R100; display as R
			displayStatus = status[:1]
		}
		if len(parts) == 2 {
			files = append(files, ChangedFile{Status: displayStatus, Path: parts[1]})
		} else if len(parts) == 3 {
			// Rename/copy: old and new
			files = append(files, ChangedFile{Status: displayStatus, Path: parts[2], OldPath: parts[1]})
		} else {
			// Unexpected but handle: join remaining?
			files = append(files, ChangedFile{Status: displayStatus, Path: parts[len(parts)-1], OldPath: parts[1]})
		}
	}
	return files, nil
}
