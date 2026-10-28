# bb - Bitbucket CLI

A powerful command-line interface for Bitbucket Cloud, enabling you to manage repositories, pull requests, pipelines, and more directly from your terminal.

> **Token storage:** By default, OAuth tokens are stored in the system keyring
> **and in a plaintext file at `~/.config/bb/tokens.json`** with owner-only Unix
> permissions. The file provides recovery when the keyring becomes unavailable
> or loses its entry; the root cause of the observed keyring loss is unresolved.
> The file is not encrypted. Select `bb config set token_store keyring` to disable
> file mirroring/fallback, accepting that a lost entry may require login again.
> See [token storage and diagnostics](#token-storage).

## Features

- **Authentication**: OAuth and API-token authentication; keyring storage with a plaintext recovery mirror by default (see [Token storage](#token-storage))
- **Repositories**: List, view, clone, create and update repositories; inspect forks
- **Pull Requests**: Create, view, merge, approve, comment, and diff pull requests
- **Branches**: Manage branches and branch restrictions
- **Pipelines**: Trigger, monitor, and view pipeline logs
- **Issues**: Full issue tracking with comments support
- **Tags**: Create and manage repository tags
- **Webhooks**: Configure repository webhooks
- **Deployments**: Manage deployment environments
- **Workspaces**: View workspace info, members, and projects
- **Interactive Mode**: Select resources interactively with `-i` flag
- **Multiple Output Formats**: Table and JSON output support
- **Shell Completions**: Bash, Zsh, Fish, and PowerShell

## Bulk operation safety

Bulk PR approval, decline and merge, and issue state updates support `--dry-run`:

```bash
bb pr bulk merge -R workspace/repo 12 15 --strategy squash --dry-run --json
bb issue bulk -R workspace/repo 12 15 --state resolved --dry-run --json
```

Dry runs resolve the target and validate IDs/options without authentication or
API requests. They cannot verify permissions, resource existence or whether a
remote operation will succeed. IDs must be positive decimal integers; duplicates
(including `12` and `012`) reject the batch before any API requests.

Dry runs and executions support table, JSON and CSV. JSON returns an array in
input order with `repository`, `resource`, `id` (a string), `action` and `status`
(`planned`, `success` or `failed`), plus relevant options and failure `error` text.
Merge options omitted from a plan inherit Bitbucket's defaults. `success` means
the API request succeeded, not that an asynchronous merge finished.

Execution can partially succeed: each attempted ID has a result and any failure
produces a nonzero exit. Successful operations are not rolled back. Check remote
state before retrying an ambiguous mutation. JSON/CSV results go to stdout;
overall CLI errors and usage messages remain on stderr.

## Installation

The public preview is **v0.3.0-rc.1**. See the
[preview release notes](RELEASE_NOTES.md) for changes and known limitations.

### Homebrew (macOS / Linux)

```bash
brew install bitbucket-cli/tap/bb
```

### go install

```bash
go install github.com/bitbucket-cli/bb/cmd/bb@v0.3.0-rc.1
```

### Linux packages (.deb / .rpm)

Download the `.deb` or `.rpm` for your architecture from the
[v0.3.0-rc.1 preview release](https://github.com/bitbucket-cli/bb/releases/tag/v0.3.0-rc.1), then:

```bash
sudo dpkg -i bb_*_linux_amd64.deb   # Debian/Ubuntu
sudo rpm -i  bb_*_linux_amd64.rpm   # Fedora/RHEL
```

Prebuilt binaries for Linux, macOS, and Windows (amd64 + arm64) are also
attached to each release.

### From Source

```bash
# Clone the repository
git clone https://github.com/bitbucket-cli/bb.git
cd bb

# Build
go build -o bb ./cmd/bb

# Install to PATH
sudo mv bb /usr/local/bin/
```

### Prerequisites

- Go 1.24 or later with automatic toolchain downloads enabled (only to build from source); builds use the patched toolchain specified in `go.mod`
- A Bitbucket Cloud account
- OAuth consumer credentials (see [Authentication](#authentication))

## Quick Start

```bash
# Login to Bitbucket
bb auth login

# List your repositories
bb repo list myworkspace

# View open pull requests
bb pr list myworkspace/myrepo

# Create a pull request
bb pr create myworkspace/myrepo -t "My feature" -s feature-branch

# Trigger a pipeline
bb pipeline trigger myworkspace/myrepo -b main
```

## Authentication

### Setting Up OAuth

1. Go to Bitbucket Settings > OAuth consumers
2. Create a new consumer with callback URL `http://localhost:34106/callback`.
3. Grant the permissions needed for your commands:
   - Account: Read
   - Repositories: Read, Write, Admin
   - Pull requests: Read, Write
   - Pipelines: Read, Write
4. Note your Client ID (Key) and Client Secret

### Login

```bash
# Interactive login (opens browser, prompts for credentials if not configured)
bb auth login

# Pass credentials directly (note: visible in shell history — prefer env vars)
bb auth login --client-id YOUR_CLIENT_ID --client-secret YOUR_CLIENT_SECRET

# Over SSH / no browser: print the URL, paste back the redirect
bb auth login --manual
# Or exchange a callback URL you already have (this standalone flow cannot
# validate which login attempt produced it; only use your own fresh callback):
bb auth login --callback-url "http://localhost:34106/callback?code=...&state=..."

# Using environment variables (recommended)
export BB_CLIENT_ID=your_client_id
export BB_CLIENT_SECRET=your_client_secret
bb auth login
```

### Check Authentication Status

```bash
bb auth status
```

### Logout

```bash
bb auth logout
```

## Commands

### Repositories

```bash
# List repositories in a workspace
bb repo list myworkspace

# View repository details
bb repo view myworkspace/myrepo

# Clone a repository
bb repo clone myworkspace/myrepo

# Create a new repository
bb repo create myworkspace/myrepo --description "My new repo" --private

# Update repository settings
bb repo update myworkspace/myrepo --description "Updated description"

# Fork a repository
bb repo fork myworkspace/myrepo --target-workspace myworkspace --name myrepo-fork

# Delete a repository
bb repo delete myworkspace/myrepo

# Manage default reviewers
bb repo reviewers list myworkspace/myrepo
bb repo reviewers add myworkspace/myrepo --user johndoe
bb repo reviewers remove myworkspace/myrepo --user johndoe
```

### Pull Requests

```bash
# List pull requests
bb pr list myworkspace/myrepo
bb pr list myworkspace/myrepo --state MERGED

# View PR details
bb pr view myworkspace/myrepo 123

# Interactive PR selection
bb pr view myworkspace/myrepo -i

# Create a pull request
bb pr create myworkspace/myrepo \
  --title "Add new feature" \
  --source feature-branch \
  --dest main \
  --body "Description of changes" \
  --reviewer johndoe

# Create a draft that deletes its source branch on merge
bb pr create --title "WIP" --draft --close-source-branch

# Retarget a PR, or take it out of draft
bb pr edit myworkspace/myrepo 123 --dest develop
bb pr edit myworkspace/myrepo 123 --ready

# Merge a pull request
bb pr merge myworkspace/myrepo 123

# Squash-merge with a custom message and delete the source branch
bb pr merge myworkspace/myrepo 123 --strategy squash -m "Add feature" --close-source-branch

# Approve a pull request (or withdraw your approval)
bb pr approve myworkspace/myrepo 123
bb pr approve myworkspace/myrepo 123 --remove

# Decline a pull request
bb pr decline myworkspace/myrepo 123

# View PR diff
bb pr diff myworkspace/myrepo 123

# List comments
bb pr comments myworkspace/myrepo 123

# Add a comment
bb pr comment myworkspace/myrepo 123 --body "Looks good!"
```

### Branches

```bash
# List branches
bb branch list myworkspace/myrepo

# View branch details
bb branch view myworkspace/myrepo main

# Create a branch
bb branch create myworkspace/myrepo feature-branch --target main

# Delete a branch
bb branch delete myworkspace/myrepo feature-branch

# Branch restrictions
bb branch restriction list myworkspace/myrepo
bb branch restriction create myworkspace/myrepo \
  --kind push \
  --pattern "main" \
  --value 2
bb branch restriction view myworkspace/myrepo 12345
bb branch restriction delete myworkspace/myrepo 12345
```

### Pipelines

```bash
# List pipelines
bb pipeline list myworkspace/myrepo

# View pipeline details
bb pipeline view myworkspace/myrepo 123

# Trigger a pipeline
bb pipeline trigger myworkspace/myrepo --branch main
bb pipeline trigger myworkspace/myrepo --branch main --pattern deploy -e ENVIRONMENT=staging

# Re-run a pull request's pipeline (clears a "No failed builds on last commit"
# merge check without pushing a commit, which can reset approvals)
bb pipeline trigger myworkspace/myrepo --pull-request 197

# Stop a running pipeline
bb pipeline stop myworkspace/myrepo 123

# View pipeline steps
bb pipeline steps myworkspace/myrepo 123

# View step logs
bb pipeline logs myworkspace/myrepo 123 --step step-uuid
```

### Issues

```bash
# List issues
bb issue list myworkspace/myrepo
bb issue list myworkspace/myrepo --state open

# View issue details
bb issue view myworkspace/myrepo 42

# Create an issue
bb issue create myworkspace/myrepo \
  --title "Bug report" \
  --content "Description of the bug" \
  --priority major \
  --kind bug

# Update an issue
bb issue update myworkspace/myrepo 42 --state resolved

# Delete an issue
bb issue delete myworkspace/myrepo 42

# Comments
bb issue comments myworkspace/myrepo 42
bb issue comment myworkspace/myrepo 42 --body "Working on this"
```

### Tags

```bash
# List tags
bb tag list myworkspace/myrepo

# View tag details
bb tag view myworkspace/myrepo v1.0.0

# Create a tag
bb tag create myworkspace/myrepo v1.0.0 --target main

# Delete a tag
bb tag delete myworkspace/myrepo v1.0.0
```

### Webhooks

```bash
# List webhooks
bb webhook list myworkspace/myrepo

# View webhook details
bb webhook view myworkspace/myrepo webhook-uuid

# Create a webhook
bb webhook create myworkspace/myrepo \
  --url https://example.com/webhook \
  --events repo:push,pullrequest:created \
  --description "CI webhook" \
  --secret mysecret

# Delete a webhook
bb webhook delete myworkspace/myrepo webhook-uuid
```

### Deployments

```bash
# List deployments
bb deploy list myworkspace/myrepo

# Manage environments
bb deploy env list myworkspace/myrepo
bb deploy env create myworkspace/myrepo \
  --name production \
  --type Production
bb deploy env delete myworkspace/myrepo env-uuid
```

### Workspaces

```bash
# List workspaces
bb workspace list

# View workspace details
bb workspace view myworkspace

# List workspace members
bb workspace members myworkspace

# List workspace projects
bb workspace projects myworkspace
```

### Configuration

```bash
# List all settings
bb config list

# Get a setting
bb config get workspace

# Set a setting
bb config set workspace myworkspace
bb config set output_format json

# Show config file path
bb config path
```

Available settings:
- `workspace` - Default workspace for commands
- `output_format` - Default output format (table, json, csv)
- `client_id` - OAuth client ID
- `client_secret` - OAuth client secret

### Environment Variables

| Variable | Description |
|----------|-------------|
| `BB_WORKSPACE` | Default workspace |
| `BB_CLIENT_ID` | OAuth client ID |
| `BB_CLIENT_SECRET` | OAuth client secret |
| `BITBUCKET_API_TOKEN` | API token to use instead of OAuth (see below) |
| `BITBUCKET_EMAIL` | Account email; pairs with `BITBUCKET_API_TOKEN` to use Basic auth |

Environment variables take precedence over config file values.

`BB_CLIENT_ID_COMMAND` and `BB_CLIENT_SECRET_COMMAND` (or the corresponding
`client_id_command` / `client_secret_command` settings) can retrieve credentials
from an external store. Commands run through `sh -c` on Linux/macOS and
`cmd.exe /D /S /C` on Windows, with a 30-second timeout. A command failure emits
a diagnostic on stderr and falls back to the next configured credential source.
Use command syntax appropriate to the operating system.

#### Token authentication

As an alternative to `bb auth login`, set `BITBUCKET_API_TOKEN`:

- **Atlassian API token** (the replacement for deprecated app passwords): also
  set `BITBUCKET_EMAIL`. The two are sent as HTTP Basic auth (`email:token`).
- **Repository/Workspace Access Token**: set only `BITBUCKET_API_TOKEN` (no
  email). It is sent as a `Bearer` token.

> Bitbucket app passwords are deprecated and being removed. Create an Atlassian
> API token with the scopes you need and set `BITBUCKET_EMAIL` alongside it.

## Output Formats

### Table Output (Default)

```bash
bb repo list myworkspace
```

### JSON Output

```bash
bb repo list myworkspace --json
```

## Interactive Mode

Use the `-i` flag to interactively select resources:

```bash
# Select a PR to view
bb pr view myworkspace/myrepo -i

# Select a PR to merge
bb pr merge myworkspace/myrepo -i

# Select a PR to approve
bb pr approve myworkspace/myrepo -i
```

## Shell Completions

### Bash

```bash
# Add to ~/.bashrc
source <(bb completion bash)

# Or install system-wide
bb completion bash > /etc/bash_completion.d/bb
```

### Zsh

```bash
# Add to ~/.zshrc
echo "autoload -U compinit; compinit" >> ~/.zshrc

# Generate completion
bb completion zsh > "${fpath[1]}/_bb"
```

### Fish

```bash
bb completion fish > ~/.config/fish/completions/bb.fish
```

### PowerShell

```powershell
bb completion powershell | Out-String | Invoke-Expression
```

## Configuration File

Configuration is stored in `~/.config/bb/config.yaml`:

```yaml
workspace: myworkspace
output_format: table
client_id: your_client_id
client_secret: your_client_secret
```

> **Security note:** `client_secret` is stored in plaintext in the config file. For better
> security, use `bb auth login` (which stores OAuth tokens in the system keyring) and supply
> credentials via `BB_CLIENT_ID` / `BB_CLIENT_SECRET` environment variables rather than
> writing them to the config file.

### Token storage

OAuth tokens are persisted according to `token_store` (or the `BB_TOKEN_STORE`
env var):

| Mode | Behaviour |
|------|-----------|
| `auto` *(default)* | System keyring **plus** a plaintext `0600` file mirror at `~/.config/bb/tokens.json`. Reads prefer the keyring, fall back to the file and re-seed the keyring. This is recovery behavior, not a fix for the cause of a disappearing entry. |
| `keyring` *(explicit opt-in)* | System keyring only; no plaintext token mirror or implicit file fallback. Writes are retried and read back to verify persistence. An unavailable keyring is reported as a storage problem. A genuinely missing token requires login or explicit file-storage selection. |
| `file` | `0600` file only (`~/.config/bb/tokens.json`). Reliable and portable (works headless); tokens sit at-rest on disk. |

```bash
bb config set token_store auto      # default; keyring plus plaintext recovery mirror
bb config set token_store keyring   # opt out of file mirroring/fallback
bb config set token_store file      # explicitly opt into file storage
BB_TOKEN_STORE=file bb auth status  # one-off override
```

Existing explicit storage choices are preserved. In `auto` mode an existing
token file can be used for recovery and to re-seed the keyring. In `keyring` mode
it is left untouched and is not imported; `bb auth status` explains the choices.
Selecting keyring-only storage does not erase an existing plaintext copy. `bb auth logout`
clears both backends and reports backend deletion failures.

For troubleshooting, set `BB_AUTH_DEBUG=1` when reproducing the problem. Auth
events are appended to `~/.config/bb/auth-debug.log` with owner-only Unix
permissions, timestamps and process IDs. They include retry attempts, successful
write/read-back verification, selected storage mode and missing/unavailable
backend classifications. Token values, backend error messages and OAuth error
response bodies are not written to this log. Logging is opt-in and cannot
reconstruct events from before it was enabled. OS keyring/session logs may still
be needed to establish why an entry disappeared. Native Windows access controls
should be checked separately; Unix permission bits are not a Windows ACL policy.

## Examples

### Daily Workflow

```bash
# Check your open PRs
bb pr list myworkspace/myrepo --state OPEN

# Review and approve a PR
bb pr view myworkspace/myrepo 123
bb pr diff myworkspace/myrepo 123
bb pr approve myworkspace/myrepo 123

# Merge after approval
bb pr merge myworkspace/myrepo 123
```

### CI/CD Integration

```bash
# Trigger pipeline on main branch
bb pipeline trigger myworkspace/myrepo --branch main

# Wait and check status
bb pipeline list myworkspace/myrepo | head -5

# View logs if failed
bb pipeline logs myworkspace/myrepo 123 --step build-step-uuid
```

### Create Release

```bash
# Create a tag for the release
bb tag create myworkspace/myrepo v1.0.0 --target main

# Run the deployment pipeline
bb pipeline trigger myworkspace/myrepo --branch main --pattern deploy
```

## Troubleshooting

### Authentication Issues

```bash
# Check current auth status
bb auth status

# Re-authenticate
bb auth logout
bb auth login
```

### API Errors

API failures are reported on stderr with a nonzero exit status. `--json` controls
successful command output; it does not turn errors into JSON.

## More Commands

### Commits

```bash
bb commit list myworkspace/myrepo --branch main --limit 20
bb commit view myworkspace/myrepo <hash>
bb commit activity myworkspace/myrepo        # commits per author
```

### Source browsing

```bash
bb source list myworkspace/myrepo            # repo root (default branch)
bb source list myworkspace/myrepo src --ref develop
bb source list --path src/nested             # auto-detect the repository
bb source view myworkspace/myrepo README.md
```

### Snippets

```bash
bb snippet list myworkspace --role owner
bb snippet create myworkspace main.go --title "helper" --private
cat notes.txt | bb snippet create myworkspace --filename notes.txt
bb snippet view myworkspace <id>
bb snippet update myworkspace <id> main.go
bb snippet delete myworkspace <id>
```

### Downloads

```bash
bb download list myworkspace/myrepo
bb download upload myworkspace/myrepo dist/app-linux dist/app-macos
bb download get myworkspace/myrepo app-linux -o ./app
bb download delete myworkspace/myrepo app-linux
```

### Deploy keys

```bash
bb deploy-key list myworkspace/myrepo
bb deploy-key add myworkspace/myrepo --key-file ~/.ssh/id_ed25519.pub --label ci
bb deploy-key delete myworkspace/myrepo <key-id>
```

### Pipeline variables, schedules, caches, analytics

```bash
bb variable list myworkspace/myrepo
bb variable set myworkspace/myrepo KEY value --secured
bb pipeline schedule list myworkspace/myrepo
bb pipeline schedule create myworkspace/myrepo --branch main --cron "0 0 6 * * ? *"
bb pipeline cache list myworkspace/myrepo
bb pipeline analytics myworkspace/myrepo     # success rate, counts by result
```

### Pull request extras

```bash
bb pr checkout 42                            # auto-detects repo from git remote
bb pr create myworkspace/myrepo -t "Title"   # source defaults to current branch
bb pr comment myworkspace/myrepo 42 -b "nit" --path main.go --line 10
bb pr request-changes myworkspace/myrepo 42 --comment "please fix the tests"
bb pr threads myworkspace/myrepo 42          # comments as nested threads, with IDs
bb pr activity myworkspace/myrepo 42 --csv   # approvals, change requests, head moves; STALE = approval predates head
bb pr comment myworkspace/myrepo 42 -b "done" --reply-to 1234
bb pr comment-edit myworkspace/myrepo 42 1234 -b "corrected text"
bb pr comment-delete myworkspace/myrepo 42 1234
bb pr metrics myworkspace/myrepo             # avg/median time to merge
bb pr bulk approve myworkspace/myrepo 1 2 3  # runs concurrently
```

### Repository settings & issue metadata

```bash
bb repo branching-model myworkspace/myrepo
bb repo permissions myworkspace/myrepo --groups
bb repo forks myworkspace/myrepo
bb issue milestones myworkspace/myrepo
bb issue bulk myworkspace/myrepo 12 15 19 --state resolved
```

### Output formats

List output supports `--json` and `--csv`. Empty lists produce `[]` in JSON and
column headers in CSV. Set a default with `bb config set output_format json`;
explicit output flags override that default:

```bash
bb pr list myworkspace/myrepo --csv > prs.csv
bb issue list myworkspace/myrepo --json | jq '.[].title'
```

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for development
checks and [ROADMAP.md](ROADMAP.md) for planned features. To report a vulnerability
privately, follow [SECURITY.md](SECURITY.md).

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Running Tests

```bash
go test ./... -cover
```

### Building

```bash
go build -o bb ./cmd/bb
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) for CLI framework
- Uses [Viper](https://github.com/spf13/viper) for configuration
- Interactive prompts powered by [promptui](https://github.com/manifoldco/promptui)
- API client generated from Bitbucket OpenAPI specification
