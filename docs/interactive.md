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
| **Changed Files** (`FILES`) | What files did this commit change? | Change status (`A`, `M`, `D`, `R`), path(s); renames show old and new path |

## Interactive Filtering

Press `/` while on the **List** surface to enter filter editing. Filter editing is a genuine text-entry state: the visible repository set filters against display identity and branch (case-insensitive substring; deterministic order preserved), and display identities re-resolve against the visible subset — filtering away one member of a name-collision group sheds the survivor's parent qualification until the query is cleared.
- **Typing**: Appends printable characters to the query, including characters bound to actions outside editing (`q`, `/`, `H`, `L`). Multi-byte characters (CJK, emoji, accented) arrive as single characters.
- **Backspace**: Removes the last query character (rune-safe).
- **`Esc`**: Clears the query and exits editing, restoring the full set.
- **`Enter`**: Exits editing while keeping the current query active; press `Enter` again to inspect the selected repository.
- **`Ctrl+C`**: Quits even while editing. A printable `q` is query text, not quit.
- While editing, the footer swaps to the editing keymap above, so the displayed bindings always match what the keys do.
- **Selection**: Never moves while editing; navigation keys are inert. Clearing restores a valid selection. A query matching nothing keeps the footer and an explicit empty state; `Enter` is then a harmless no-op.
- Underlying repository selection and CLI selection filters (`--clean`, `--dirty`, `--recent`) remain fully orthogonal.

## Selection & Scrolling

The selected row is identified by a reverse-video highlight (no pointer glyph). Selection stays visible across vertical and horizontal scrolling. Detail, Commit Detail, and Changed Files bodies scroll vertically with `↑`/`↓` within a bounded viewport so the footer keymap never leaves the visible terminal. Neither the breadcrumb header nor the footer ever scrolls horizontally; only body content moves under `H`/`L`.

## Footers

Every surface ends with a contextual keymap footer describing exactly the actions available there:

| Surface | Footer |
|---|---|
| List | `Enter detail    ↑/↓ select    / filter    q quit` |
| List (while filter editing) | `type to filter    Enter accept    Backspace delete    Esc clear    Ctrl+C quit` |
| Detail | `Enter history    ↑/↓ scroll    ←/Esc back    q quit` |
| History | `Enter commit    ↑/↓ select    ←/Esc back    q quit` |
| Commit | `Enter files    ↑/↓ scroll    ←/Esc back    q quit` |
| Files | `↑/↓ scroll    ←/Esc back    q quit` |

Bindings follow one canonical order — `Enter`, `↑/↓`, `/`, `←/Esc`, quit — on every surface; a missing entry means the action is unavailable there, never a reordering.

The header breadcrumb carries the surface position right-aligned (`1 / 5` on List through `5 / 5` on Files); over-wide titles truncate to keep the indicator on a single line.

Horizontal scrolling (`H`/`L`) is deliberately undisclosed in the footers: at narrow terminal widths a longer footer would wrap and break the stable chrome geometry, so the keys live in this guide instead of consuming permanent footer space.

## Non-TTY Behavior

When stdin is not a terminal (e.g. `keen -i < input`), `keen -i` automatically degrades to a one-shot static list render with zero terminal control sequences or escape codes, preserving full script and CI compatibility.