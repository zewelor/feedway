# Feedway

## Product contract

- `README.md` states the product promise and links to the handbook. The relevant page under `docs/` owns each detailed
  API, deployment, integration, or operations contract. Keep both layers in sync when behavior changes.
- Update `docs/README.md` when adding, removing, or renaming a documentation page.
- Update `docs/api.md` with every change to the HTTP contract or observable behavior. Check the implementation and
  examples against it.
- Update the relevant handbook page for changes to environment variables, Compose, deployment, probes, logs, retention,
  or troubleshooting. Update README examples when the happy path changes.
- Keep one hand-maintained API contract. OpenAPI or generated reference documentation requires an explicitly accepted
  package, consumer, and source-of-truth decision.
- `docs/future-ideas.md` is a parking lot, not a backlog. The completed MVP has no active backlog.
- Work only within explicitly agreed scope and acceptance criteria. Do not implement adjacent ideas early.

## Engineering principles

Apply Jane Street-style correctness, KISS, and convention over configuration: keep one coherent, opinionated path, a
small public surface, and few operator decisions.

- Make invalid states hard to represent. Validate untrusted input at the boundary; use domain types when they enforce a
  real invariant.
- Keep the functional core separate from effects. Data transformation should not perform database, network, clock, or
  logging work. Keep those dependencies explicit at the edges.
- Prefer immutable values and visible data flow. Give mutable state one clear owner.
- Treat readability as part of correctness. Prefer direct code, precise names, exhaustive decisions, and small
  reviewable changes.
- Prefer conventions and strong defaults to configuration. Expose configuration only for secrets and values that
  genuinely differ between deployment environments.
- Add an abstraction, interface, fallback, or extension point only for a current requirement. Define interfaces at the
  consumer boundary; delete speculative flexibility.
- Prefer the standard library. Let `http.ServeMux` provide 404 and 405 responses unless the product contract requires
  otherwise.
- Measure before optimizing. Optimize the observed bottleneck and keep performance-specific complexity local.
- Design effects so behavior remains deterministic and independently checkable.

## Dependencies

- Stay within Go 1.26.x, PostgreSQL 18.x, pgx/v5, and Bluemonday.
- Before changing a module, tool, image, or GitHub Action, verify the latest stable upstream release. Do not use
  prereleases without explicit approval.
- Use Context7 for current upstream documentation before dependency or external-API work.
- Pin external container images by digest. The deployment example intentionally uses
  `ghcr.io/zewelor/feedway:latest`; CI also publishes a full-commit-SHA tag.
- Reference GitHub Actions by major tag, for example `actions/checkout@v7`.
- Record Go tools with `go get -tool`. Justify every additional runtime dependency in review.

## Change discipline

- Keep each change set focused on one accepted package and present its diff for review.
- During review, revise the diff without running validation unless the user asks. Validate only after approval, then
  present any resulting fixes.
- After final acceptance, create exactly one local commit. Pushes, pull requests, tags, releases, and image publication
  require separate instructions.

## Repo-local skills

- Treat `.agents/skills` and `skills-lock.json` as CLI-owned. Update them only as a separate reviewed package with
  `npx skills update -p -y`.

## Validation

- After approval, run the required formatters and linters, then `just test`. Use `just ci` for the complete local gate;
  GitHub Actions remains authoritative after a push.
- Run Go tooling through the project Docker image, orchestrated by `just` or Docker Compose, never directly on the host.
- After changing the root file layout, run `just test_dockerignore` and exclude files the image build does not need.

## Git output

- For automated inspection, use `git --no-pager diff --staged`, `git --no-pager diff --stat`, and
  `git --no-pager show --stat`.
- Do not override `GIT_PAGER` globally.
