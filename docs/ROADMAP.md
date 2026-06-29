# bb CLI — Roadmap to Full Bitbucket Cloud API Coverage

Status snapshot: 2026-06-29. Baseline is ~50 leaf commands across 10 resource
areas (PRs, repos, branches, issues, webhooks, tags, pipelines, workspaces,
deployments, auth). This roadmap closes the gaps to full coverage, ordered by
value rather than by API completeness for its own sake.

## Principles

- **Daily workflow first.** Prioritise what a developer touches every day
  (commits, PR review depth, file/source read) over admin surface (snippets,
  keys).
- **Coverage breadth ≠ command depth.** Each new area ships with pagination,
  `--output json`, and filter flags from day one, and existing commands get
  back-filled to the same bar.
- **Ergonomics are a feature.** Auto-detecting `workspace/repo` from the git
  remote affects every command and is worth more than any single new verb.

---

## Phase 0 — Keep the lights on (blocking, not coverage)

These don't add coverage but gate everything else.

- **0.1 — Modernise token auth. ✅ Done.** `BITBUCKET_API_TOKEN` + `BITBUCKET_EMAIL`
  now authenticate via HTTP Basic (Atlassian scoped API tokens); token-only stays
  Bearer (Access Tokens / OAuth). See `internal/auth/token.go` (`Credential`,
  `GetCredential`) and `internal/api/client.go`.

  Original context: Bitbucket app passwords are deprecated
  (no new ones since 2025-09-09; existing stopped working 2026-06-09; full
  removal 2026-07-28). The current `BITBUCKET_API_TOKEN` path attaches as
  `Authorization: Bearer`, but the replacement Atlassian **scoped API tokens**
  authenticate via **Basic auth** (`email:token`). Rework that path (and add
  `BITBUCKET_EMAIL` or equivalent) or it is dead on arrival.
  Files: `internal/api/client.go`, `internal/auth/`.
- **0.2 — OAuth secret sourcing.** Optional: support fetching the OAuth client
  secret from an external provider (e.g. AWS Secrets Manager) at runtime
  instead of plaintext config, storing in the system keyring rather than
  `config.yaml`. (PKCE is *not* an option — Bitbucket Cloud doesn't support it.)

## Cross-cutting workstream (runs alongside all phases)

- **X.1 — Git remote auto-detection.** Infer `workspace/repo` from the origin
  remote so `<workspace/repo>` becomes optional on every command (à la `gh`).
  Highest-leverage UX change in the whole roadmap. Today every command requires
  the positional (e.g. `cmd/bb/pr.go:40`).
- **X.2 — Pagination + `--limit` consistency** across all `list` commands.
- **X.3 — `--output json|table|yaml` everywhere**, jq-friendly.
- **X.4 — Test coverage parity** for each new command (mirror existing
  `*_test.go` mock pattern).

---

## Phase 1 — Close the daily-workflow holes (high value)

- **1.1 — Commits.** Biggest functional gap.
  - `bb commit list` → `GET /2.0/repositories/{ws}/{repo}/commits` (by branch/ref)
  - `bb commit view <sha>` → `GET .../commit/{sha}`
  - `bb commit compare <a> <b>` → `GET .../diff/{spec}` / `commits/{spec}`
- **1.2 — Build/commit statuses (code that gates merges).**
  - `bb commit status <sha>` → `GET .../commit/{sha}/statuses`
  - `bb pr checks <id>` → surface statuses on a PR's head commit
- **1.3 — Source / file read.**
  - `bb repo files <ws/repo> [path] [--ref]` → `GET .../src/{ref}/{path}` (list dir)
  - `bb repo cat <ws/repo> <path> [--ref]` → raw file contents at a ref
- **1.4 — PR review depth.**
  - Inline/line comments → `POST .../pullrequests/{id}/comments` with `inline`
  - `bb pr reviewers` / participants → from PR object
  - PR `request-changes`, PR tasks → `.../pullrequests/{id}/tasks`
- **1.5 — Existing-command depth.** Issue filters (assignee, reporter, labels,
  state, kind); PR list filters (author, reviewer, source/dest branch).

## Phase 2 — Admin & completeness (medium value)

- **2.1 — Projects CRUD.** `bb project list|view|create|update|delete`
  → `/2.0/workspaces/{ws}/projects/{key}`
- **2.2 — Keys.**
  - Deploy keys → `bb repo deploy-key list|add|delete`
    (`.../deploy-keys`)
  - SSH keys (account) → `bb ssh-key list|add|delete`
    (`/2.0/users/{uuid}/ssh-keys`)
- **2.3 — Users / account.**
  - `bb user view [name]` → `/2.0/users/{selected_user}`
  - `bb auth whoami` / `bb user current` → `/2.0/user`
- **2.4 — Deployments / pipelines depth.**
  - Environment variables → `.../deployments_config/environments/{uuid}/variables`
  - Pipeline variables, caches, known-hosts, SSH config
  - Pipeline schedules → `.../pipelines_config/schedules`
- **2.5 — Branch restriction `update`** (PUT) to complete that CRUD set.
- **2.6 — Repo lifecycle completeness.** `bb repo delete`, `fork`, and
  permission/group/user access management.

## Phase 3 — Long tail (low value, finish the surface)

- **3.1 — Snippets.** `bb snippet list|view|create|delete`
  → `/2.0/snippets/{ws}`
- **3.2 — Downloads.** `bb repo download list|upload|delete`
  → `.../downloads`
- **3.3 — Code Insights (reports/annotations).**
  `.../commit/{sha}/reports` — CI/quality tooling integration.
- **3.4 — Webhook depth.** Update (PUT), event-type validation.
- **3.5 — Misc refs.** Anything left under `refs/` beyond branches/tags.

---

## Suggested sequencing

1. **Phase 0.1** (auth breakage) — do first; it's a live regression.
2. **X.1** (remote auto-detect) — ship early; every later command benefits.
3. **Phase 1** in order (commits → statuses → source → PR review → filters).
4. **Phase 2** as internal demand dictates (keys/projects/users).
5. **Phase 3** opportunistically.

Coverage is "complete" when Phases 1–3 land with the cross-cutting bar
(pagination, JSON, tests) applied uniformly.
