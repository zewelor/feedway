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

# Run the complete local CI surface available in the current package.
ci: test

# Preview the files included in the Docker build context.
test_dockerignore:
    rsync -avn . /dev/shm --exclude-from .dockerignore
