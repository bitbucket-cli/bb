# API client generation and provenance

`make generate` downloads the official Bitbucket Cloud OpenAPI JSON into temporary
storage, verifies the SHA-256 pinned in `scripts/apigen/source.json`, removes
explanatory prose/examples and regenerates the committed Go client using the
pinned oapi-codegen version. Normal builds do not download the schema.

The official source is:
https://dac-static.atlassian.com/cloud/bitbucket/swagger.v3.json

Atlassian describes its schema and client generation at:
https://developer.atlassian.com/cloud/bitbucket/rest/intro/#open-api-specification

The input JSON is not vendored. **The generated Go code and structural
compatibility patch are distributed.** Automation changes how the input is
obtained; it does not mean that generated output contains no upstream-derived
API definitions. The compatibility patch retains paths, model definitions and
operation IDs needed by existing commands when they differ from the current
upstream schema. See [generation instructions](scripts/apigen/README.md).

The maintainer chose to publish this preview after removing the former bundled
schema and copied generated descriptions from the public Git history. A private
backup retains the development history. The project's MIT licence should not be
read as a separate licence grant over the downloaded Atlassian input schema.

The original input was added during private development in January 2026. Its
exact download/conversion commands were not recorded. Current generation has an
explicit official URL, checksum, generator version and structural compatibility
patch and has been verified reproducible. No claim is made that old private-era
input bytes equal the current upstream download.
