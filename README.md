# Keen

Keen is a quiet, read-only workspace-orientation and repository-investigation CLI. It recursively discovers Git repositories beneath the current working directory and provides progressively deeper information about them.

## Installation

```bash
go install github.com/divijg19/Keen/cmd/keen@latest
```

## Usage

```bash
keen              # Canonical grouped report (clean before dirty)
keen -r           # Rich columnar report adapting to terminal width
keen -i           # Interactive repository investigation
```

## Modes

| Mode | Description |
|------|-------------|
| `keen` | Grouped report with CLEAN/DIRTY section headings |
| `keen --compact` | One-line per repository with per-row `[clean]`/`[dirty]` tags |
| `keen -r` | Rich columnar table with width-aware tiers |
| `keen -i` | Interactive investigation across five surfaces (List → Detail → History → Commit → Files) |

## Selection

```bash
keen --clean          # Clean repositories only
keen --dirty          # Dirty repositories only
keen --recent 7d      # Commits within duration (s, m, h, d, w)
```

## Example

```text
===KEEN===

2 repositories, 1 clean, 1 dirty

    CLEAN
    api             (main → origin/main) ↑0 ↓0 | a1b2c3d initial commit | 2 hours ago

    DIRTY
    web             (dev) ↑– ↓– | e4f5g6h work in progress | 3 days ago
```

## Documentation

- [Usage Guide](docs/usage.md) — CLI flags, filtering, duration syntax
- [Interactive Guide](docs/interactive.md) — Five surfaces, navigation, keymaps
- [Output Guide](docs/output.md) — Renderer formats, responsive behavior

## License

MIT
