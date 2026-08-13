package keen

import "slices"

// Sort orders repositories stably: clean before dirty, preserving the discovery
// order within each group.
func Sort(repositories []Repository) {
	slices.SortStableFunc(repositories, func(a, b Repository) int {
		if !a.Dirty && b.Dirty {
			return -1
		}
		if a.Dirty && !b.Dirty {
			return 1
		}
		return 0
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
