package keen

import (
	"fmt"
	"strconv"
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
	Status    string // A, M, D, R, C, etc.
	Path      string // for renames/copies: new path or just path
	OldPath   string // only for renames/copies
	Additions int
	Deletions int
	IsBinary  bool
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
	statusOut, err := runGit(repoPath, "diff-tree", "--no-commit-id", "--name-status", "-r", "-M", "-z", "--root", hash)
	if err != nil {
		return nil, err
	}
	numstatOut, err := runGit(repoPath, "diff-tree", "--no-commit-id", "--numstat", "-r", "-M", "-z", "--root", hash)
	if err != nil {
		return parseChangedFiles(statusOut)
	}
	return parseChangedFilesWithNumstat(statusOut, numstatOut)
}

func parseChangedFilesWithNumstat(statusRaw, numstatRaw string) ([]ChangedFile, error) {
	files, err := parseChangedFiles(statusRaw)
	if err != nil {
		return nil, err
	}
	fields := strings.Split(numstatRaw, "\x00")
	type statEntry struct {
		add, del int
		isBin    bool
		path     string
	}
	var stats []statEntry
	i := 0
	for i < len(fields) {
		addStr := fields[i]
		if addStr == "" {
			break
		}
		i++
		if i >= len(fields) {
			break
		}
		delStr := fields[i]
		i++
		if i >= len(fields) {
			break
		}
		p1 := fields[i]
		i++
		isBin := (addStr == "-" || delStr == "-")
		add, _ := strconv.Atoi(addStr)
		del, _ := strconv.Atoi(delStr)
		stats = append(stats, statEntry{add: add, del: del, isBin: isBin, path: p1})
	}

	for idx := range files {
		f := &files[idx]
		for _, s := range stats {
			if s.path == f.Path || (f.OldPath != "" && (s.path == f.Path || s.path == f.OldPath)) {
				f.Additions = s.add
				f.Deletions = s.del
				f.IsBinary = s.isBin
				break
			}
		}
	}
	return files, nil
}

// parseChangedFiles parses git diff-tree --name-status -z output, which is a
// sequence of NUL-terminated records. Status is the first field of each record;
// statuses that begin with R or C carry an old and new path, all others a single
// path. Paths are already raw (unescaped) because -z never quote-escapes them.
func parseChangedFiles(raw string) ([]ChangedFile, error) {
	fields := strings.Split(raw, "\x00")
	var files []ChangedFile
	i := 0
	for i < len(fields) {
		status := fields[i]
		if status == "" {
			break
		}
		i++
		// Status may include a similarity score (e.g. R100); reduce to its
		// first letter for display so R100/C100 render as R/C.
		displayStatus := status
		if len(displayStatus) > 1 {
			displayStatus = displayStatus[:1]
		}
		if displayStatus == "R" || displayStatus == "C" {
			if i >= len(fields) || fields[i] == "" || i+1 >= len(fields) || fields[i+1] == "" {
				return nil, fmt.Errorf("unexpected rename/copy record for status %q", status)
			}
			files = append(files, ChangedFile{Status: displayStatus, OldPath: fields[i], Path: fields[i+1]})
			i += 2
		} else {
			if i >= len(fields) || fields[i] == "" {
				return nil, fmt.Errorf("unexpected changed-file record for status %q", status)
			}
			files = append(files, ChangedFile{Status: displayStatus, Path: fields[i]})
			i++
		}
	}
	return files, nil
}
