# Operations

## Probes

Use the probes from the deployment environment:

```text
GET /healthz  process is alive; does not query PostgreSQL
GET /readyz   startup finished, PostgreSQL responds, shutdown has not started
```

`/healthz` is suitable for a liveness check. `/readyz` is suitable for traffic
routing and reports `503` until the database is ready or after shutdown begins.
The complete HTTP contract is in the [API reference](api.md#health-and-readiness).

## Logs

Successful health and readiness probes are not logged. Other requests use
structured JSON logs containing the method, matched route, status, and duration.

With Docker Compose, follow the Feedway logs with:

```bash
docker compose logs -f feedway
```

Inspect PostgreSQL separately when readiness fails:

```bash
docker compose ps
docker compose logs postgres
```

## Retention

Retention runs once at startup and then every 24 hours. It keeps entries for 60
days by default. Override the default only when the deployment has a concrete
reason:

```text
RETENTION_DAYS=90
```

The setting must be a positive integer.

## Troubleshooting

- **Compose requires `API_TOKEN` or `DB_PASSWORD`:** Create `.env` as shown in
  the [deployment guide](deployment.md#quick-start-with-compose).
- **Feedway rejects `API_TOKEN`:** Generate a 64-character hexadecimal token
  with `openssl rand -hex 32`.
- **`/readyz` returns `503`:** Check `docker compose ps` and the PostgreSQL logs.
- **Publishing returns `401`:** Check the `Authorization: Bearer ...` header and
  the credential value.
- **Publishing returns `422`:** Check that HTML remains after sanitization and
  stays within the documented limits.
- **Port 8080 is already allocated:** Change the host port mapping in Compose;
  the container listens on port 80 by default.
