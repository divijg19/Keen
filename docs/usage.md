# Keen Usage Guide

`keen` recursively discovers Git repositories beneath the current working directory and reports their status.

## Installation

```bash
go install github.com/divijg19/Keen/cmd/keen@latest
```

## Command-Line Flags

| Flag | Description |
|---|---|
| `--clean` | Show only clean repositories. |
| `--dirty` | Show only dirty repositories. |
| `--recent <duration>` | Show only repositories with commits within duration (e.g. `30m`, `24h`, `7d`, `2w`). |
| `--compact` | Compact one-line layout (canonical report only). |
| `-r` | Rich textual report (columnar aligned table). |
| `-i` | Interactive repository investigation. |
| `-version` | Print release version and exit. |
| `--help` | Print usage. |

## Presentation Modes

```text
keen        canonical textual report (default, grouped by clean/dirty)
keen -r     rich textual report (columnar table adapting to terminal width)
keen -i     interactive repository investigation
```

Selecting a mode never changes which repositories are shown. `--clean`, `--dirty`, and `--recent` control selection; presentation modes control rendering. All modes share the same filtered pipeline: `Discover → Enrich → Sort → Filter → ResolveDisplayIdentities → Present`.

`--compact` is specific to the canonical renderer. Combining `--compact` with `-r` or `-i` produces a usage error rather than being silently ignored.

## Filtering & Composition

Selection filters combine using **AND** semantics:
- `keen --clean --dirty` selects all repositories.
- `keen --clean --recent 7d` selects clean repositories active within the last 7 days.

Supported `--recent` duration units: `s` (seconds), `m` (minutes), `h` (hours), `d` (days), `w` (weeks). Durations must be positive.