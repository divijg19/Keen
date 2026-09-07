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
- **`Shift+←` / `Shift+→`**: Scroll horizontally across wide viewports.

## Surfaces

| Surface | Answers | Shows |
|---|---|---|
| **List** (`REPOSITORIES`) | Which repositories exist? | Identity, working-tree status, branch, synchronization (`↑n ↓n`) |
| **Detail** (`DETAIL`) | What is this repository? | Name, absolute path, status, branch, upstream, ahead/behind, latest commit |
| **Commit History** (`HISTORY`) | What happened recently? | Bounded recent commit history (latest 20 commits: hash, subject, author date) |
| **Commit Detail** (`COMMIT`) | Which commit am I inspecting? | Full hash, subject, author, committer, dates, parents, body |
| **Changed Files** (`FILES`) | What files did this commit change? | Change status (`A`, `M`, `D`, `R`), path(s), and change magnitude (`+N −N`) |

## Interactive Filtering

Press `/` while on the **List** surface to enter filter mode. Type a substring to filter the visible repository set against display identity and branch.
- **`Esc`**: Clear active filter or exit filter mode.
- **`Enter`**: Exit filter mode while keeping filter active.
- Underlying repository selection and CLI selection filters (`--clean`, `--dirty`, `--recent`) remain fully orthogonal.

## Non-TTY Behavior

When stdin is not a terminal (e.g. `keen -i < input`), `keen -i` automatically degrades to a single one-shot static list render with zero terminal control sequences or escape codes, preserving full script and CI compatibility.
