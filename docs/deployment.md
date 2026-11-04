# Deployment

Feedway is a stateless HTTP container with PostgreSQL as its only persistent
service. The repository provides a hardened Docker Compose example for a small
single-host installation.

The production image uses `scratch` and contains the statically linked Feedway
binary, its license, and CA certificates copied from the pinned Go build image.
It runs as UID/GID `65532:65532` with a read-only root filesystem in Compose.
The image has no shell or package manager; run diagnostics from the host or a
separate container. Rebuild the image to refresh its binary and CA certificates.

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
PostgreSQL data in the `postgres-data` volume. Feedway creates and upgrades the
`entries` table automatically at startup; there is no migration command to run.

The example uses `ghcr.io/zewelor/feedway:latest`. Every green push to `main`
also publishes an immutable full-commit-SHA image tag.

## Configuration

### Upgrading an existing database

Feedway prepares its schema at application startup. The upgrade ensures the
nullable `data_json` and `feed` columns are present and adds a `content_hash`
column to the existing `entries` table. Existing IDs, content, timestamps, and JSON
attachments are preserved. Existing entries remain unassigned (`feed` is
`NULL`) and therefore appear in the aggregate feed only. Their existing
`sha256-v1` IDs are retained and copied into `content_hash` so retries continue
to deduplicate. New entries receive PostgreSQL 18 UUIDv7 IDs; the content hash
is stored separately and deduplicated within each feed by a unique
`(feed, content_hash)` constraint. Feedway uses no feed registry table or
migration framework.

The upgrade runs atomically and is safe to repeat during startup. The backfill
and ID constraint replacement run once; subsequent startups retain the existing
constraint. The database user must own the table or otherwise have permission
to alter it. Take a database backup before upgrading. Schema preparation uses
the existing five-second timeout; lock contention or insufficient privileges fails startup
visibly. Resolve the cause before restarting the application.

Replace the old application when upgrading. Earlier binaries do not supply the
required `content_hash` and cannot publish against the upgraded schema. The
upgrade preserves existing HTTP clients, but does not support running old and
new application versions together or rolling back only the binary.

The `data_json` column uses PostgreSQL `json` to retain numeric tokens and JSON
escapes without imposing `jsonb` numeric or Unicode storage restrictions.
Feedway does not query or index fields inside these documents.

The application needs no persistent disk, upload directory, or extra service
for attachments. JSON stays in PostgreSQL across application restarts.

### Environment variables

Only values that differ between deployment environments are configurable:

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `API_TOKEN` | yes | — | 64-character hexadecimal Bearer token |
| `BASE_URL` | no | — | Public origin for absolute permalinks and feed URL metadata |
| `DB_PASSWORD` | yes | — | PostgreSQL password |
| `DB_HOST` | yes | — | PostgreSQL host |
| `DB_PORT` | no | `5432` | PostgreSQL port |
| `DB_SSLMODE` | no | `disable` | PostgreSQL TLS mode: `disable`, `require`, `verify-ca`, or `verify-full` |
| `DB_NAME` | yes | — | PostgreSQL database |
| `DB_USER` | yes | — | PostgreSQL user |
| `HTTP_PORT` | no | `80` | HTTP listen port inside the container |
| `RETENTION_DAYS` | no | `30` | Days to retain entries, from 1 to 36,500 |
| `FEED_RESPONSE_MAX_ENTRIES` | no | `100` | Maximum feed entries per response, from 1 to 1,000 |

`API_TOKEN` must be exactly 64 hexadecimal characters. Generate one with:

```bash
openssl rand -hex 32
```

Set `BASE_URL` to Feedway's public origin, for example
`https://feed.example.com`. A trailing slash is ignored. With it configured,
feeds advertise their own absolute `feed_url` and a shared `home_page_url`
pointing to `/`. When it is unset, these two metadata fields are omitted and
entry and attachment permalinks are relative to the origin root, such as
`/entries/{id}`, including when a reader requests a feed under `/feeds/{feed}/`.

Feed identifiers are chosen in publish requests and need no deployment-time
configuration or setup. See the [HTTP API reference](api.md#feed-identifiers)
for the allowed format and filtering behavior.

Set `FEED_RESPONSE_MAX_ENTRIES` when the default feed count does not cover the intended
reading workflow, for example:

```text
FEED_RESPONSE_MAX_ENTRIES=500
```

The Compose example passes this value from the shell or `.env`; an empty or
unset value uses the application's default. After changing it, run
`docker compose up -d feedway` to recreate the application container
with the new environment. A plain container restart does not reload environment
variables. Values outside 1–1,000 or non-integer values fail startup. The upper
bound is an operator guardrail against accidentally requesting an excessive
number of rows, not a PostgreSQL limit.

The 4 MiB response limit still applies. The database query uses the same budget
for raw HTML and keeps one extra entry to detect truncation. Each feed read
therefore transfers at most 4 MiB + 256 KiB of HTML to the application. This does
not cap total process memory: metadata, JSON encoding, and concurrent requests
also use memory. Raising the count can increase database work and memory use
and does not guarantee a complete feed; check
`_feedway.truncated` in the response. Retention remains independent of this
setting.

The feed byte size, request size, timeouts, and cleanup interval are
application conventions, not configuration. Feedway configures an 8 KiB HTTP
header limit, 5-second header-read timeout, 15-second read and write timeouts,
60-second idle timeout, and 15-second graceful-shutdown timeout.

## External PostgreSQL

Feedway itself does not need a persistent disk. In Kubernetes or another
orchestrator, run only the stateless Feedway container and provide the `DB_*`
values for an existing PostgreSQL 18 service. The Compose PostgreSQL service is
a convenience for a small installation, not a requirement of the application
image.

The Compose example uses `DB_SSLMODE=disable` on its private container network.
For an external PostgreSQL service, set `DB_SSLMODE` to the mode required by the
provider; prefer `verify-full` when its certificate chain and hostname support
verification.

Feedway does not include Kubernetes manifests. Connect it to the database using
the conventions of the platform you already operate.

Various hosted PostgreSQL services offer free or low-cost tiers that can be
useful for trying Feedway without operating PostgreSQL yourself. These tiers
and their limits change, so verify current pricing, quotas, backups, and network
access before relying on one for a deployment.
