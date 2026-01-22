# Releasing gitstreams

This document describes the release process for gitstreams.

## Versioning

gitstreams uses [semantic versioning](https://semver.org/):

- **MAJOR** (v2.0.0): Breaking changes to CLI flags, output format, or behavior
- **MINOR** (v1.1.0): New features, new flags, non-breaking enhancements
- **PATCH** (v1.0.1): Bug fixes, documentation updates, performance improvements

## When to Release

- **Patch release**: Bug fix that users need
- **Minor release**: New feature is complete and tested
- **Major release**: Breaking change is necessary (rare)

## How to Release

1. Ensure `main` is in a releasable state (CI passing, tests green)

2. Create and push a tag:

   ```bash
   git checkout main
   git pull
   git tag v1.0.0
   git push origin v1.0.0
   ```

3. The GitHub Actions workflow automatically:
   - Builds binaries for macOS (arm64, amd64) and Linux (amd64)
   - Creates a GitHub Release with auto-generated release notes
   - Uploads binaries and checksums

4. Verify the release at <https://github.com/justinabrahms/gitstreams/releases>

## Build Artifacts

Each release includes:

| File | Platform |
|------|----------|
| `gitstreams-darwin-arm64` | macOS Apple Silicon |
| `gitstreams-darwin-amd64` | macOS Intel |
| `gitstreams-linux-amd64` | Linux x86_64 |
| `checksums.txt` | SHA256 checksums |

## Version Information

Binaries include embedded version info:

```bash
gitstreams -version
# gitstreams v1.0.0 (commit: abc123, built: 2026-01-22)
```

## Local Build with Version

To build locally with version info:

```bash
go build -ldflags "-X main.version=v1.0.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%d)" .
```
