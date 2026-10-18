# v0.3.0-rc.1 — Public preview

`bb` brings Bitbucket Cloud repositories, pull requests, pipelines, issues,
workspaces and related resources to the terminal. This release candidate opens
the project for feedback before a stable release.

## Installation

```bash
brew install bitbucket-cli/tap/bb
go install github.com/bitbucket-cli/bb/cmd/bb@v0.3.0-rc.1
```

Prebuilt archives for Linux, macOS and Windows (amd64 and arm64), plus Linux
`.deb` and `.rpm` packages, are available on the
[preview release](https://github.com/bitbucket-cli/bb/releases/tag/v0.3.0-rc.1).
Choose the method appropriate for your platform. Archives and packages include
third-party licence notices; `checksums.txt` covers the release artifacts.

## Changes since the private releases

- Restored default keyring plus plaintext recovery mirroring, prominently disclosed
  in README while disappearance remains under investigation.
- Pagination audit fixes cover keys, variables, hooks, permissions, reviewers,
  tasks, reports and branch selection. Regression tests cross 100 workspace PRs
  and reject incomplete results rather than silently producing wrong counts.

- Hardened OAuth callbacks, credential handling, token storage permissions and
  authenticated API request destinations.
- Improved positional argument checks, repository inference and global output
  handling. Expanded regression tests cover pagination and command behavior.
- Builds use the patched Go toolchain recorded in `go.mod`. Tagged releases must
  pass CI, race tests, coverage ratchets, linting and binary vulnerability checks.
- A separate Homebrew formula publisher preserves macOS and Linux support and
  checks each release archive against its checksum before updating the tap.
- Dependency notice generation and the API client generator are reproducible
  using the documented development commands.
- Explicit API client regeneration downloads a checksum-pinned official schema
  into temporary storage and strips descriptive prose/examples. A structural
  compatibility patch preserves the existing client; ordinary builds use its
  committed Go code and do not download the schema.

## Preview scope

The CLI targets Bitbucket Cloud. OAuth consumers and API tokens need permissions
for the commands you use; see [authentication](README.md#authentication).
Automated tests use mock API servers. Live Bitbucket end-to-end verification
needs account credentials and is not part of routine CI.

OAuth's default token storage uses the keyring plus an owner-only plaintext
recovery mirror at `~/.config/bb/tokens.json`. Keyring loss remains under
investigation; choose `token_store=keyring` to opt out of file mirroring/fallback.
Existing explicit storage choices are preserved. OAuth client
secrets entered interactively are saved to the owner-only configuration file;
external credential commands or environment variables provide alternatives.

Report bugs and feature requests through
[GitHub issues](https://github.com/bitbucket-cli/bb/issues). Follow
[SECURITY.md](SECURITY.md) for private vulnerability reports and
[CONTRIBUTING.md](CONTRIBUTING.md) for development and release instructions.
