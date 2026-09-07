# Keen Output & Rendering Guide

Keen provides three presentation modes sharing a single deterministic pipeline.

## 1. Canonical Output (`keen`)

Grouped by working-tree status (`CLEAN` before `DIRTY`), ordered deterministically by status → name → path.
- **Workspace Summary**: A one-line orientation summary preceding the report (e.g., `3 repositories, 2 clean, 1 dirty`).
- **Status as Structure**: Section headings (`CLEAN`, `DIRTY`) carry the primary status signal; rows carry no redundant per-repo status tags.
- **Responsive Squeezing**: On narrow terminals, relative time and commit details drop gracefully before base identity facts.

## 2. Compact Output (`keen --compact`)

Dense one-line representation optimized for fast scanning or script-adjacent human use.
- Retains per-row status tags (`[clean]`, `[dirty]`) as an immediate row-level status signal.

## 3. Rich Output (`keen -r`)

Columnar aligned report presenting the same information semantics as interactive views.
- **Explicit Width Tiers**:
  - **Wide**: `NAME | BRANCH | UPSTREAM | AHEAD | BEHIND | HASH | SUBJECT | TIME`
  - **Medium**: Compact comparison schema
  - **Narrow / Fallback**: Automatic silent degradation to canonical grouped output below minimum width threshold.
- Tested across 12 explicit terminal widths (40 to 200).

## 4. Repository Identity & Unicode

- **Collision-Aware Display Identities**: Repositories are shown by basename (`api`), gaining minimal parent qualification only when name collisions occur (`work/api`, `personal/api`).
- **Terminal-Cell-Aware Layout**: Built-in support for CJK, Hangul, Fullwidth forms, emoji, and combining characters via cell-width measurement (`stringCellWidth`, `truncateCells`, `padCells`).
