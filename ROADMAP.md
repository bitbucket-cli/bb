# Bitbucket CLI Roadmap

This document outlines planned features and improvements for the Bitbucket CLI (`bb`).

## Current Status

The CLI currently supports:
- **Authentication**: OAuth login/logout with secure token storage
- **Repositories**: List, view, clone, create, update, fork, delete
- **Pull Requests**: List, view, create, merge, approve, comment, diff
- **Branches**: List, view, create, delete, restrictions
- **Pipelines**: List, view, trigger, stop, steps, logs
- **Issues**: List, view, create, update, delete, comments
- **Tags**: List, view, create, delete
- **Webhooks**: List, view, create, delete
- **Deployments**: List environments, create, delete
- **Workspaces**: List, view, members, projects
- **Configuration**: Manage CLI settings
- **Shell Completions**: Bash, Zsh, Fish, PowerShell

## Open-Source Release Blockers

These must be resolved before a public release. See `review.md` for full details.

### Bugs (fix before release)

- [x] **Double body injection** — `repo.go` and `branch.go`: removed redundant `withBody` editor; body is now only set via the positional `io.Reader` arg.
- [x] **`DeleteTokens` calls `deleteTokenFile()` twice** — fixed; both keyring and file deletions now always run, errors ignored (both stores may legitimately be absent).
- [x] **`pkg/pr/pr.go` only returns the first page** — `pr.List` now follows `result.Next` pagination links.
- [x] **`branch list`, `pipeline list`, `pr list` missing pagination** — all three now paginate fully; `pipeline list` also has a `--limit` flag (default 25).
- [x] **Hash slice panic** — `branch.go`: now uses `h[:min(12, len(h))]`.
- [x] **Token refresh body reset** — `api/client.go`: body is snapshotted before the first attempt and restored on 401 retry.

### Documentation mismatches (fix before release)

- [x] **`bb auth login --client-id / --client-secret` flags** — flags now exist; README updated with shell-history warning.
- [x] **`BB_DEBUG=1` documented but not implemented** — removed from README.
- [x] **Module path** — updated to `github.com/ThomasRedstone/bb` across all files in both repos.

### Architecture (fix before release)

- [x] **`output.PrintSingle` map iteration is non-deterministic** — now sorts keys alphabetically before printing.
- [x] **Magic 50-row threshold in `output.Print`** — removed; always uses the table renderer.

### Polish (recommended before release)

- [x] **Propagate `cmd.Context()`** — all RunE functions now use `cmdCtx(cmd)` (a helper with nil fallback for tests) instead of `context.Background()`.
- [x] **`prInteractive` global variable shared across PR subcommands** — removed; each command reads `cmd.Flags().GetBool("interactive")` inside RunE.
- [x] **`fetchPage` in `repo.go` brittle triple-dereference** — replaced with `client.GetURL()` helper on `api.Client`.
- [x] **`client_secret` stored in plaintext** — security note added to README pointing users to `bb auth login` and env vars instead.
- [x] **Unicode symbols in `output.Success`/`output.Error`** — `✓`/`✗` now fall back to `OK`/`ERROR` when stdout is not a terminal.

---

## Planned Features

### High Priority

#### Git Integration
- [x] Auto-detect workspace/repo from current git directory — `internal/git.DetectRepo` (origin remote), used by `bb pr checkout`
- [x] `bb pr checkout <pr-number>` - checkout PR branch locally (repo auto-detected)
- [x] `bb pr create` from current branch without specifying source — `--source` defaults to the current git branch

#### Enhanced Interactive Mode
- [ ] Add `-i` flag to more commands (branch, issue, etc.)
- [ ] Interactive PR creation wizard
- [ ] Interactive branch selection for operations

#### Pipeline Improvements
- [ ] Stream pipeline logs in real-time
- [x] Pipeline variable management — `bb variable list/set/delete` (repo pipeline variables)
- [x] Schedule management — `bb pipeline schedule list/view/delete` (create deferred: target/selector body needs live-API verification)
- [x] Caches management — `bb pipeline cache list/delete` (artifacts download still pending)

### Medium Priority

#### Code Review Features
- [x] Inline comments support for PR reviews — `bb pr comment --path <file> --line <n>`
- [ ] PR review threads navigation
- [ ] Request changes / approve with comments
- [ ] PR templates support

#### Repository Management
- [ ] Repository settings management
- [x] Access keys (SSH keys) management — `bb deploy-key list/add/view/delete`
- [x] Repository variables for pipelines — `bb variable list/set/delete`
- [ ] Branch merge strategies configuration

#### Snippets Support
- [x] List snippets — `bb snippet list [workspace] [--role]`, paginated
- [x] Create snippets from files or stdin — `bb snippet create` (multipart, `--title`/`--private`/`--filename`)
- [x] View snippet content — `bb snippet view <workspace> <id>`
- [x] Delete snippets — `bb snippet delete <workspace> <id>`
- [x] Update snippets — `bb snippet update <workspace> <id> [files...]` (multipart PUT via Client.RequestRaw)

#### Downloads
- [x] List repository downloads — `bb download list <workspace/repo>`, paginated
- [x] Upload files to downloads — `bb download upload <workspace/repo> <files...>`
- [x] Delete downloads — `bb download delete <workspace/repo> <filename>`
- [x] Fetch a download — `bb download get <workspace/repo> <filename> [-o path]`

### Low Priority

#### Reports and Analytics
- [ ] Pipeline analytics and statistics
- [ ] Commit activity reports
- [ ] PR metrics (time to merge, etc.)

#### Bulk Operations
- [ ] Bulk PR operations (approve multiple, merge multiple)
- [ ] Bulk issue updates
- [ ] Export operations (PRs, issues to CSV/JSON)

#### Notifications
- [ ] List notifications
- [ ] Mark notifications as read
- [ ] Notification preferences

## Technical Improvements

### Testing
- [ ] Increase test coverage to 90%+
- [ ] Add integration tests with mock API server
- [ ] Add end-to-end tests

### Documentation
- [ ] Man pages generation
- [ ] Comprehensive README with examples
- [ ] Contributing guide
- [ ] API documentation

### Build and Distribution
- [x] GitHub Actions CI/CD pipeline (ci.yml + release.yml + .goreleaser.yaml)
- [x] Cross-platform binaries (Linux, macOS, Windows) — goreleaser builds amd64+arm64
- [x] Homebrew formula — goreleaser publishes to ThomasRedstone/homebrew-tap
- [ ] APT/RPM packages
- [ ] Scoop manifest for Windows

### Performance
- [ ] Response caching for frequently accessed data
- [ ] Concurrent API requests where applicable
- [ ] Lazy loading for large result sets

### Code Quality
- [ ] Golangci-lint integration
- [ ] Pre-commit hooks
- [ ] Semantic versioning
- [ ] Changelog automation

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests for any of the features listed above or suggest new ones.

## Version History

### v0.1.0 (Current)
- Initial release with core functionality
- Authentication, repositories, PRs, branches, pipelines, issues
- Tags, webhooks, deployments, workspaces
- Interactive mode for PR commands
- Shell completion support
- Configuration management
