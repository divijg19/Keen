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
| `--compact`| Use a compact one-line-per-repository layout.                        |
| `--help`   | Print usage information.                                             |

Filtering and output mode are orthogonal: `--clean`, `--dirty`, and `--recent` control *which* repositories are selected, `--compact` controls *how* they are presented.

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

Supported `--recent` duration units: `s` (seconds), `m` (minutes), `h` (hours), `d` (days), `w` (weeks).

## Output

Grouped mode prints a `Git Status: CLEAN` / `Git Status: DIRTY` section only when that section contains at least one repository:

```text
===KEEN===

    Git Status: CLEAN
---------------------------
[clean] project-a       (main        ) ↑0  ↓0  | 2 days ago

    Git Status: DIRTY
---------------------------
[dirty] project-b       (feature/x   ) ↑1  ↓0  | 5 minutes ago
```

If no repositories exist, `keen` prints `No repositories found.` If repositories exist but none match active filters, `keen` prints `No repositories match the selected filters.`

For each repository, `keen` reports the branch, commits ahead/behind the upstream (`↑n ↓n`), and the relative time of the last commit.

## Known limitations

- A repository in a detached HEAD state reports an empty branch name; this is expected and not an error.
- Nested repositories (a repository inside another repository) are not separately reported, since traversal stops at the first discovered repository.
- Repositories without an upstream report `↑0 ↓0`; repositories with no commits report `No commits`.
