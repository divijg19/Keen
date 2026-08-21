package keen

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
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

// ParseRecentDuration parses duration strings supporting s, m, h, d, and w units.
// Zero, negative, or malformed durations return an error.
func ParseRecentDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("duration cannot be empty")
	}
	if strings.HasPrefix(s, "-") {
		return 0, fmt.Errorf("duration must be positive: %q", s)
	}

	// Handle 'd'/'D' (days) and 'w'/'W' (weeks)
	if strings.HasSuffix(s, "d") || strings.HasSuffix(s, "D") {
		valStr := s[:len(s)-1]
		val, err := strconv.Atoi(valStr)
		if err != nil || val <= 0 {
			return 0, fmt.Errorf("invalid day duration: %q", s)
		}
		return time.Duration(val) * 24 * time.Hour, nil
	}
	if strings.HasSuffix(s, "w") || strings.HasSuffix(s, "W") {
		valStr := s[:len(s)-1]
		val, err := strconv.Atoi(valStr)
		if err != nil || val <= 0 {
			return 0, fmt.Errorf("invalid week duration: %q", s)
		}
		return time.Duration(val) * 7 * 24 * time.Hour, nil
	}

	// Standard library units (s, m, h, etc.)
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, fmt.Errorf("duration must be positive: %q", s)
	}
	return d, nil
}

// Filter applies selection based on --clean/--dirty state and --recent temporal cutoffs.
// Independent filters compose using AND semantics. Supplying both --clean and --dirty
// selects all repositories. Filter preserves the deterministic ordering of the input slice.
func Filter(repositories []Repository, opts Options) []Repository {
	hasStatusFilter := opts.ShowClean != opts.ShowDirty
	hasRecentFilter := opts.HasRecent

	if !hasStatusFilter && !hasRecentFilter {
		return repositories
	}

	var filtered []Repository
	for _, r := range repositories {
		statusMatch := true
		if hasStatusFilter {
			if opts.ShowClean {
				statusMatch = !r.Dirty
			} else {
				statusMatch = r.Dirty
			}
		}

		recentMatch := true
		if hasRecentFilter {
			if r.LastCommitAt.IsZero() {
				recentMatch = false
			} else {
				recentMatch = !r.LastCommitAt.Before(opts.RecentAfter)
			}
		}

		if statusMatch && recentMatch {
			filtered = append(filtered, r)
		}
	}
	return filtered
}
