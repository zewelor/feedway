# Feedway contributor instructions

## Source of truth

- `README.md` defines the current product promise and provides the documentation
  landing page. The relevant page under `docs/` is the source of truth for the
  detailed API, deployment, integration, or operations contract.
- `docs/future-ideas.md` is a non-prioritized parking lot, not a backlog. Moving
  an idea into the product requires an explicit user decision.
- The completed MVP has no active backlog. Start a new implementation package
  only after its scope and acceptance criteria are explicitly agreed.
- Keep changes inside the current agreed package. Do not implement adjacent
  ideas early. Update the relevant `docs/` page and `README.md` when the
  product contract changes.

### Documentation synchronization

- Keep the documentation structure in `docs/README.md` current when adding,
  removing, or renaming a documentation page.
- Markdown prose, headings, and code blocks may use lines up to 120 characters;
  Markdown tables are exempt from the line-length rule and should not be
  reshaped merely to satisfy it.
- Any change to an HTTP method, path, authentication requirement, request or
  response shape, status code, header, limit, or observable behavior must update
  `docs/api.md` in the same package. Check the route implementation and its
  examples against the documented contract.
- Any change to environment variables, Compose defaults, deployment behavior,
  health probes, logs, retention, or troubleshooting must update the matching
  page under `docs/` in the same package. Update README examples or links when
  the happy path changes.
- Do not add a second hand-maintained API contract. Add OpenAPI or generated
  reference tooling only as a separately accepted package with a concrete
  consumer and an explicit source-of-truth decision.

## Product design

Feedway follows KISS and convention over configuration, inspired by the Rails
and DHH style of providing one coherent, opinionated path:

- prefer a hardcoded convention or a strong default over a configuration knob;
- keep one obvious way to perform each operation;
- expose configuration only for secrets and values that genuinely differ
  between deployment environments;
- add an option, abstraction, interface, fallback, or extension point only when
  a concrete current requirement needs it;
- rely on standard-library behavior before adding custom fallback handlers;
  in particular, let `http.ServeMux` provide 404 and 405 responses unless the
  current product contract explicitly requires different behavior;
- delete speculative flexibility instead of preserving it for hypothetical
  future use;
- grow the product incrementally from observed needs, changing the contract in
  a reviewed implementation package.

Do not copy Rails architecture into Go. Apply the product-design principle:
small public surface, strong conventions, direct code, and few decisions for
the operator.

## Package workflow

For every package:

1. implement only that package;
2. present the diff for user review;
3. apply requested corrections and repeat review without running formatters,
   linters, or tests after every iteration;
4. wait for the user to explicitly approve the change set;
5. only after that approval, run the required formatters and linters, followed
   by `just test` in Docker Compose;
6. present any validation-induced changes or fixes for final review;
7. create exactly one local commit only after the final diff is accepted.

Do not run formatters, linters, or test suites during an active review loop
unless the user explicitly asks for them. Validation is intentionally deferred
until the proposed changes are approved so repeated review corrections do not
waste time, compute, or tokens.

Do not commit before explicit acceptance. Pushes, pull requests, tags, releases,
and publishing images require a separate user instruction.

The package diff must contain only the current package. `.agents/` must not be
modified by formatters or pre-commit tooling.

## Dependencies and versions

- Before adding or updating a Go module, tool, container image, or GitHub
  Action, verify the latest stable release in official documentation and the
  upstream project. Do not use prereleases without explicit approval.
- Stay within Go 1.26.x, PostgreSQL 18.x, pgx/v5, and Bluemonday.
- Pin external container images by digest. The deployment example intentionally
  uses the rolling `ghcr.io/zewelor/feedway:latest` image; CI also publishes an
  immutable full-commit-SHA tag. Reference GitHub Actions by their major tag,
  for example `actions/checkout@v7`.
- Record Go tools with `go get -tool`.
- Any additional runtime dependency requires an explicit justification in the
  review.

For library, framework, SDK, API, CLI, or cloud-service documentation, use the
Context7 CLI before answering or implementing:

```text
ctx7 library <official-name> "<full question>"
ctx7 docs <library-id> "<full question>"
```

Resolve the library first unless the user supplied a `/org/project` ID. Use no
more than three Context7 commands per question. If Context7 reports a quota
error, report it and suggest `ctx7 login` or `CONTEXT7_API_KEY`; do not silently
fall back to remembered API details.

## Repo-local Go skills

Install the exact skill set recorded in `skills-lock.json` directly from
`samber/cc-skills-golang`, using `npx skills add ... --copy` for all supported
agents. The CLI owns `.agents/skills` and `skills-lock.json`.

Verify installations with `npx skills list --json`. Update them only in a
separate reviewed package with:

```text
npx skills update -p -y
```

## Validation and Git output

- Install the repository-managed native Git hooks after cloning with
  `just hooks-install`. This sets the repository-local `core.hooksPath` to the
  tracked `.githooks/` directory without requiring a separate hook manager.
- The pre-push hook runs the complete `just ci` gate. Keep this expensive gate
  on push rather than commit so local commits stay fast during review.
- Git hooks are a local feedback layer, not a replacement for GitHub Actions.
  The remote CI result remains authoritative.
- `just test` is the acceptance gate for every implementation package.
- Use the narrower `just test-unit` and `just test-integration` only while
  iterating; use `just ci` for the complete local CI surface.
- Run Go tests and quality tools only inside the project Docker image. This
  includes `go test`, `go vet`, `golangci-lint`, `govulncheck`, `go mod verify`,
  and module-tidiness checks. The host may invoke `just`, Docker, and Docker
  Compose to orchestrate these checks; do not run equivalent Go commands
  directly on the host, even as an additional verification step.
- After changing the file layout, and especially after adding a file or
  directory at the repository root, run `just test_dockerignore`. Review the
  dry-run output and update `.dockerignore` when a file is not required in the
  Docker build context.
- For automated inspection, use non-paging Git commands such as
  `git --no-pager diff --staged`, `git --no-pager diff --stat`, and
  `git --no-pager show --stat`. Do not globally override `GIT_PAGER`.
