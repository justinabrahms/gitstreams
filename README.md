# gitstreams

[![CI](https://github.com/justinabrahms/gitstreams/actions/workflows/ci.yml/badge.svg)](https://github.com/justinabrahms/gitstreams/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/justinabrahms/gitstreams/graph/badge.svg)](https://codecov.io/gh/justinabrahms/gitstreams)

Track what your GitHub social network has been up to. Get desktop notifications
and a rich HTML report showing repos starred, new projects, and activity from
developers you follow.

## What it looks like
<img width="798" height="842" alt="Screenshot 2026-01-22 at 9 01 30 AM" src="https://github.com/user-attachments/assets/573e5aa3-0126-48b8-bb6f-bde1be203293" />

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
| `-sync-lookback-days` | How far back to fetch GitHub data (1-365 days, default: 30) |
| `-report-since` | Generate report from historical data starting from this date (e.g., `2026-01-15` or `7d` for 7 days ago) |
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

# Fetch GitHub data from the last 7 days
gitstreams -sync-lookback-days 7

# Generate report from historical cached data (last 7 days)
gitstreams -report-since 7d

# Generate report from a specific date using cached data
gitstreams -report-since 2026-01-15 -offline

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

## OpenTelemetry Instrumentation (Optional)

gitstreams includes optional OpenTelemetry instrumentation to monitor sync operation performance. Enable it by setting:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318
```

See [OTEL_SETUP.md](OTEL_SETUP.md) for detailed setup instructions, including Docker Compose configuration for a local collector and Jaeger UI.

Benefits:
- Monitor sync operation duration
- Track per-user API call timings
- Analyze pagination patterns
- Identify performance bottlenecks

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
