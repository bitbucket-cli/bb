# v0.3.0-rc.1 — Public preview (draft)

This release is planned and unpublished. Commands and asset links below become
available after publication.

`bb` brings Bitbucket Cloud repositories, pull requests, pipelines, issues,
workspaces and related resources to the terminal. This release candidate opens
the project for feedback before a stable release.

## Installation

```bash
brew install bitbucket-cli/tap/bb
go install github.com/bitbucket-cli/bb/cmd/bb@v0.3.0-rc.1
```

Prebuilt archives for Linux, macOS and Windows (amd64 and arm64), plus Linux
`.deb` and `.rpm` packages, will be available on the
[preview release](https://github.com/bitbucket-cli/bb/releases/tag/v0.3.0-rc.1).
Choose the method appropriate for your platform. Archives and packages include
third-party licence notices; `checksums.txt` covers the release artifacts.

## Changes since the private releases

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

## Preview scope

The CLI targets Bitbucket Cloud. OAuth consumers and API tokens need permissions
for the commands you use; see [authentication](README.md#authentication).
Automated tests use mock API servers. Live Bitbucket end-to-end verification
needs account credentials and is not part of routine CI.

OAuth's default token storage includes an owner-only plaintext file mirror.
Select `token_store=keyring` if you want keyring-only persistence. OAuth client
secrets entered interactively are saved to the owner-only configuration file;
external credential commands or environment variables provide alternatives.

Report bugs and feature requests through
[GitHub issues](https://github.com/bitbucket-cli/bb/issues). Follow
[SECURITY.md](SECURITY.md) for private vulnerability reports and
[CONTRIBUTING.md](CONTRIBUTING.md) for development and release instructions.
