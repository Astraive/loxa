# Tools

Shared tooling for the LOZA monorepo.

| Tool | Description | Usage |
|------|-------------|-------|
| `check-doc-links.sh` | Validates all markdown links resolve to existing files | `./tools/check-doc-links.sh` |
| `check-kebab-case.sh` | Checks doc filenames use kebab-case | `./tools/check-kebab-case.sh` |
| `generate-parity-report.py` | Reads parity manifests from all SDKs, produces cross-SDK matrix | `./tools/generate-parity-report.py` |
| `update-changelog.sh` | Parses git log since last tag, generates changelog entries | `./tools/update-changelog.sh` |
