# bb - Bitbucket CLI

A powerful command-line interface for Bitbucket Cloud, enabling you to manage repositories, pull requests, pipelines, and more directly from your terminal.

## Features

- **Authentication**: Secure OAuth authentication with token storage in system keyring
- **Repositories**: List, view, clone, create, update, fork, and delete repositories
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

## Installation

### Homebrew (macOS / Linux)

```bash
brew install ThomasRedstone/tap/bb
```

### go install

```bash
go install github.com/ThomasRedstone/bb/cmd/bb@latest
```

### Linux packages (.deb / .rpm)

Download the `.deb` or `.rpm` for your architecture from the
[latest release](https://github.com/ThomasRedstone/bb/releases/latest), then:

```bash
sudo dpkg -i bb_*_linux_amd64.deb   # Debian/Ubuntu
sudo rpm -i  bb_*_linux_amd64.rpm   # Fedora/RHEL
```

Prebuilt binaries for Linux, macOS, and Windows (amd64 + arm64) are also
attached to each release.

### From Source

```bash
# Clone the repository
git clone https://github.com/ThomasRedstone/bb.git
cd bb

# Build
go build -o bb ./cmd/bb

# Install to PATH
sudo mv bb /usr/local/bin/
```

### Prerequisites

- Go 1.24 or later (only to build from source)
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
2. Create a new consumer with these permissions:
   - Account: Read
   - Repositories: Read, Write, Admin
   - Pull requests: Read, Write
   - Pipelines: Read, Write
3. Note your Client ID and Client Secret

### Login

```bash
# Interactive login (opens browser, prompts for credentials if not configured)
bb auth login

# Pass credentials directly (note: visible in shell history — prefer env vars)
bb auth login --client-id YOUR_CLIENT_ID --client-secret YOUR_CLIENT_SECRET

# Over SSH / no browser: print the URL, paste back the redirect
bb auth login --manual
# Or exchange a callback URL you already have:
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

# Merge a pull request
bb pr merge myworkspace/myrepo 123

# Approve a pull request
bb pr approve myworkspace/myrepo 123

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
- `output_format` - Default output format (table, json)
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
| `auto` *(default)* | System keyring **plus** a `0600` file mirror at `~/.config/bb/tokens.json`. bb reads the keyring first and falls back to the file, re-seeding the keyring. This means a keyring that drops the entry (locking, session/collection eviction, headless boxes) can no longer log you out. |
| `keyring` | System keyring only. Most secure at-rest, but you'll be logged out whenever the keyring drops the entry. |
| `file` | `0600` file only (`~/.config/bb/tokens.json`). Reliable and portable (works headless); tokens sit at-rest on disk. |

```bash
bb config set token_store keyring   # opt into keyring-only
BB_TOKEN_STORE=file bb auth status  # one-off override
```

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

# Trigger deployment pipeline
bb pipeline trigger myworkspace/myrepo --tag v1.0.0
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

Use `--json` flag for detailed error responses:

```bash
bb pr create myworkspace/myrepo -t "Test" -s branch --json
```

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
bb pr threads myworkspace/myrepo 42          # comments as nested threads
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

Every list command supports `--json` and `--csv`:

```bash
bb pr list myworkspace/myrepo --csv > prs.csv
bb issue list myworkspace/myrepo --json | jq '.[].title'
```

## Contributing

Contributions are welcome! Please see [ROADMAP.md](ROADMAP.md) for planned features.

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
