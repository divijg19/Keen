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

Run `keen` from any directory containing Git repositories. Discovered repositories are grouped by working-tree status (clean before dirty) in a stable order.

## Flags

| Flag       | Description                                      |
| ---------- | ------------------------------------------------ |
| `--clean`  | Show only repositories with a clean worktree.    |
| `--dirty`  | Show only repositories with uncommitted changes. |
| `--compact`| Use a compact one-line-per-repository layout.    |
| `--help`   | Print usage information.                         |

Filtering and output mode are orthogonal: `--clean`/`--dirty` control *which* repositories are selected, `--compact` controls *how* they are presented.

| Invocation               | Selection | Presentation |
| ------------------------ | --------- | ------------ |
| `keen`                   | all       | grouped      |
| `keen --clean`           | clean     | grouped      |
| `keen --dirty`           | dirty     | grouped      |
| `keen --compact`         | all       | compact      |
| `keen --clean --compact` | clean     | compact      |
| `keen --dirty --compact` | dirty     | compact      |

`--clean --dirty` together means all repositories.

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

If no repositories match, `keen` prints `No repositories found.`

For each repository, `keen` reports the branch, commits ahead/behind the upstream (`↑n ↓n`), and the relative time of the last commit.

## Known limitations

- A repository in a detached HEAD state reports an empty branch name; this is expected and not an error.
- Nested repositories (a repository inside another repository) are not separately reported, since traversal stops at the first discovered repository.
- Repositories without an upstream report `↑0 ↓0`; repositories with no commits report `No commits`.
