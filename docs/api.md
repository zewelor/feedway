# HTTP API

Feedway exposes one authenticated publishing endpoint, one public JSON Feed,
and two operational probes. Paths are relative to the HTTP origin where the
container is published.

There is exactly one hardcoded feed named `Feedway`. It has no feed identifier,
feed-management API, landing page, `home_page_url`, or `feed_url`.

## Authentication

Only `POST /api/v1/entries` requires authentication. Send the deployment's
`API_TOKEN` as a Bearer token:

```http
Authorization: Bearer <API_TOKEN>
```

Missing or invalid credentials return `401 Unauthorized` with the common error
shape:

```json
{"error":"unauthorized"}
```

The response also includes:

```http
WWW-Authenticate: Bearer
```

The feed and probes are public. Put TLS and any additional access control at a
reverse proxy when the service is exposed beyond a trusted network.

## Publish an entry

### `POST /api/v1/entries`

Content type must be `application/json`. The request body is limited to 1 MiB,
unknown fields are rejected, and the JSON object contains:

| Field | Required | Limit | Description |
| --- | --- | --- | --- |
| `content_html` | yes | 256 KiB before and after sanitization | HTML body of the entry |
| `title` | no | 1,000 Unicode characters | Entry title |

Example:

```bash
curl --fail-with-body \
  --request POST \
  --header "Authorization: Bearer $API_TOKEN" \
  --header 'Content-Type: application/json' \
  --data '{
    "title": "Morning briefing",
    "content_html": "<h2>Today</h2><p>Three systems reported healthy.</p>"
  }' \
  http://localhost:8080/api/v1/entries
```

Feedway normalizes line endings and surrounding whitespace, sanitizes the HTML,
and derives an immutable `sha256-v1:<hex>` ID from the final title and HTML.
Publishing the same final content again is safe: the first request returns:

```http
201 Created
```

```json
{"result":"created","id":"sha256-v1:..."}
```

An identical retry returns `200 OK` with `"result":"deduplicated"` and the
same ID. Changed content creates a new entry. A producer that needs every run
to appear separately must include a run date, time, or another occurrence
marker in the title or content.

Expected error statuses are:

| Status | Meaning |
| --- | --- |
| `400` | Invalid JSON or more than one JSON value |
| `401` | Missing or invalid Bearer token |
| `413` | Request body exceeds 1 MiB |
| `415` | Content type is not JSON |
| `422` | Content is invalid, empty after sanitization, or exceeds a field limit |
| `500` | Unexpected publishing failure |

Application errors use one JSON shape:

```json
{"error":"content_html is required"}
```

Successful and error responses from the publishing endpoint use
`Content-Type: application/json; charset=utf-8`.

## Read the feed

### `GET /feed.json`

The response is [JSON Feed 1.1](https://www.jsonfeed.org/version/1.1/) with the
hardcoded feed title `Feedway`. It contains up to the latest 100 complete
entries, newest first. The top-level object contains `version`, `title`, and
`items`; each item contains `id`, `content_html`, `date_published`, and an
optional `title`.

Example:

```json
{
  "version": "https://jsonfeed.org/version/1.1",
  "title": "Feedway",
  "items": [
    {
      "id": "sha256-v1:...",
      "title": "Morning briefing",
      "content_html": "<p>Three systems reported healthy.</p>",
      "date_published": "2026-07-18T08:00:00Z"
    }
  ]
}
```

The uncompressed representation is limited to 16 MiB. If the newest 100 entries
do not fit, Feedway serves the newest complete entries that do fit; it never
serves a partial item. The response uses:

```http
Content-Type: application/feed+json; charset=utf-8
Cache-Control: public, max-age=60, must-revalidate
ETag: "<sha256-of-response>"
Content-Length: <response bytes>
X-Content-Type-Options: nosniff
```

Feedway does not emit `Last-Modified` or compress responses. A reverse proxy
may add compression.

If the feed cannot be loaded, the endpoint returns `500`. Feedway builds the
feed newest-first and stops before the first complete item that would exceed
the maximum size; that item and all older items are omitted. It never returns a
partial item or a feed-size error during normal feed generation.

### `HEAD /feed.json`

`HEAD` returns the same status and headers as `GET` without a response body.
Send the `ETag` value in `If-None-Match` to validate a cached response:

```bash
etag="$(curl --fail --silent --head http://localhost:8080/feed.json \
  | sed -n 's/^[Ee][Tt][Aa][Gg]:[[:space:]]*\(.*\)\r$/\1/p')"

curl --include \
  --header "If-None-Match: $etag" \
  http://localhost:8080/feed.json
```

An unchanged representation returns `304 Not Modified` with no body.

## Health and readiness

### `GET /healthz`

Returns `200 OK` when the process is alive. It does not query PostgreSQL.

### `GET /readyz`

Returns `200 OK` after startup has completed and PostgreSQL responds. It returns
`503 Service Unavailable` while startup is incomplete, PostgreSQL is unavailable,
or shutdown has started.

The `503` response uses the common error shape:

```json
{"error":"not ready"}
```

## Routing and errors

Unknown paths and unsupported methods use the standard Go `net/http` `404` and
`405` responses. They are not wrapped in Feedway's JSON error shape.
