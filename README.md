# Keen

`keen` is a quiet, deterministic, read-only workspace orientation and repository investigation CLI. It recursively discovers Git repositories beneath the current working directory and reports their status — clean/dirty, branch, upstream synchronization, and recent commits.

## Installation

```bash
go install github.com/divijg19/Keen/cmd/keen@latest
```

## Quick Start

```bash
keen              # Canonical grouped report (clean before dirty)
keen -r           # Rich columnar report adapting to terminal width
keen -i           # Interactive repository investigation
```

## Presentation Modes & Flags

| Mode / Flag | Description |
|---|---|
| `keen` | Grouped report with workspace summary and structural status headings. |
| `keen --compact` | Compact one-line per repository layout with per-row status tags. |
| `keen -r` | Rich columnar table with intelligent responsive width tiers. |
| `keen -i` | Interactive terminal UI for deep repository and commit investigation. |
| `--clean` / `--dirty` | Select only clean or dirty repositories. |
| `--recent <dur>` | Select repositories with recent commits (e.g., `24h`, `7d`, `2w`). |

## Interactive Investigation (`keen -i`)

Keen provides a fast, keyboard-driven investigation surface across five hierarchical views:
1. **List** (`REPOSITORIES`) — Addressable repository index supporting `/` filtering.
2. **Detail** (`DETAIL`) — Full repository metadata and working-tree state.
3. **Commit History** (`HISTORY`) — Bounded recent commit history (latest 20 commits).
4. **Commit Detail** (`COMMIT`) — Commit metadata, author details, parents, and body.
5. **Changed Files** (`FILES`) — Touched paths, change status, and change magnitude (`+N −N`).

## Documentation

Comprehensive guides are available in `docs/`:
- [Usage Guide](docs/usage.md) — Flags, filtering semantics, duration syntax, and invocation rules.
- [Interactive Guide](docs/interactive.md) — Surface hierarchy, navigation keymaps, and filtering.
- [Output Guide](docs/output.md) — Canonical, compact, and rich output formats, responsive tiers, and Unicode cell-width layout.

## License

MIT
