set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

compose := "docker compose -f compose.test.yaml -p feedway-test"

# Run the complete test suite against ephemeral PostgreSQL.
test:
    #!/usr/bin/env bash
    set -euo pipefail
    trap '{{compose}} down --volumes --remove-orphans' EXIT
    {{compose}} run --build --rm test

# Run tests that do not require PostgreSQL.
test-unit:
    #!/usr/bin/env bash
    set -euo pipefail
    trap '{{compose}} down --volumes --remove-orphans' EXIT
    {{compose}} run --build --rm --no-deps test go test -race ./...

# Run all tests, including integration tests, against ephemeral PostgreSQL.
test-integration:
    #!/usr/bin/env bash
    set -euo pipefail
    trap '{{compose}} down --volumes --remove-orphans' EXIT
    {{compose}} run --build --rm test

# Check repository Markdown without modifying files.
lint-markdown:
    {{compose}} run --rm --no-deps markdownlint "**/*.md" "#.agents/**"

# Apply safe automatic fixes to repository Markdown.
format-markdown:
    {{compose}} run --rm --no-deps --user "$(id -u):$(id -g)" markdownlint --fix "**/*.md" "#.agents/**"

# Run tests and static checks without building the production image.
ci-checks build="yes": lint-markdown
    #!/usr/bin/env bash
    set -euo pipefail
    trap '{{compose}} down --volumes --remove-orphans' EXIT
    build_flag=""
    if [[ "{{build}}" == "yes" ]]; then build_flag="--build"; fi
    {{compose}} run $build_flag --rm test sh -ec '
        go test -race -tags=integration ./...
        unformatted="$(find cmd internal -type f -name "*.go" -exec gofmt -l {} +)"
        test -z "$unformatted" || { echo "$unformatted"; exit 1; }
        go vet -tags=integration ./...
        golangci-lint run --build-tags=integration
        govulncheck ./...
    '

# Run the complete local CI quality gate.
ci: ci-checks
    docker build .

# Preview the files included in the Docker build context.
test_dockerignore:
    rsync -avn . /dev/shm --exclude-from .dockerignore
