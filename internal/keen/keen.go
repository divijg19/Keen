package keen

import "time"

// reportIndent is the shared content indentation applied by every report and
// investigation renderer. Both the textual reports and the interactive
// surfaces use the same leading indent so their output aligns consistently.
const reportIndent = "    "

// Repository is the central domain model for a discovered Git repository.
// Its fields are intentionally minimal and stable across the v0.4.x / v0.5.x line.
//
// The model is deliberately flat: synchronization facts (Ahead, Behind,
// Upstream) and commit identity (LastCommitHash, LastCommitSubject) live
// directly on the struct rather than in nested state objects. Adding a small
// number of scalar facts does not, by itself, justify an information-model
// redesign; that remains reserved for detail-oriented features that create
// genuine semantic grouping pressure.
type Repository struct {
	Path              string
	Name              string
	Branch            string
	Dirty             bool
	Ahead             int
	Behind            int
	Upstream          string
	LastCommitAt      time.Time
	LastCommitTime    string
	LastCommitHash    string
	LastCommitSubject string
}

// OutputMode selects how repositories are presented.
type OutputMode int

const (
	OutputGrouped OutputMode = iota
	OutputCompact
)

// Options carries the CLI selection and presentation preferences.
//
// Presentation is expressed as three explicit, mutually exclusive modes:
//   - canonical textual report (default)
//   - rich textual report (-r)
//   - interactive investigation (-i)
//
// Selection flags (ShowClean/ShowDirty/HasRecent) apply identically across all
// three modes; a presentation mode never alters repository selection.
type Options struct {
	ShowClean   bool
	ShowDirty   bool
	HasRecent   bool
	RecentAfter time.Time
	Compact     bool
	Rich        bool
	Interactive bool
}
