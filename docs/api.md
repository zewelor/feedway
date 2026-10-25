# HTTP API

Feedway exposes one authenticated publishing endpoint, one public JSON Feed,
public entry pages and optional JSON attachments, and two operational probes.
Paths are relative to the HTTP origin where the container is published.

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

The feed, individual entry pages, JSON attachments, and probes are public.
Put TLS and any additional access control at a reverse proxy when the service is exposed beyond
a trusted network.

## Publish an entry

### `POST /api/v1/entries`

Content type must be `application/json`. The request body is limited to 1 MiB,
unknown fields are rejected, and the JSON object contains:

| Field | Required | Limit | Description |
| --- | --- | --- | --- |
| `content_html` | yes | 256 KiB before and after sanitization | HTML body of the entry |
| `title` | no | 1,000 Unicode characters | Entry title |
| `data_json` | no | 256 KiB | Native JSON value accompanying the entry |

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
`data_json` does not affect the ID. All requests keep the existing ID algorithm.
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

### Optional structured data

`data_json` accepts an object, array, string, number, or boolean. For a new entry,
omitting the field or sending `null` means no attachment. JSON never changes
the entry ID. Empty objects and arrays are attachments. HTML
remains required; JSON is never sanitized as HTML or rendered into the entry page.

Example:

```json
{
  "title": "Morning briefing",
  "content_html": "<p>Three systems reported healthy.</p>",
  "data_json": {
    "healthy_systems": 3,
    "sources": ["https://example.org/status"]
  }
}
```

Send a native JSON value, not a string containing serialized JSON. Feedway does
not require a schema or interpret the document's fields. Its contents are public
through the attachment endpoint, under the same access model as the HTML page.

Feedway first checks the 256 KiB byte limit, then validates UTF-8 encoding and
JSON syntax. It stores the JSON value without decoding or reserializing it;
whitespace inside the value, key order, duplicate keys, number spellings, and
string escapes are preserved. Feedway does not interpret or normalize them.
The syntax check also rejects literal NUL bytes. An escaped `\u0000` is valid
JSON and remains escaped in storage and the attachment response.

### Known deduplication limitation

Only the final title and sanitized HTML identify an entry. Changing, adding,
or omitting `data_json` while keeping those fields unchanged returns
`deduplicated` and the same ID. The first successful publication wins: its JSON
attachment remains unchanged, including when that first entry had no attachment.
Deduplication never updates stored content or adds or removes an attachment.
This is a deliberate simplification; a producer that needs a new JSON document
must also change the final title or HTML, for example with an occurrence marker.

The field limit applies once to the raw JSON value's bytes, including quotes
and escapes. Data over the limit is rejected, never truncated. The complete
request must still fit within 1 MiB. Malformed JSON returns `400`; data exceeding
the field limit or using invalid UTF-8 returns `422`.

Expected error statuses are:

| Status | Meaning |
| --- | --- |
| `400` | Invalid JSON or more than one JSON value |
| `401` | Missing or invalid Bearer token |
| `413` | Request body exceeds 1 MiB |
| `415` | Content type is not JSON |
| `422` | HTML or JSON data violates validation rules or a field limit |
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
`items`; each item contains `id`, `url`, `content_html`, `date_published`, and
an optional `title`. When `BASE_URL` is set, `url` is an absolute public
permalink; otherwise it remains relative.

An item with structured data also contains one standard `attachments` entry:

```json
{
  "attachments": [
    {
      "url": "https://feed.example.com/entries/sha256-v1:.../data.json",
      "mime_type": "application/json"
    }
  ]
}
```

The attachment URL uses the same `BASE_URL` convention as entry permalinks.
Without it, the URL is relative: `/entries/{id}/data.json`. Items without data
omit `attachments`. The feed includes only the link, never the JSON document
itself. Attachment metadata counts toward the 16 MiB feed limit.

Example:

```json
{
  "version": "https://jsonfeed.org/version/1.1",
  "title": "Feedway",
  "items": [
    {
      "id": "sha256-v1:...",
      "url": "https://feed.example.com/entries/sha256-v1:...",
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

## Read an entry

### `GET /entries/{id}`

Returns one retained entry as a minimal HTML document. The page uses the entry
title for both `<title>` and `<h1>`; an entry without a title uses `Feedway` for
`<title>` and omits `<h1>`. The stored, sanitized `content_html` is rendered
inside `<article>` without additional styling or JavaScript.

JSON Feed items use this endpoint as their `url`. `BASE_URL` makes it absolute;
without that setting, it remains relative to the feed origin.

A successful response uses:

```http
Content-Type: text/html; charset=utf-8
Content-Length: <response bytes>
X-Content-Type-Options: nosniff
```

If the ID does not exist or retention has deleted the entry, the endpoint
returns `404`. A database or rendering failure returns `500` and is logged
without exposing internal details.

### `HEAD /entries/{id}`

`HEAD` returns the same status and headers as `GET` without a response body.

## Read an entry's JSON attachment

### `GET /entries/{id}/data.json`

Returns the retained entry's stored JSON value directly, without a wrapper
or double encoding. This endpoint is public. A successful response uses:

```http
Content-Type: application/json; charset=utf-8
Content-Length: <response bytes>
X-Content-Type-Options: nosniff
```

The endpoint does not render HTML, send `Content-Disposition`, or provide ETag
or conditional requests. The response can be any permitted JSON value, not
necessarily an object. It returns `404` for an unknown entry, an entry without
data, or data removed by retention. A database failure returns `500` and is
logged without exposing internal details.

### `HEAD /entries/{id}/data.json`

`HEAD` returns the same status and headers as `GET` without a response body.

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
