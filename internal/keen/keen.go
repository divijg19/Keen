package keen

// Repository is the central domain model for a discovered Git repository.
// Its fields are intentionally minimal and stable across the v0.4.x line.
type Repository struct {
	Path           string
	Name           string
	Branch         string
	Dirty          bool
	Ahead          int
	Behind         int
	LastCommitTime string
}

// OutputMode selects how repositories are presented.
type OutputMode int

const (
	OutputGrouped OutputMode = iota
	OutputCompact
)

// Options carries the CLI selection and presentation preferences.
type Options struct {
	ShowClean bool
	ShowDirty bool
	Compact   bool
}
