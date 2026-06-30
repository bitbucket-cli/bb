# Contributing to bb

Thanks for your interest in improving the Bitbucket CLI.

## Development setup

```bash
git clone https://github.com/ThomasRedstone/bb
cd bb
go build ./cmd/bb
```

## Before you open a PR

Run the same checks CI runs:

```bash
go build ./...
go vet ./...
go test ./...
golangci-lint run ./...    # config in .golangci.yml
gofmt -l .                 # should print nothing
```

A pre-commit config is provided — `pre-commit install` will run these automatically.

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
