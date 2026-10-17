# Updating the generated Bitbucket client

Run `make generate` from the repository root. It downloads the official OpenAPI
3.0 JSON URL pinned in `source.json`, checks the exact SHA-256, removes descriptive
text and examples, applies the structural compatibility patch, and runs the
pinned oapi-codegen version. Downloads and intermediate schemas live in a temporary
directory removed when the command exits. Only the generated Go client is saved.

Normal builds, tests and `go install` use that committed client. They do not run
generation or download a schema. The generator itself is obtained through Go's
module tooling, using the version in `source.json`.

For an already-downloaded input, including when the upstream server changes:

```bash
go run ./scripts/apigen -schema /path/to/swagger.v3.json
```

The same pinned checksum is required for local input. There is no bypass flag.
The command must run from the repository root and leaves the existing client
intact if downloading, verification, patching or generation fails.

## Compatibility and upstream updates

The first pin was fetched on 13 October 2026. The live upstream schema then had
171 paths; the original client was generated from a schema with 186 paths.
Upstream had removed or renamed APIs, including issue and workspace endpoints.
Regenerating directly from it would remove methods and models the CLI still uses.

`compatibility.json` is a JSON Patch containing add/remove/replace operations on
object keys (arrays are replaced whole). It preserves the established structural
contract: paths, operation IDs, models, parameters and validation constraints.
It contains no copied descriptive documentation or examples. It was derived by
comparing the prose-stripped current upstream schema with the prose-stripped
original private-era input. This is retained API structure, not a claim
that all upstream-derived material has been eliminated.

Initial regeneration was checked against the previous generated file using a Go
token comparison excluding comments: the code was identical. A fresh generation
also reproduced the new file byte for byte. Keeping old endpoints in the client
does not establish that Bitbucket still serves them; live API validation remains
separate from generation compatibility.

If Atlassian changes its download, the checksum fails rather than silently
changing the CLI. To intentionally adopt another schema:

1. Download it from the official source and review the structural API changes.
2. Update the URL/checksum in `source.json` and review/rebase the compatibility
   patch. Do not change only the checksum to accept an unreviewed download.
3. Run `make generate`, inspect the client diff and run the full tests/lint checks.
4. Run generation again and verify byte-for-byte reproducibility before committing
   the pin, patch and generated code together.

Descriptive keywords are stripped only where they are OpenAPI fields. Real API
properties named `description`, `summary` or `example`, and literal defaults/enums,
are preserved. Required names, types, tags, IDs and other structural information
remain available to the generator.

## Provenance

The upstream raw schema is no longer bundled in the current tree, and upstream
descriptions are no longer embedded in the generated Go comments. The public
history is cleaned of the old bundled specification and copied generated
descriptions; the original history is retained in a private backup.
The structural patch and generated code remain derived from the API definitions.
See `SPECIFICATION_PROVENANCE.md` at the repository root.
