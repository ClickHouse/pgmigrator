Release a new version of pgmigrator.

Steps:
1. Run `git fetch --tags origin` and find the latest version tag (tags matching `v*`). If no tags exist, the current version is "none" and the next default is `v0.1.0`.
2. Parse the latest tag to determine current semver (major.minor.patch).
3. Ask the user what type of bump they want: patch, minor, or major. Show the current version and what each bump would produce.
4. After the user picks, confirm the exact new tag before proceeding. Show: "Will tag **current → next** and push. This triggers the release workflow. Proceed?"
5. Only after explicit confirmation: create the tag on HEAD and push it with `git tag <version> && git push origin <version>`.
6. Print the link to the GitHub Actions run: `https://github.com/ClickHouse/pgmigrator/actions`
