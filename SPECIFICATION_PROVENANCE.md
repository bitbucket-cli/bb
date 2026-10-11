# Bitbucket API specification provenance and redistribution status

Research date: 11 October 2026.

## Status

An explicit redistribution license for the bundled Bitbucket Cloud OpenAPI
specification and its copied descriptive text has not been established from the
official sources reviewed. This is an unresolved permission question, **not a
conclusion that this project or its distribution is illegal**.

The project's MIT license does not, by itself, establish rights to sublicense
Atlassian-origin material. Attribution records provenance but does not replace
an applicable license or permission. No Creative Commons or MIT license should
be assigned to that material without supporting evidence.

Public publication and release of the affected source and binaries should remain
on hold pending clarification or a legal assessment of the intended distribution.
The project owner can pursue either route. Nothing in this document implies that
Atlassian has been contacted or has approved this project.

## Local artifacts examined

SHA-256 hashes identify the exact local files examined, rather than asserting
that they are byte-identical to today's upstream JSON:

| Artifact | SHA-256 |
| --- | --- |
| `spec/openapi.yaml` | `194e35109d20987b9a78a33fe89ab72f31db8dc003b08fe18aedc2341c180880` |
| `generated/bitbucket/client.go` | `aedace461c2f2a484f7af7cd906d7ef0a05daf111fc19d80cdb303d6e0308289` |

The local specification identifies itself as Bitbucket API version 2.0, contains
Atlassian descriptions and examples, and has no `info.license`. Its
`info.termsOfService` points to
<https://www.atlassian.com/legal/customer-agreement>.

The Makefile regenerates Go types and client code with
`github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1` using this local
specification. The permission question covers the specification itself and the
generated file, including descriptions copied into generated comments. The
generator's own license is a separate question from rights in its input.

## Official sources reviewed

1. [Bitbucket Cloud REST API introduction](https://developer.atlassian.com/cloud/bitbucket/rest/intro/#schemas-and-serialization).
   Atlassian identifies the canonical schema and encourages client boilerplate
   generation. This supports the intended use of generation, but does not
   expressly identify a redistribution or sublicensing license.
2. [Published OpenAPI JSON](https://dac-static.atlassian.com/cloud/bitbucket/swagger.v3.json)
   and [canonical Swagger JSON](https://api.bitbucket.org/swagger.json).
   Both were inspected on the research date. Neither contained `info.license`;
   both referred to the Customer Agreement through `info.termsOfService`.
3. [Atlassian Developer Terms](https://developer.atlassian.com/platform/marketplace/atlassian-developer-terms/),
   effective 1 December 2025. Sections 3, 6, 13 and 23 address integration use,
   distribution restrictions, ownership and separately identified open-source
   code. No specific license for this specification was established there.
4. [Atlassian Customer Agreement](https://www.atlassian.com/legal/atlassian-customer-agreement),
   effective 1 October 2026; the specification's terms link redirects here.
   Its product-use provisions do not identify a specification redistribution
   license.
5. [Historical Confluence documentation contribution policy](https://confluence.atlassian.com/conf59/contributing-to-the-confluence-documentation-792499944.html).
   This policy describes CC BY 2.5 Australia licensing for that documentation.
   Its applicability to the current Bitbucket Cloud OpenAPI artifact was not
   established; it is not being treated as a grant for these files.

The current Bitbucket API introduction and pull-request reference HTML were also
checked for a Creative Commons notice; none was found. Searches of official
Atlassian sources and inspection of the public `atlassian/atlascode` and
`atlassian/swagger-request-validator` repository trees did not locate an
authoritatively licensed copy of this specification. These checks describe the
research performed, not proof that no applicable permission exists elsewhere.

## Unsent permission enquiry draft

**Status: draft only. Not sent.**

**Subject:** Redistribution permission for Bitbucket Cloud OpenAPI and generated Go client in an MIT CLI

Hello Atlassian team,

I maintain `bb`, an independent Bitbucket Cloud command-line client, and would
like to publish its source under MIT and distribute source archives and compiled
binaries, including through a Homebrew tap. It is not an official Atlassian
product.

The project includes a local YAML representation of the Bitbucket Cloud OpenAPI
specification and a Go models/client file generated with oapi-codegen v2.5.1.
The specification and generated comments include API descriptions and examples.
The exact files and SHA-256 hashes are listed above. The official JSON sources
are the two links recorded in this document.

Could you identify the applicable license or confirm permission for:

- Redistributing the raw specification, including its YAML representation and
  any project-maintained modifications, in a public source repository and source
  release archives.
- Redistributing and modifying generated Go models and client code, including
  copied API descriptions, examples and documentation comments.
- Distributing compiled CLI binaries incorporating that generated code and
  allowing downstream users to use, modify and redistribute the CLI.
- Applying MIT to the project's own code and generated output, or, if the
  specification/descriptive text must retain separate terms, identifying those
  terms and the required attribution, notices and license files.

If permission differs between the structural API definitions, generated code
and copied descriptive text, please specify the distinctions. Please also
confirm whether permission covers subsequent versions of the published schema
or only the versions identified here.

We will preserve required attribution and make the independent status of the
project clear. A link to an existing authoritative grant would be sufficient if
it covers these uses. Otherwise, written permission describing these uses and
any conditions would help us resolve this question before public release.

Thank you.
