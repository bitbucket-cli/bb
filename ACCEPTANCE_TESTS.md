# Live acceptance evidence for a production release

Mock-server tests and generation establish local behavior. The checks below need
recorded results against real Bitbucket before their command families are called
production ready. Untested checks remain unverified. APIs preserved by the
compatibility patch particularly need live confirmation.

## Target and authorization

Record the disposable `workspace/repository`, CLI version/commit, OS, credential
kind/scopes and date. Never record credential values or private response bodies.
Use existing configured credentials; never put tokens in command-line arguments.

The maintainer identifies the target and authorizes mutation scope before live
execution. Start with reads. Use uniquely named test resources and record their
IDs. Clean up only resources created by the test. Deleting existing resources,
changing settings/permissions, merging and running pipelines need separate
explicit authorization. A failure is not permission to widen credential scopes.

## Acceptance matrix

| Area | Required evidence | Additional prerequisites |
| --- | --- | --- |
| Discovery/reads | Explicit `-R` and inferred repository agree; repo/branch/PR lists match the service; nested source paths work; empty JSON is `[]` | Read access; repo with a commit |
| PR lifecycle | Create a temporary branch/PR; read back title/target/draft fields; comment/reply/edit/delete; live reply survives deleted parent; decline/clean up | Authorized branch/PR/comment writes; avoid notifying other users |
| Bulk safety | Inspect `--dry-run`; invalid/duplicate IDs reject the entire batch; controlled success/partial failure produce ordered results and correct exit codes | Mutating cases authorized separately; dry-run makes no API calls |
| Merge | Preview strategy/source-branch options; merge only a test PR; verify final remote state, not just an accepted request | Explicit merge authorization; disposable branches |
| Pipelines | Read lists/details; trigger a controlled pipeline; verify completion, logs and cancellation | Explicit run authorization, known YAML and cost limits; no unreviewed pipeline code |
| Retained APIs | Verify advertised issue/workspace and compatibility-patched APIs; distinguish unavailable features from permission errors | Account with the relevant features; writes authorized separately |
| Permissions | Read-only identity gets a useful failure/nonzero exit on an attempted write, with no secret output | Separate scoped credential and authorized test target |
| Authentication | OAuth/API token; expired/revoked credentials; refresh; concurrent requests at expiry; logout; no leaked secrets | Dedicated test credentials; permission before replacing/deleting stored credentials |
| Native storage | Linux/macOS/Windows keyrings; locked/unavailable keyring; explicit file mode and native access controls | Native test accounts; avoid personal keyring entries |
| Installation/upgrade | Clean archive/Homebrew installation; version/completion; upgrade preserves config; checksums/notices; anonymous asset access | Published candidate; clean supported-platform environments |

Simulate rate limits, timeouts and interrupted requests locally first. Do not
exhaust a real account's quota. An ambiguous mutation may already have succeeded:
check remote state before retrying. Live tests must not disrupt shared access.

## Result record

For each check record pass/fail/unverified, environment, sanitized command,
expected result, observed exit/result, created resource IDs and cleanup outcome.
An accepted API request does not prove an asynchronous operation completed.
Keep outstanding cleanup visible. Stop mutations if target identity or resource
ownership is uncertain.

Promote verified command families individually. Claims about the whole CLI need
evidence across its advertised commands and supported platforms.
