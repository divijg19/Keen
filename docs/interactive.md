# Keen Interactive Investigation Guide

Running `keen -i` opens the interactive repository investigation terminal UI.

## Navigation Hierarchy

The investigation model is strict, hierarchical, and non-cyclic:

```text
List (REPOSITORIES)
  ↓ Enter
Detail
  ↓ Enter
Commit History (HISTORY)
  ↓ Enter
Commit Detail (COMMIT)
  ↓ Enter
Changed Files (FILES)
```

- **Enter**: Advance to child surface.
- **`←` / `Esc`**: Return to parent surface.
- **`q` / `Ctrl+C`**: Quit from any level.
- **`↑` / `↓`**: Move selection on List and History; scroll Detail, Commit Detail, and Changed Files content.
- **`H` / `L` / `Shift+←` / `Shift+→`**: Scroll horizontally across wide viewports.

## Surfaces

| Surface | Answers | Shows |
|---|---|---|
| **List** (`REPOSITORIES`) | Which repositories exist? | Identity, working-tree status, branch, synchronization (`↑n ↓n`) |
| **Detail** (`DETAIL`) | What is this repository? | Name, absolute path, status, branch, upstream, ahead/behind, latest commit |
| **Commit History** (`HISTORY`) | What happened recently? | Bounded recent commit history (latest 20 commits: hash, subject, author date); footer reads `Enter commit` |
| **Commit Detail** (`COMMIT`) | Which commit am I inspecting? | Full hash, subject, author, committer, dates, parents, body |
| **Changed Files** (`FILES`) | What files did this commit change? | Change status (`A`, `M`, `D`, `R`), path(s), and change magnitude (`+N −N`) |

## Interactive Filtering

Press `/` while on the **List** surface to enter filter editing. Type to filter the visible repository set against display identity and branch (case-insensitive substring; deterministic order preserved).
- **Typing**: Appends printable characters to the query.
- **Backspace**: Removes the last query character.
- **`Esc`**: Clears the query and exits editing, restoring the full set.
- **`Enter`**: Exits editing while keeping the current query active; press `Enter` again to inspect the selected repository.
- **`q` / `Ctrl+C`**: Quits even while editing (a literal `q` cannot be typed into a query).
- **Selection**: Never moves while editing; clearing restores a valid selection.
- Underlying repository selection and CLI selection filters (`--clean`, `--dirty`, `--recent`) remain fully orthogonal.

## Selection & Scrolling

The selected row is identified by a reverse-video highlight (no pointer glyph). Selection stays visible across vertical and horizontal scrolling. Detail, Commit Detail, and Changed Files bodies scroll vertically with `↑`/`↓` within a bounded viewport so the footer keymap never leaves the visible terminal.

## Non-TTY Behavior

When stdin is not a terminal (e.g. `keen -i < input`), `keen -i` automatically degrades to a one-shot static list render with zero terminal control sequences or escape codes, preserving full script and CI compatibility.