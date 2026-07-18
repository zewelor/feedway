# Feedway documentation

This handbook is the detailed, versioned documentation for Feedway. The root
[README](../README.md) is the product landing page and the shortest path to a
working deployment.

## Use Feedway

- [HTTP API](api.md) — publish entries, consume the JSON Feed, and inspect
  health and readiness.
- [Deployment](deployment.md) — prepare secrets, start Compose, configure the
  application, and connect an external PostgreSQL service.
- [Integrations](integrations.md) — publish from n8n, subscribe from Miniflux,
  and verify the feed with `curl`.
- [Operations](operations.md) — probes, logs, retention, and troubleshooting.

## Project notes

- [Development](../README.md#-development) — repository checks and the local
  contributor workflow.
- [Future ideas](future-ideas.md) — deliberately deferred ideas, not an active
  roadmap.

The pages above document the current MVP. Feedway has one hardcoded feed and a
small HTTP surface; there is no dashboard, feed-management API, or user system.
