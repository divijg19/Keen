package keen

import (
	"os"
	"path/filepath"
)

// ResolveDisplayIdentities returns presentation copies of repositories in
// which Name carries the minimum display identity that keeps every
// repository in the given set distinguishable.
//
// Repositories whose basenames are unique within the set keep their basename.
// When two or more repositories share a basename, only those repositories
// expand — each prepending one ancestor path component at a time until their
// candidates differ. Growth is targeted: a candidate stops expanding as soon
// as it is unique within the set, so unrelated repositories never pay for a
// collision they are not part of. Expanded identities always contain a path
// separator and therefore can never collide with any plain basename.
//
// The transform is index-aligned: input[i] maps to output[i], ordering is
// preserved, and the input slice (and its elements) are never mutated. It is
// a pure function of the repository paths in the set: no filesystem access,
// no Git subprocesses, no traversal.
//
// Termination is guaranteed. A member of a collision class grows only while
// it has ancestor components remaining; when none remain (or when duplicate
// paths make expansion futile), the class freezes at its deepest available
// suffix. Duplicate repository paths therefore freeze to identical
// identities rather than looping forever; deduplication remains outside this
// function's responsibility.
func ResolveDisplayIdentities(repositories []Repository) []Repository {
	result := make([]Repository, len(repositories))
	copy(result, repositories)

	candidates := make([]string, len(repositories))
	dirs := make([]string, len(repositories)) // next ancestor directory; "" = exhausted
	for i := range result {
		candidates[i] = result[i].Name
		dirs[i] = firstAncestorDir(result[i].Path)
	}

	for {
		groups := make(map[string][]int)
		for i := range candidates {
			groups[candidates[i]] = append(groups[candidates[i]], i)
		}

		progressed := false
		for _, indices := range groups {
			if len(indices) < 2 {
				continue // already unique; frozen
			}
			for _, i := range indices {
				if !identityCanGrow(dirs[i]) {
					continue // exhausted; freezes at its deepest suffix
				}
				candidates[i] = filepath.Base(dirs[i]) + "/" + candidates[i]
				dirs[i] = identityNextDir(dirs[i])
				progressed = true
			}
		}
		if !progressed {
			break
		}
	}

	for i := range result {
		result[i].Name = candidates[i]
	}
	return result
}

// firstAncestorDir returns the directory containing the path's final
// component, or "" when the path has no meaningful ancestor.
func firstAncestorDir(path string) string {
	dir := filepath.Dir(path)
	if identityCanGrow(dir) {
		return dir
	}
	return ""
}

func identityCanGrow(dir string) bool {
	return dir != "" && dir != "." && dir != string(os.PathSeparator)
}

// identityNextDir returns dir's parent, or "" once dir is the filesystem
// root (or its equivalent), signalling exhaustion.
func identityNextDir(dir string) string {
	parent := filepath.Dir(dir)
	if parent == dir {
		return ""
	}
	return parent
}
