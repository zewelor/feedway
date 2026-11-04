# Feedway — future ideas

This document is a non-prioritized parking lot for ideas deliberately excluded
from the MVP. It is neither a backlog nor a promise to implement them.

An idea enters the product only after a concrete need appears, the contract
change is explicitly accepted, and a minimal acceptance criterion is agreed.

## More feeds

- a `feeds` registry table for feed metadata;
- creating, listing, updating, and deleting registered feeds;
- configurable titles, descriptions, and public URLs;
- private feeds and per-feed tokens.

## Entry management

- listing entries through the API;
- deleting individual entries;
- cursor pagination;
- updating entries;
- revision history or soft deletion.

## Alternative content and client identity

- client-provided identifiers and updates to existing entries;
- separate `content_text`;
- client-provided `published_at`;
- authors, tags, attachments, and icons.

## Alternative storage

- SQLite for simple single-node deployments, if a concrete need appears to run
  without external PostgreSQL.

## HTTP surface and publishing

- Huma, Chi, OpenAPI, and Swagger UI if the number of endpoints justifies a
  framework;
- Problem Details for a larger API;
- RSS, Atom, WebSub, and public feed pagination;
- application-level HTTP compression.

## Operations for later needs

- Prometheus metrics;
- a migration command and versioned migrations for future schema changes;
- migration modes and expected schema version checks;
- batched retention and an advisory lock when data volume or replica count
  requires them;
- `Last-Modified` and `If-Modified-Since`;
- a custom extended HTML sanitization policy;
- outgoing webhooks, queues, and Redis;
- full-text search;
- an image proxy;
- Kubernetes manifests;
- a debug container target;
- formal release automation, SBOM generation, image scanning, and signing;
- production backup, restore, and upgrade procedures after choosing the target
  deployment environment;
- a production smoke test with Miniflux, public routing, and TLS;
- multiple tokens, users, roles, and permissions;
- configuration for current hardcoded conventions, but only when a real
  deployment needs a different value.
