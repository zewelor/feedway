set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

# Run the complete suite against a fresh PostgreSQL instance on loopback.
test:
    #!/usr/bin/env bash
    set -euo pipefail
    # Hold the loopback port through teardown, including across checkouts.
    exec 9>>"/tmp/feedway-test-${UID}.lock"
    flock 9
    compose=(docker compose -f compose.test.yaml -p "feedway-test-$$")
    trap '"${compose[@]}" down --volumes --remove-orphans' EXIT
    "${compose[@]}" up --wait --wait-timeout 60 postgres
    go test -count=1 -race -tags=integration ./...

# Run tests that do not require PostgreSQL.
test-unit:
    go test -race ./...

alias test-integration := test

# Check Markdown formatting without modifying files.
check-markdown:
    mise exec -- dprint check

# Format repository Markdown.
format-markdown:
    mise exec -- dprint fmt

# Enable the repository-managed native Git hooks.
hooks-install:
    git config --local core.hooksPath .githooks

# Run tests and static checks without building the production image.
ci-checks: test
    go mod verify
    go mod tidy -diff
    (cd tools && go mod verify && go mod tidy -diff)
    unformatted="$(find cmd internal -type f -name '*.go' -exec gofmt -l {} +)"; test -z "$unformatted" || { echo "$unformatted"; exit 1; }
    go vet -tags=integration ./...
    go tool -modfile=tools/go.mod golangci-lint run --build-tags=integration
    go tool -modfile=tools/go.mod govulncheck ./...

# Run the complete local CI quality gate.
ci: check-markdown ci-checks
    docker build .

# Preview the files included in the Docker build context.
show_dockerignore:
    #!/bin/sh
    set -eu
    LC_ALL=C
    export LC_ALL

    test_dir="$(mktemp -d)"
    trap 'rm -rf "$test_dir"' 0 1 2 3 15
    mkdir -p "$test_dir/context"

    docker build --file - --progress=quiet --output "type=local,dest=$test_dir/context" . >/dev/null <<-'EOF'
    # syntax=docker/dockerfile:1
    #check=skip=CopyIgnoredFile
    FROM scratch
    COPY . /context
    EOF

    find "$test_dir/context/context" \( -type f -o -type l \) -print |
        sed "s#^$test_dir/context/context/##" |
        sort > "$test_dir/included"

    cat "$test_dir/included"
    printf '\n%s\n' '---'
    printf 'Total files:\t%s\n' "$(wc -l < "$test_dir/included" | awk '{print $1}')"
    printf 'Total size:\t%s\n' "$(du -sh "$test_dir/context/context" | awk '{print $1}')"
