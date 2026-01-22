# gitstreams

Track what your GitHub social network has been up to. Get desktop notifications
and a rich HTML report showing repos starred, new projects, and activity from
developers you follow.

## Installation

### From source

```bash
go install github.com/justinabrahms/gitstreams@latest
```

Or clone and build:

```bash
git clone https://github.com/justinabrahms/gitstreams.git
cd gitstreams
go build -o gitstreams .
```

## Usage

```bash
# Set your token
export GITHUB_TOKEN=your_token_here

# Run
gitstreams
```

### CLI Flags

| Flag | Description |
|------|-------------|
| `-token` | GitHub token (default: `$GITHUB_TOKEN`) |
| `-db` | Path to SQLite database (default: `~/.gitstreams/gitstreams.db`) |
| `-report` | Path to write HTML report (default: temp file) |
| `-days` | Number of days to look back for activity (1-365, default: 30) |
| `-offline` | Skip GitHub API sync and use cached data |
| `-no-notify` | Skip desktop notification |
| `-no-open` | Don't open report in browser |
| `-v` | Verbose output |

### Examples

```bash
# Run quietly, just generate report
gitstreams -no-notify -no-open -report ~/reports/today.html

# Verbose mode with custom database
gitstreams -v -db /path/to/my.db

# Only show activity from the last 7 days
gitstreams -days 7

# Use cached data without hitting GitHub API (fast, but may be stale)
gitstreams -offline
```

## HTML Report

The generated report includes:

- **Summary stats** — stars, new repos, PRs, forks, pushes, issues at a glance
- **Highlight of the day** — featured activity (prioritizes new repos and PRs)
- **Dual view toggle** — switch between "By Category" and "By User" groupings
- **Collapsible sections** — expand/collapse each category or user
- **Activity icons** — ⭐ stars, 🆕 repos, 🔀 PRs, 🔱 forks, 📤 pushes, 🐛 issues
- **Hot activity badges** — 🔥 marks high-engagement actions (new repos, PRs)
- **MVP badge** — 🏆 highlights the most active user
- **Relative timestamps** — "2 hours ago", "yesterday", "last week"
- **Fun taglines** — dynamic header message based on activity volume

## Development

### Git Hooks (Recommended)

This project uses [lefthook](https://github.com/evilmartians/lefthook) for Git hooks to catch lint errors before they're pushed.

**Install lefthook:**

```bash
# macOS
brew install lefthook

# Go
go install github.com/evilmartians/lefthook@latest

# Or see: https://github.com/evilmartians/lefthook#installation
```

**Enable hooks:**

```bash
lefthook install
```

The hooks will:
- **Pre-commit**: Run fast checks (gofmt, goimports, go vet)
- **Pre-push**: Run full CI suite (golangci-lint, tests, build, mod tidy)

**Dependencies:**

```bash
# Install goimports (for import formatting)
go install golang.org/x/tools/cmd/goimports@latest

# Install golangci-lint (for comprehensive linting)
# See: https://golangci-lint.run/welcome/install/
brew install golangci-lint
```

## Requirements

- Go 1.22+
- GitHub personal access token with `read:user` scope
- macOS for notifications (optional — use `-no-notify` elsewhere)

## License

MIT
