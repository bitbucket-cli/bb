# Contributing to bb

Thanks for your interest in improving the Bitbucket CLI.

## Development setup

```bash
git clone https://github.com/bitbucket-cli/bb
cd bb
go build ./cmd/bb
```

## Before you open a PR

Run the same checks CI runs:

```bash
go build ./...
go vet ./...
go test -race ./...
make test-coverage
golangci-lint run ./...    # config in .golangci.yml
gofmt -l .                 # should print nothing
```

A pre-commit config is provided — `pre-commit install` runs format, build, vet, lint and tests. Install `golangci-lint` v2.11.4 to match CI. CI additionally checks race detection, package coverage ratchets, release binary vulnerabilities, third-party notice freshness and native macOS/Windows smoke tests.

## Adding a command

Commands live in `cmd/bb/<resource>.go` and call the OpenAPI-generated client in
`generated/bitbucket` via `pkg/bitbucket.Client`. Mirror an existing command
(e.g. `webhook.go`) for structure:

- a parent `*cobra.Command` plus `list`/`view`/`create`/`delete` subcommands,
- a mockable client factory var (`var fooNewAPIClient = api.NewClient`) so tests
  can point it at an `httptest` server via `mockAPIClientFactory`,
- table + JSON output through `internal/output`,
- pagination via the `next` link where the endpoint is paginated.

For endpoints the generated client doesn't cover, use `Client.Request` (JSON) or
`Client.RequestRaw` (arbitrary body) on `pkg/bitbucket`.

Document the command thoroughly in its cobra `Short`/`Long` — that's the source
for `--help` and the generated man pages (`bb man <dir>`).

## Tests

Every command should have an httptest-backed test covering its happy path and
the no-client error path. Keep `golangci-lint run` clean.

## Commit messages

Use a `area: summary` prefix (e.g. `pr: add request-changes`). Releases are cut
with goreleaser; `docs:`/`test:`/`chore:` commits are excluded from the changelog.

## Dependency notices and releases

After changing a dependency or the Go toolchain, run `make licenses` and review
and commit `THIRD_PARTY_LICENSES.txt`. The generator collects licence and notice
files from every dependency linked by the six CGO-disabled release targets,
including platform-specific dependencies, and the selected Go runtime. It fails
when a module has no discoverable licence; resolve that module's attribution
manually before releasing. These notices are included in binary archives and
Linux packages. Extra upstream notices may cover unlinked files.

CI reads the toolchain from `go.mod`; keep its toolchain directive on a supported,
patched Go release. Coverage checks use `go-test-coverage` v2.18.3 and package
thresholds in `.testcoverage.yml`. Raise the ratchets when coverage improves.
Generated code and the release notice generator are excluded from the ratchet.

Tagged releases rerun CI against the tagged commit before publishing. GoReleaser
is pinned to v2.18.1; validate changes with `goreleaser check` and a local
`goreleaser release --snapshot --clean --skip=publish` in a clean checkout.
Homebrew stays a formula for macOS and Linux (amd64 and arm64). After GoReleaser
successfully publishes release assets, the workflow runs `scripts/homebrew`,
which verifies each local archive against `dist/checksums.txt` and generates the
formula. The tap publishing job runs only when the source repository is public; a public
tap cannot install private release assets. While private, formula generation and
validation still run and the generated artifact stays in the private workflow.
A separate workflow job retrieves that formula as a workflow artifact and
updates only `bb.rb` in the tap, preserving
an existing root or `Formula/` layout. A new tap uses `Formula/bb.rb`; duplicate
layouts fail rather than choosing an ambiguous destination. Identical formulas
produce no commit. If tap publishing fails, rerun only the failed Homebrew job
within the formula artifact's 14-day retention period; the release job does not
need to run again.

Set the `HOMEBREW_TAP_GITHUB_TOKEN` repository secret to a token with write access
to the tap. Set the `HOMEBREW_TAP_REPOSITORY` repository variable to change its
`owner/repository`; the default is `bitbucket-cli/homebrew-tap`. Release URLs
follow the releasing GitHub repository, so moving to an organisation only
requires configuring the tap destination and its token. The source module and
installation documentation still need updating separately if their names move.

The generator writes local files and has no publishing operation. To inspect a
formula after a snapshot without touching a tap:

```bash
go run ./scripts/homebrew -version v0.2.0 -repository bitbucket-cli/bb
ruby -c dist/homebrew/bb.rb
go test ./scripts/homebrew
```

Use the intended release tag for `-version`. Snapshot archives are suitable for
checking formula layout and checksums; their generated release URLs need an
actual published tag before users can install them. The generator rejects
invalid versions/repository names, missing or duplicate checksum entries and
archive checksum mismatches. No local validation command publishes a release.


## Public preview releases

Use a prerelease tag such as `v0.3.0-rc.1` for a public preview. GoReleaser marks
prerelease tags as GitHub prereleases, labels their title "Public preview", and
keeps them separate from the latest stable release. The Homebrew formula retains
the same prerelease version and downloads assets from that exact tag.

Finish and validate changes while the source is private. Once any publication
requirements are resolved, make the source public before pushing the release
tag: the public-tap guard uses the visibility recorded in the tag push event.
Rerunning a workflow originally triggered while private does not change that
original event. Retain old unsafe/private-era artifacts as draft releases until
they have been rebuilt and checked; drafting preserves their assets and tags.
Point installation instructions at the preview tag explicitly because both
GitHub's latest stable release endpoint and `go install ...@latest` omit previews.

## Generated API client provenance

Run `make generate` from the repository root to fetch the official schema pinned
in `scripts/apigen/source.json`, verify its SHA-256, strip descriptive prose and
examples, apply the structural compatibility patch, and regenerate the client
with oapi-codegen v2.5.1. The raw input and intermediate schema are temporary.
Only the generated Go client is committed; normal builds and `go install` do not
fetch schemas or run generation.

For a previously downloaded copy, use
`go run ./scripts/apigen -schema /path/to/swagger.v3.json`; verification remains
mandatory. A changed upstream download fails without replacing the client.
See [generation instructions](scripts/apigen/README.md) for updating the pin and
reviewing compatibility. The compatibility patch preserves the established API
structure because the current upstream schema omits APIs the CLI still uses.

The original raw schema and generated descriptions remain in historical Git
commits. Structural API definitions also remain in the patch and generated code.
Do not claim that removing the bundled raw schema establishes redistribution
permission; see [provenance](SPECIFICATION_PROVENANCE.md).
