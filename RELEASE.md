# Release Process

## Quick release (via Claude Code)

```
/release
```

This will prompt for the bump type (patch/minor/major), confirm the new version, tag, and push. The GitHub Actions release workflow handles the rest.

## Manual release

1. Decide the next version (follows [semver](https://semver.org/)):

   ```
   git fetch --tags origin
   git tag --sort=-v:refname | head -1   # current version
   ```

2. Tag and push:

   ```
   git tag v0.2.0
   git push origin v0.2.0
   ```

3. The `release.yml` workflow will:
   - Build binaries for Linux (x86_64, arm64) and macOS (arm64)
   - Generate SHA256 checksums
   - Create a GitHub release with auto-generated notes from commits

## Artifacts

Each release produces:

| File | Description |
|------|-------------|
| `pgmigrator-v*-linux-amd64.tar.gz` | Linux x86_64 binary |
| `pgmigrator-v*-linux-arm64.tar.gz` | Linux arm64 binary |
| `pgmigrator-v*-darwin-arm64.tar.gz` | macOS arm64 binary |
| `checksums.txt` | SHA256 checksums for all archives |
