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

Columnar aligned report presenting the same information semantics as interactive views. Columns drop by information value, not merely geometry: a field that fits but carries almost no usable content is removed.
- **Explicit Width Tiers** (terminal widths, four-space indent included):
  - **Wide (≥95)**: `NAME | BRANCH | UPSTREAM | AHEAD | BEHIND | HASH | SUBJECT | TIME`, proportionally allocated.
  - **Tight (68–94)**: Same eight columns on tightened floors.
  - **Seven-column (61–67)**: `TIME` dropped; `SUBJECT` kept on a 12-cell floor.
  - **Six-column (50–60)**: `TIME` and `SUBJECT` dropped; identity, branch, upstream, sync, and hash remain.
  - **Fallback (<50)**: Automatic silent degradation to the byte-identical canonical grouped output.
- `AHEAD`/`BEHIND` drop atomically, never half a divergence fact. `NAME` is never dropped. `SUBJECT` needs at least 12 cells to be useful, so narrower budgets drop it instead of squeezing a fragment.
- Tested across explicit terminal widths (40 to 200) at behavioral boundaries.

## 4. Repository Identity & Unicode

- **Collision-Aware Display Identities**: Repositories are shown by basename (`api`), gaining minimal parent qualification only when name collisions occur (`work/api`, `personal/api`). Identities resolve after filtering, so interactive filtering re-resolves against the visible subset.
- **Terminal-Cell-Aware Layout**: Display-cell-aware layout for ordinary ASCII and common wide terminal characters (CJK, Hangul, Fullwidth forms, common emoji) via cell-width measurement (`stringCellWidth`, `truncateCells`, `padCells`, `padStartCells`); the horizontal viewport (`sliceViewport`) works in display cells. Wide characters straddling a viewport or truncation edge are skipped rather than split; zero-width and combining characters ride with their cell position. Keen does not implement a complete Unicode grapheme-cluster shaping engine, and no external Unicode dependency is used.
