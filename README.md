# `Keen`

`Keen` recursively discovers Git repositories under the current working directory and reports their status. The CLI command is `keen`.

## Build & install

```text
go build -o keen ./cmd/keen
go install ./cmd/keen
```

## Usage

```text
keen [flags]
```

Run `keen` from any directory containing Git repositories. Discovered repositories are grouped by working-tree status (clean before dirty) in a deterministic order (status → name → path).

## Flags

| Flag       | Description                                                          |
| ---------- | -------------------------------------------------------------------- |
| `--clean`  | Show only repositories with a clean worktree.                        |
| `--dirty`  | Show only repositories with uncommitted changes.                     |
| `--recent` | Show only repositories with commits within duration (e.g. 24h, 7d). |
| `--compact`| Use a compact one-line-per-repository layout (canonical report only).|
| `-r`       | Use the rich textual report.                                         |
| `-i`       | Open the interactive browser (prints a one-shot overview when stdin is not a terminal). |
| `--help`   | Print usage information.                                             |

Filtering and presentation mode are orthogonal: `--clean`, `--dirty`, and
`--recent` control *which* repositories are selected, while the presentation mode
(`keen`, `-r`, `-i`) controls *how* they are presented. The mode never changes
selection; the same filters apply and the browser/rich report consume the already
filtered set.

Mode flags select the presentation; modifiers refine the canonical report unless
explicitly supported by another mode. `--compact` belongs to the canonical
renderer, so `keen -r --compact` and `keen -i --compact` fail with a usage error
rather than being ignored.

### Presentation modes

```text
keen        canonical textual report
keen -r     rich textual report
keen -i     interactive browser
```

`-i` is the permanent interactive entrypoint. The lightweight browser in v0.5.6
may later be replaced by a full alt-screen TUI without changing the public
invocation.

### Interactive browser

The browser contains exactly two views:

| View      | Answers                                  | Shows                                                        |
| --------- | ---------------------------------------- | ------------------------------------------------------------ |
| Overview  | What needs my attention?                 | status, name, branch, upstream, ahead/behind.                |
| Activity  | What happened recently?                   | name, short hash, commit subject, relative commit time.      |

Overview shows each repository's branch *and* its upstream (using `↑– ↓–` when no
upstream is configured). Activity shows the latest commit identity (short hash,
subject, relative time) and `No commits` for repositories with no history.

Navigate between views with `←`/`→` or `h`/`l`. A view wider than the terminal is
revealed with a horizontal viewport: scroll it with `Shift+←`/`Shift+→`. Press
`q` or `Esc` to quit (Ctrl+C also quits). A persistent `‹ VIEW ›  n / 2` indicator
shows the active view.

### Repository identity

Keen normally displays repositories by their directory name. When multiple
repositories in the current result set share that name, Keen automatically
includes the minimum amount of parent path needed to distinguish them:

```text
work/api
personal/api
```

Identity is resolved from the repositories currently being displayed, so
filtering can reduce an expanded identity back to its basename. Unique names
never change, and unrelated repositories are never affected by a collision
elsewhere in the output.

### Filter composition

Filters combine using **AND** semantics:

| Invocation                     | Selection               | Presentation |
| ------------------------------ | ----------------------- | ------------ |
| `keen`                         | all                     | grouped      |
| `keen --clean`                 | clean                   | grouped      |
| `keen --dirty`                 | dirty                   | grouped      |
| `keen --recent 7d`             | recent (last 7 days)    | grouped      |
| `keen --dirty --recent 7d`     | dirty AND recent        | grouped      |
| `keen --clean --recent 7d`     | clean AND recent        | grouped      |
| `keen --compact --recent 7d`   | recent                  | compact      |
| `keen --clean --dirty`         | all                     | grouped      |
| `keen -r`                      | all                     | rich         |
| `keen -r --dirty`              | dirty                   | rich         |
| `keen -i`                      | all                     | interactive  |
| `keen -i --recent 7d`          | recent (last 7 days)    | interactive  |

Supported `--recent` duration units: `s` (seconds), `m` (minutes), `h` (hours), `d` (days), `w` (weeks).

## Output

Grouped mode prints a `Git Status: CLEAN` / `Git Status: DIRTY` section only when that section contains at least one repository:

```text
===KEEN===

    Git Status: CLEAN
---------------------------
[clean] project-a       (main → origin/main) ↑0 ↓0  | 4e02b68 add temporal repository filtering | 2 days ago

    Git Status: DIRTY
---------------------------
[dirty] project-b       (feature/x → origin/feature/x) ↑1 ↓0  | a1b2c3d fix parser panic | 5 minutes ago
```

If no repositories exist, `keen` prints `No repositories found.` If repositories exist but none match active filters, `keen` prints `No repositories match the selected filters.`

For each repository, `keen` reports the branch, the configured upstream (`branch → upstream`), commits ahead/behind that upstream (`↑n ↓n`), the latest commit (seven-character short hash plus subject), and the relative time of that commit.

### Rich report

`keen -r` presents the same facts as an aligned column table (status, name, branch, upstream, ahead, behind, hash, subject, time) for easier comparison across repositories. The table adapts deterministically to the terminal width: columns tighten first, lower-priority columns are dropped next, and on very narrow terminals `keen -r` renders the canonical grouped report instead. Rendering is deterministic — the same repository state at the same width always produces identical output.

## Known limitations

- A repository in a detached HEAD state reports `detached` as its branch; this is expected and not an error.
- Nested repositories (a repository inside another repository) are not separately reported, since traversal stops at the first discovered repository.
- Repositories without an upstream report `↑– ↓–` (a non-numeric marker) so that "0/0" is not mistaken for a synchronized state; the configured upstream, when present, is shown as `branch → upstream`. Repositories with no commits report `No commits`.
