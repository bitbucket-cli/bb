# bb CLI — Code Review

This is a well-structured CLI with good test coverage of the API client layer, clean package separation, and sensible auth design. There are several issues to fix before open-source release, ranging from real bugs to polish items.

---

## Bugs

### 1. Double body injection in `repo.go` and `branch.go` (create/update operations)

`runRepoCreate`, `runRepoUpdate`, `runBranchCreate`, and `runBranchRestrictCreate` all pass the body via *both* the positional `io.Reader` argument *and* a `withBody` `RequestEditorFn`. The generated client uses the positional reader to build the request, then the editor overwrites `req.Body` with a second `bytes.NewReader`. The first reader has already been partially or fully consumed, and the content-length is set twice.

`repo.go:582-589`:
```go
resp, err := client.PostRepositoriesWorkspaceRepoSlugWithBody(
    context.Background(),
    repoCtx.Workspace,
    repoCtx.RepoSlug,
    "application/json",
    bytes.NewReader(body),   // sent here
    withBody,                // also set here → double write / partial read
)
```

**Fix:** Remove the `withBody` editor and rely solely on the positional body argument, which is the documented approach for `WithBody` variants.

---

### 2. Potential panic on short commit hash — `branch.go:187`

```go
commit = (*branch.Target.Hash)[:12]
```

If Bitbucket ever returns a hash shorter than 12 characters, this panics.

**Fix:** Use `min(12, len(*branch.Target.Hash))`.

---

### 3. `DeleteTokens` calls `deleteTokenFile()` twice — `keyring.go:76-83`

```go
func DeleteTokens() error {
    err := keyring.Delete(serviceName, "tokens")
    if err != nil {
        _ = deleteTokenFile()   // called if keyring fails
    }
    return deleteTokenFile()    // ALWAYS called, even on keyring success
}
```

If the keyring succeeds and no token file exists, this returns a "file not found" error, making `bb auth logout` appear to fail even when it succeeded.

**Fix:**
```go
func DeleteTokens() error {
    _ = keyring.Delete(serviceName, "tokens")
    _ = deleteTokenFile()
    return nil
}
```

---

### 4. Token refresh doesn't re-wind request body — `api/client.go:51-67`

On a 401 response, `authClient.Do` refreshes the token and retries the request. But the original `*http.Request.Body` has already been read and is exhausted. For POST/PUT requests that receive a 401, the retry sends an empty body.

**Fix:** Store the original body bytes before the first attempt and reset `req.Body` before the retry.

---

### 5. `pkg/pr/pr.go` — only fetches the first page of PRs

`pr.List` returns at most one page of results (Bitbucket default page size is 10). For repos with more than 10 open PRs, callers silently get a truncated list. This affects the `work` CLI which calls this library for multi-repo PR listing.

**Fix:** Follow `result.Next` pagination links until exhausted.

---

### 6. All list commands have the same pagination gap

`branch list`, `pipeline list`, and `pr list` all only return the first page. `repo list` correctly handles pagination behind `--all`. The other commands need the same treatment, or at minimum a "showing N of total" hint.

---

## Documentation/README Discrepancies

### 7. `bb auth login --client-id / --client-secret` flags don't exist

The README shows:
```bash
bb auth login --client-id YOUR_CLIENT_ID --client-secret YOUR_CLIENT_SECRET
```

But `runAuthLogin` has no such flags — it reads credentials interactively from stdin. Either add the flags (noting shell history exposure risk) or remove the example from the README.

### 8. `BB_DEBUG=1` is documented but not implemented

The README's troubleshooting section mentions `BB_DEBUG=1 bb repo list myworkspace` but there is no debug logging in the code.

---

## Architecture / Design Issues

### 9. Module path won't work for open-source release

`go.mod` declares `module github.com/tom/bb`. This needs to be the actual public GitHub path before release (e.g. `github.com/ThomasRedstone/bb`). All internal imports need updating to match.

### 10. Global mutable state in `output.CurrentFormat`

`output.CurrentFormat` is a package-level variable mutated by `SetFormat`, combined with the `jsonOutput` bool in `main.go`. Any parallel tests that exercise commands with different output formats could interfere.

**Fix:** Pass format as a parameter or via a scoped context, or ensure tests reset the global state between runs.

