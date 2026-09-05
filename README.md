# `Keen`

`keen` recursively discovers Git repositories beneath the current working directory and reports their status — clean/dirty, branch, upstream synchronization, and latest commit.

## Build & install

```text
go build -o keen ./cmd/keen
go install ./cmd/keen
```

## Usage

```text
keen [flags]
```

Run `keen` from any directory containing Git repositories. The report opens with a one-line workspace summary (e.g. `3 repositories, 2 clean, 1 dirty`) describing the selected set, then lists repositories grouped by working-tree status (clean before dirty) and ordered deterministically by status → name → path.

## Flags

| Flag         | Description                                                       |
| ------------ | ----------------------------------------------------------------- |
| `--clean`    | Show only clean repositories.                                     |
| `--dirty`    | Show only dirty repositories.                                     |
| `--recent`   | Show only repositories with a commit within a duration (e.g. `24h`, `7d`). |
| `--compact`  | Compact one-line layout (canonical report only).                  |
| `-r`         | Rich textual report.                                              |
| `-i`         | Interactive repository investigation.                             |
| `-version`   | Print the version (e.g. `keen v0.8.1`) and exit.                |
| `--help`     | Print usage.                                                      |

`-version` prints the release version and exits without scanning or inspecting any repository.

## Presentation modes

```text
keen        canonical textual report (default)
keen -r     rich textual report
keen -i     interactive repository investigation
```

Selecting a mode never changes which repositories are shown. `--clean`, `--dirty`, and `--recent` always control *which* repositories are selected; the mode controls *how* the selected set is rendered. All modes consume the same filtered set.

`--compact` is specific to the canonical renderer, so `keen -r --compact` and `keen -i --compact` fail with a usage error rather than being silently ignored.

## Interactive mode

`keen -i` presents six investigative surfaces:

| Surface        | Answers                                    | Shows                                                        |
| -------------- | ------------------------------------------ | ------------------------------------------------------------ |
| List           | Which repositories exist?                  | identity, status, branch, synchronization                     |
| Detail         | What is this repository?                   | name, path, status, branch, upstream, ahead, behind, last commit |
| Activity       | What happened recently in this repository? | repository, hash, subject, relative time (selected repo)    |
| Commit History | What happened before the latest commit?    | recent commits (hash, subject, author, date)                |
| Commit Detail  | Which commit am I looking at?              | full hash, subject, body, author, dates, parents             |
| Changed Files  | What files did this commit change?         | change status (A/M/D/R) and path(s)                          |

Commit History shows the latest **20 commits** of the selected repository.

List is an addressable index: each row carries one status tag, branch, and synchronization — enough to select the correct repository — with the selected row marked by a `▸` pointer and a reverse-video highlight on a terminal. Detail (not the List) carries the upstream path. Activity is contextual to the selected repository and shows its latest commit (`No commits` if none). The surfaces form a strict hierarchy (`List → Detail → Activity → Commit History → Commit Detail → Changed Files`): `Enter` descends one level, `←`/`Esc` ascend one level, and Changed Files is the deepest — pressing anything past it never wraps around to List.

| Key             | Action                                  |
| --------------- | --------------------------------------- |
| `↑`/`↓`         | Move selection                          |
| `Enter`         | Advance to the child surface (List → Detail → Activity → Commit History → Commit Detail → Changed Files) |
| `←`/`Esc`       | Return to the parent (Changed Files → Commit Detail → Commit History → Activity → Detail → List) |
| `q` / `Ctrl+C`  | Quit                                    |

A view wider than the terminal is revealed through a horizontal viewport — scroll with `Shift+←`/`Shift+→`. A persistent header (`‹ LIST › 1 / 6`, etc.) identifies the active view and position; the selection highlight tracks the viewport so the visible part of the selected row stays emphasized. The list scrolls vertically so the selected repository stays visible; filtering and sorting still determine list order.

When stdin is not a terminal (e.g. `keen -i < input`), `keen -i` prints a one-shot overview and exits, preserving script compatibility.

## Filtering

Filters combine using **AND** semantics: `--clean --dirty` selects all repositories; `--clean --recent 7d` selects repositories that are clean *and* recently active.

| Invocation                     | Selection               | Presentation |
| ------------------------------ | ----------------------- | ------------ |
| `keen`                         | all                     | grouped      |
| `keen --clean`                 | clean                   | grouped      |
| `keen --dirty`                 | dirty                   | grouped      |
| `keen --recent 7d`             | recent (last 7 days)    | grouped      |
| `keen --dirty --recent 7d`     | dirty AND recent        | grouped      |
| `keen --clean --dirty`         | all                     | grouped      |
| `keen --compact --recent 7d`   | recent                  | compact      |
| `keen -r --dirty`              | dirty                   | rich         |
| `keen -i --recent 7d`          | recent (last 7 days)    | interactive  |

Supported `--recent` units: `s` (seconds), `m` (minutes), `h` (hours), `d` (days), `w` (weeks). The duration must be positive.

## Output

Grouped mode is status-as-structure: a workspace summary line, then `CLEAN` / `DIRTY` section headings — each shown only when it contains at least one repository. The heading is the status signal, so rows carry no per-repo `[clean]`/`[dirty]` tag beneath it:

```text
===KEEN===

    2 repositories, 1 clean, 1 dirty

    CLEAN
    project-a  (main → origin/main) ↑0 ↓0  | 4e02b68 add temporal repository filtering | 2 days ago

    DIRTY
    project-b  (feature/x → origin/feature/x) ↑1 ↓0  | a1b2c3d fix parser panic | 5 minutes ago
```

Each row reports the branch, the configured upstream (`branch → upstream`), commits ahead/behind that upstream (`↑n ↓n`), the latest commit (seven-character short hash plus subject), and the commit's relative time. On narrow terminals the row drops the time, then the commit detail, before the base identity facts — which are never truncated. `--compact` is the flat variant: one line per repository, where the per-row `[clean]`/`[dirty]` tag is the status signal.

If no repositories exist, `keen` prints `No repositories found.` If repositories exist but none match the active filters, it prints `No repositories match the selected filters.`

### Rich report

`keen -r` presents the same facts as an aligned column table (name, branch, upstream, ahead, behind, hash, subject, time — status is carried structurally by the `CLEAN`/`DIRTY` section headings, with no `STATUS` column). The table adapts deterministically to terminal width: columns tighten first, lower-priority columns are dropped next (TIME, then SUBJECT, then ahead/behind as a pair), and on very narrow terminals `keen -r` renders the canonical grouped report. The same repository state at the same width always produces identical output.

## Repository identity

Keen displays repositories by their directory name. Repository names remain basenames when unique, and gain only the minimum parent path necessary when collisions exist among the currently displayed repositories:

```text
work/api
personal/api
```

Identity is resolved from the repositories currently being displayed, so filtering can reduce an expanded identity back to its basename. Unique names never change, and unrelated repositories are never affected by a collision elsewhere in the output.

## Known limitations

- A detached HEAD reports `detached` as its branch — expected, not an error.
- A repository inside another repository is not separately reported; traversal stops at the first discovered repository.
- A repository without an upstream reports `↑– ↓–` (a non-numeric marker, so `0/0` is not mistaken for a synchronized state); the configured upstream, when present, is shown as `branch → upstream`.
- A repository with no commits reports `No commits`.
