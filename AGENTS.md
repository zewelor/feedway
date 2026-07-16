# Feedway contributor instructions

## Source of truth

- `spec.md` defines the MVP contract.
- `docs/implementation-plan.md` is the only backlog. Do not create competing
  roadmaps, TODO lists, or speculative follow-up documents.
- `docs/future-ideas.md` is a non-prioritized parking lot, not a backlog. Moving
  an idea into the MVP requires an explicit user decision and updates to both
  the specification and implementation plan.
- Keep changes inside the current package from the implementation plan. Do not
  implement later packages early.

## Product design

Feedway follows KISS and convention over configuration, inspired by the Rails
and DHH style of providing one coherent, opinionated path:

- prefer a hardcoded convention or a strong default over a configuration knob;
- keep one obvious way to perform each operation;
- expose configuration only for secrets and values that genuinely differ
  between deployment environments;
- add an option, abstraction, interface, fallback, or extension point only when
  a concrete current requirement needs it;
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
2. run `just test` in Docker Compose;
3. present the diff for user review;
4. apply the requested corrections;
5. rerun `just test`;
6. wait for explicit user acceptance;
7. create exactly one local commit for the accepted package.

Do not commit before explicit acceptance. Pushes, pull requests, tags, releases,
and publishing images require a separate user instruction.

The package diff must contain only the current package. `.agents/` must not be
modified by formatters or pre-commit tooling.

## Dependencies and versions

- Before adding or updating a Go module, tool, container image, or GitHub
  Action, verify the latest stable release in official documentation and the
  upstream project. Do not use prereleases without explicit approval.
- Stay within Go 1.26.x, PostgreSQL 18.x, pgx/v5, Bluemonday, and Tern v2.
- Pin container images by digest and GitHub Actions by full commit SHA, with a
  version comment for Actions.
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

Install the exact skill set recorded in `docs/implementation-plan.md` directly
from `samber/cc-skills-golang`, using `npx skills add ... --copy` for all
supported agents. The CLI owns `.agents/skills` and `skills-lock.json`.

Verify installations with `npx skills list --json`. Update them only in a
separate reviewed package with:

```text
npx skills update -p -y
```

## Validation and Git output

- `just test` is the acceptance gate for every implementation package.
- Use the narrower `just test-unit` and `just test-integration` only while
  iterating; use `just ci` for the complete local CI surface.
- After changing the file layout, and especially after adding a file or
  directory at the repository root, run `just test_dockerignore`. Review the
  dry-run output and update `.dockerignore` when a file is not required in the
  Docker build context.
- For automated inspection, use non-paging Git commands such as
  `git --no-pager diff --staged`, `git --no-pager diff --stat`, and
  `git --no-pager show --stat`. Do not globally override `GIT_PAGER`.
