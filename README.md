# gitstreams

A local tool that tracks what your GitHub social network has been up to and
surfaces interesting activity via daily desktop notifications with an HTML
report.

## What it does

- Fetches activity from people you follow on GitHub (repos starred, repos
  created, contribution patterns, etc.)
- Stores snapshots locally to detect changes over time
- Runs daily via launchd and sends a desktop notification when there's something
  interesting
- Opens a local HTML report with details

## Status

Early exploration. "Interesting" is not yet defined - that's part of what we're
figuring out.

## Requirements

- Go 1.21+
- GitHub personal access token
- macOS (for launchd scheduling and notifications)

## Setup

```bash
# Clone
git clone https://github.com/justinabrahms/gitstreams.git
cd gitstreams

# Configure
export GITHUB_TOKEN=your_token_here

# Build
go build -o gitstreams .

# Run manually
./gitstreams
```

## Architecture

- **Data source**: GitHub API via personal access token
- **Storage**: SQLite for historical snapshots
- **Scheduling**: launchd plist (macOS)
- **Notifications**: `terminal-notifier` or `osascript`
- **Output**: Local HTML report

## License

MIT
