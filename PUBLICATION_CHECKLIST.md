# Public preview publication checklist

The first public preview is `v0.3.0-rc.1`. This document records launch checks
and follow-up maintenance.

## Publication gate

- The maintainer authorized a public preview following history cleanup. The
  public history must omit the old bundled schema and copied descriptions.
  Generation downloads the schema temporarily; generated code and structural
  compatibility definitions remain distributed. See
  [generation provenance](SPECIFICATION_PROVENANCE.md).

## Launch sequence

1. Verify the cleaned public history and remove old private-era refs/artifacts
   from the public publication scope, retaining a private backup.
2. Verify hosted CI passes on the intended release commit.
3. Confirm legacy private-era releases remain drafts; retain their assets/tags.
4. Make `bitbucket-cli/bb` public before pushing the preview tag, so the release
   event permits the public Homebrew tap job.
5. Push `v0.3.0-rc.1`, verify the release workflow and all archive/package assets.
6. Verify `bitbucket-cli/homebrew-tap` points at the preview's public assets.
   If the tap token lacks access after the transfer, grant it access and rerun
   the failed tap job, or push the verified generated formula using maintainer
   access. Do not regenerate checksums from unrelated binaries.
7. Check anonymous asset access and an actual Homebrew installation, then remove
   the unpublished-preview wording from the installation guidance.

## Checks completed locally

Race tests, vet, lint, coverage ratchets, binary vulnerability scanning, generated
notices, client regeneration, six-target snapshot packaging and Homebrew formula
validation passed before publication preparation. Native hosted CI results must
be checked separately.

Gitleaks v8.30.1 scanned the cleaned history across all candidate refs (105
commits) with no findings. Old GitHub PR refs and private-era releases remain
in the separate private archive; the new public repository received only cleaned
`main` history. Automated secret scanning cannot guarantee absence of secrets.

## Follow-up maintenance

- Replace process-global Cobra flags, Viper and output state incrementally with
  command constructors and injected dependencies. Verify two executions in one
  process do not retain repository, format, flags or writers from each other.
- Extend isolated native tests to portable CLI/auth behavior on macOS/Windows;
  document manual keyring checks and Windows file-access expectations.
- Add an opt-in live Bitbucket smoke suite using a disposable repository, with
  explicit authorization for mutation tests.
  [ACCEPTANCE_TESTS.md](ACCEPTANCE_TESTS.md) defines the test matrix and evidence.
- Document and extend structured output beyond list commands, including mutation
  results and error output.
  Bulk PR/issue operations now have offline dry runs, full-batch ID validation
  before authentication, and ordered JSON/CSV success/failure results. Other
  mutation command families still need contract review.
- Keyring plus plaintext recovery mirroring is the default following the
  maintainer's decision, prominently disclosed in README. Native disappearance
  and session/locking behavior still need dedicated-platform evidence.