### 11. `output.PrintSingle` iterates a map in non-deterministic order

`format.go:89-97`:
```go
for key, value := range fields {
    fmt.Printf("%-*s  %s\n", maxKeyLen, key+":", value)
}
```

Map iteration in Go is randomised. Output fields will appear in a different order on each run.

**Fix:** Replace with an ordered `[]struct{key, value string}` slice.

### 12. `prInteractive` global is shared across PR subcommands

`prInteractive` is a package-level bool registered on multiple commands. If two `cobra.Command` instances are invoked in the same process (e.g. in tests), they share state.

**Fix:** Use `cmd.Flags().GetBool("interactive")` inside the `RunE` instead of a package-level variable.

### 13. `fetchPage` in `repo.go` accesses the underlying HTTP doer via `client.Client.Client.Do`

`repo.go:342-347`:
```go
return client.Client.Client.Do(req)
```

This triple-dereferences through the generated client's embedded doer. It works because the `authClient` is the doer injected at construction, but any refactor to the generated client or `api.Client` struct could silently break auth for paginated repo fetches.

**Fix:** Use a proper API call with params, or expose a helper on `api.Client` that makes authenticated raw requests.

### 14. Context is propagated inconsistently

`pr.go` correctly captures `cmd.Context()` and falls back to `context.Background()`. All other commands use `context.Background()` directly.

**Fix:** Standardise on `cmd.Context()` throughout so in-flight requests can be cancelled.

---

## Security Notes

### 15. `client_secret` stored in plaintext in the config file

`~/.config/bb/config.yaml` stores `client_secret` in plaintext. This is conventional for CLIs, but should be called out explicitly in the README security section with a recommendation to use `bb auth login` (keyring-backed) instead.

### 16. Sensitive fields shown in error messages

Several places stringify `errResp` maps directly into error messages. If the API returns sensitive content in error bodies, it will appear in stderr. Consider only showing raw error bodies in a debug mode.

---

## Minor Polish

### 17. Unicode symbols in `output.Success` / `output.Error` may not render on Windows

`✓` and `✗` may not render on Windows cmd.exe. Provide a fallback or check for terminal capability.

### 18. Magic 50-row threshold in `output.Print` is undocumented

```go
if len(rows) > 50 {
    return PrintSimple(headers, rows)
}
```

The silent switch to tab-separated output at 51 rows is invisible to the user. Either always use the table renderer (it handles large datasets fine) or expose a `--simple` / `--no-table` flag.

---

## Positive Observations

- OAuth CSRF state (16-byte random) and state-mismatch detection are correct.
- The 401 refresh-and-retry logic correctly uses a `sync.RWMutex` to protect concurrent token reads.
- File-based token fallback when keyring is unavailable is well-handled.
- The `pkg/pr` separation (used by the `work` CLI) is a clean, importable API boundary.
- Per-command `newAPIClient` mocking pattern makes commands testable without a live server.
- The test coverage of `internal/api` is solid, including retry and refresh scenarios.

---

## Remediation Plan

| Priority | Issue | File(s) | Effort |
|---|---|---|---|
| High | Module path for release (#9) | `go.mod`, all imports | Low |
| High | Double body injection (#1) | `repo.go`, `branch.go` | Low |
| High | `DeleteTokens` double-call (#3) | `keyring.go` | Low |
| High | Pagination in `pr`/`branch`/`pipeline list` (#5, #6) | `pr.go`, `branch.go`, `pipeline.go`, `pkg/pr/pr.go` | Medium |
| Medium | README flags that don't exist (#7, #8) | `README.md` | Low |
| Medium | Hash slice panic (#2) | `branch.go` | Low |
| Medium | Token refresh body reset (#4) | `api/client.go` | Medium |
| Medium | Non-deterministic map output (#11) | `output/format.go` | Low |
| Low | Context propagation consistency (#14) | all `cmd/bb/*.go` | Low |
| Low | `fetchPage` triple-dereference (#13) | `repo.go` | Medium |
| Low | Global output format state (#10) | `output/format.go`, tests | Low |
| Low | `prInteractive` global sharing (#12) | `pr.go` | Low |
| Low | Magic 50-row threshold (#18) | `output/format.go` | Low |
