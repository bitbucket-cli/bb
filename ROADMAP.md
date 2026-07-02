# Bitbucket CLI Roadmap

This document outlines planned features and improvements for the Bitbucket CLI (`bb`).

## Current Status

The CLI currently supports:
- **Authentication**: OAuth login/logout (selectable scope levels) with secure token storage; API-token fallback
- **Repositories**: List, view, clone, create, update, fork, delete
- **Pull Requests**: List (incl. `--all` cross-repo, per-user), view, create (source defaults to current branch), merge, approve, request-changes, decline, comment (incl. inline), diff, checkout, **checks** (build/CI statuses)
- **Branches**: List, view, create, delete, restrictions
- **Commits**: List (by branch/ref), view, activity, **status** (build/CI), **report** (Code Insights)
- **Pipelines**: List, view, trigger, stop, steps, logs, analytics; schedules (list/view/delete); caches (list/delete); variables (list/set/delete)
- **Issues**: List, view, create, update, delete, comments, milestones/versions/components, bulk
- **Tags**: List, view, create, delete
- **Webhooks**: List, view, create, delete (repo **and workspace** level)
- **Deploy keys**: List, add, view, delete
- **Snippets**: List, view, create, update, delete
- **Downloads**: List, upload, get, delete
- **Deployments**: List deployments, environments (list/create/delete)
- **Projects**: List, view, create, delete
- **Workspaces**: List, view, members, projects; workspace-level webhooks + pipeline variables
- **Users**: Current user (whoami), profile lookup
- **Account keys**: SSH keys and GPG keys (list/add/delete)
- **Search**: Workspace-wide code search
- **Configuration**: Manage CLI settings; `client_secret_command` for external secret stores
- **Shell Completions**: Bash, Zsh, Fish, PowerShell
- **Git integration**: Auto-detect workspace/repo from the origin remote; every repo-scoped command takes an optional positional / `-R`

All major Bitbucket Cloud REST API groups are now covered. The remaining surface
is a low-value / niche long tail:

- **Repository settings *write*** — branching-model/permissions are read-only (the
  branching-model settings PUT has a large nested body; deferred as low-value).
- **Project sub-resources** — project-level default reviewers, deploy keys,
  permissions, and branch restrictions (niche admin; `bb project` covers core CRUD).
- **Pipeline step artifact download** — list/delete of caches exists; artifact
  download pending.
- **Not feasible (no Bitbucket Cloud API)**: notifications, real-time log
  streaming, merge-strategy configuration.

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
- [x] Add `-i` flag to more commands — `bb branch delete -i` (interactive branch picker)
- [x] Interactive PR creation wizard — `bb pr create -i` prompts for the title
- [x] Interactive branch selection for operations — `bb branch delete` prompts when the name is omitted

#### Pipeline Improvements
- [~] Stream pipeline logs in real-time — not feasible: the API returns the full log, no streaming/websocket endpoint
- [x] Pipeline variable management — `bb variable list/set/delete` (repo pipeline variables)
- [x] Schedule management — `bb pipeline schedule list/create/view/delete` (create body is best-effort — verify against the live API)
- [x] Caches management — `bb pipeline cache list/delete` (artifacts download still pending)

### Medium Priority

#### Code Review Features
- [x] Inline comments support for PR reviews — `bb pr comment --path <file> --line <n>`
- [x] PR review threads navigation — `bb pr threads` (comments nested by reply, inline anchored)
- [x] Request changes with comment — `bb pr request-changes [--remove] [--comment]`
- [x] PR templates support — `bb pr create` defaults the description from `.bitbucket/`, root, `.github/`, or `docs/` `pull_request_template.md`

#### Repository Management
- [~] Repository settings management — read views added: `bb repo branching-model`, `bb repo permissions [--groups]`, `bb repo forks` (write/update still pending)
- [x] Access keys (SSH keys) management — `bb deploy-key list/add/view/delete`
- [x] Repository variables for pipelines — `bb variable list/set/delete`
- [~] Branch merge strategies configuration — not feasible: no merge-strategy/pullrequest-settings endpoint in the generated API spec

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
- [x] Pipeline analytics and statistics — `bb pipeline analytics` (success rate, counts by result)
- [x] Commit activity reports — `bb commit activity` (commits per author)
- [x] PR metrics — `bb pr metrics` (count, avg/median time to merge)

#### Bulk Operations
- [x] Bulk PR operations — `bb pr bulk approve/decline/merge <repo> <id...>`
- [x] Bulk issue updates — `bb issue bulk <repo> <id...> --state`
- [x] Export operations (PRs, issues to CSV/JSON) — global `--csv` (and existing `--json`) on every list command

#### Notifications
- [~] List notifications — not feasible: Bitbucket Cloud has no notifications REST API
- [~] Mark notifications as read — not feasible: no Bitbucket Cloud notifications API
- [~] Notification preferences — not feasible: no Bitbucket Cloud notifications API

## Technical Improvements

### Testing
- [~] Increase test coverage to 90%+ — ongoing: config/output/auth 90%+, cmd/bb ~80%; every command has httptest coverage
- [x] Add integration tests with mock API server — full cobra Execute path against httptest (integration_test.go), plus per-command httptest tests
- [~] End-to-end tests — cobra-Execute integration tests added (integration_test.go); live e2e vs the real API needs credentials, out of CI scope

### Documentation
- [x] Man pages generation — hidden `bb man <dir>` (cobra/doc)
- [x] Comprehensive README with examples — per-resource examples incl. all new commands
- [x] Contributing guide — CONTRIBUTING.md
- [x] API documentation — `bb man <dir> --format markdown` (cobra/doc) generates per-command reference

### Build and Distribution
- [x] GitHub Actions CI/CD pipeline (ci.yml + release.yml + .goreleaser.yaml)
- [x] Cross-platform binaries (Linux, macOS, Windows) — goreleaser builds amd64+arm64
- [x] Homebrew formula — goreleaser publishes to ThomasRedstone/homebrew-tap
- [x] APT/RPM packages — goreleaser nfpms (deb/rpm)
- [x] Scoop manifest for Windows — goreleaser scoops (needs a scoop-bucket repo + token, like the Homebrew tap)

### Performance
- [~] Response caching — not applicable: bb is a stateless single-shot CLI; superseded by on-demand pagination + --limit
- [x] Concurrent API requests where applicable — bulk PR/issue ops run with bounded concurrency (`runConcurrent`)
- [x] Lazy loading for large result sets — list commands paginate on demand and stop at `--limit`

### Code Quality
- [x] Golangci-lint integration — `.golangci.yml` (v2) + CI lint job
- [x] Pre-commit hooks — .pre-commit-config.yaml
- [x] Semantic versioning — goreleaser cuts releases from semver git tags (`-X main.version`)
- [x] Changelog automation — goreleaser changelog

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
