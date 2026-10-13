# Public preview publication checklist

The intended preview is `v0.3.0-rc.1`. This checklist records work still needed;
it does not announce a published release.

## Publication gate

- Resolve publication scope and redistribution terms for the retained API
  structure and historical specification/descriptions. The current tree fetches
  the schema during explicit generation and removes prose from generated code,
  but the old material remains in Git history. See
  [research and unsent enquiry](SPECIFICATION_PROVENANCE.md). These changes alone
  do not clear the history question.

## Launch sequence

1. Obtain and record the permission or legal assessment resolving the gate.
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

Gitleaks v8.30.1 scanned all reachable Git refs (98 commits) and the working tree.
The reported basic-auth findings were upstream/documentation examples using
placeholder usernames, passwords or client credentials. No real credential was
identified. Automated secret scanning cannot guarantee absence of secrets.

## Follow-up maintenance

- Replace process-global Cobra flags, Viper and output state incrementally with
  command constructors and injected dependencies. Verify two executions in one
  process do not retain repository, format, flags or writers from each other.
- Extend isolated native tests to portable CLI/auth behavior on macOS/Windows;
  document manual keyring checks and Windows file-access expectations.
- Add an opt-in live Bitbucket smoke suite using a disposable repository, with
  explicit authorization for mutation tests.
- Document and extend structured output beyond list commands, including mutation
  results and error output.
- Revisit default plaintext token mirroring as a separate compatibility decision.
  This preview documents the default and the keyring-only setting.
