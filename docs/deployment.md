# Deployment

Feedway is a stateless HTTP container with PostgreSQL as its only persistent
service. The repository provides a hardened Docker Compose example for a small
single-host installation.

## Quick start with Compose

You need Docker with Docker Compose, `curl`, and `openssl`. Create a deployment
directory and generate its secrets:

```bash
mkdir feedway && cd feedway
umask 077
export API_TOKEN="$(openssl rand -hex 32)"
export DB_PASSWORD="$(openssl rand -hex 32)"
printf 'API_TOKEN=%s\nDB_PASSWORD=%s\n' "$API_TOKEN" "$DB_PASSWORD" > .env
```

Download the example from `main` and start the services:

```bash
curl --fail --location \
  --output compose.yaml \
  https://raw.githubusercontent.com/zewelor/feedway/main/compose.example.yaml
docker compose up -d
docker compose ps
curl --fail http://localhost:8080/readyz
```

The example publishes the container's port 80 as host port 8080 and persists
PostgreSQL data in the `postgres-data` volume. Feedway creates its single table
automatically; there is no migration command to run.

The example uses `ghcr.io/zewelor/feedway:latest`. Every green push to `main`
also publishes an immutable full-commit-SHA image tag.

## Configuration

Only values that differ between deployment environments are configurable:

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `API_TOKEN` | yes | — | 64-character hexadecimal Bearer token |
| `DB_PASSWORD` | yes | — | PostgreSQL password |
| `DB_HOST` | yes | — | PostgreSQL host |
| `DB_PORT` | no | `5432` | PostgreSQL port |
| `DB_NAME` | yes | — | PostgreSQL database |
| `DB_USER` | yes | — | PostgreSQL user |
| `HTTP_PORT` | no | `80` | HTTP listen port inside the container |
| `RETENTION_DAYS` | no | `60` | Days to retain entries |

`API_TOKEN` must be exactly 64 hexadecimal characters. Generate one with:

```bash
openssl rand -hex 32
```

The feed size, request size, item count, timeouts, and cleanup interval are
application conventions, not configuration.

## External PostgreSQL

Feedway itself does not need a persistent disk. In Kubernetes or another
orchestrator, run only the stateless Feedway container and provide the `DB_*`
values for an existing PostgreSQL 18 service. The Compose PostgreSQL service is
a convenience for a small installation, not a requirement of the application
image.

Feedway does not include Kubernetes manifests. Connect it to the database using
the conventions of the platform you already operate.

Various hosted PostgreSQL services offer free or low-cost tiers that can be
useful for trying Feedway without operating PostgreSQL yourself. These tiers
and their limits change, so verify current pricing, quotas, backups, and network
access before relying on one for a deployment.
