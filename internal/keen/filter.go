package keen

import (
	"slices"
	"strings"
)

// Sort orders repositories deterministically:
// 1. Clean repositories precede dirty repositories.
// 2. Within each status group, repositories are ordered alphabetically by Name (ascending).
// 3. When names are equal, sort by Path (ascending) as a deterministic tie-breaker.
func Sort(repositories []Repository) {
	slices.SortFunc(repositories, func(a, b Repository) int {
		if a.Dirty != b.Dirty {
			if a.Dirty {
				return 1
			}
			return -1
		}

		if n := strings.Compare(a.Name, b.Name); n != 0 {
			return n
		}

		return strings.Compare(a.Path, b.Path)
	})
}

// Filter applies the --clean/--dirty selection. Supplying both flags, or
// neither, yields all repositories.
func Filter(repositories []Repository, opts Options) []Repository {
	if !opts.ShowClean && !opts.ShowDirty {
		return repositories
	}
	var filtered []Repository
	for _, r := range repositories {
		if opts.ShowClean && !r.Dirty {
			filtered = append(filtered, r)
		}
		if opts.ShowDirty && r.Dirty {
			filtered = append(filtered, r)
		}
	}
	return filtered
}
