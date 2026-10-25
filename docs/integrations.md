# Integrations

Feedway accepts HTML from any HTTP client and publishes one feed for a reader.
These examples cover the two intended integration points.

## Publish from n8n

Many independent workflows can end with the same **HTTP Request** node. They
all publish into the same Miniflux subscription.

Configure the node with:

- **Method:** `POST`
- **URL:** `https://feed.example.com/api/v1/entries`
- **Authentication:** Header Auth credential containing
  `Authorization: Bearer <API_TOKEN>`
- **Body Content Type:** JSON
- **JSON Body:**

```javascript
{{ {
  title: $json.title,
  content_html: $json.content_html
} }}
```

Store the token in an n8n credential rather than directly in a workflow. The
upstream node only needs to produce `title` and `content_html`; an empty title
is allowed, while `content_html` is required.

See the complete request contract and status codes in the [HTTP API reference](api.md#publish-an-entry).

To attach structured data, include a native JSON value in the same body:

```javascript
{{ {
  title: $json.title,
  content_html: $json.content_html,
  data_json: $json.report
} }}
```

Do not apply `JSON.stringify` to `data_json`. Generate the readable HTML and
structured data from the same source when they describe the same report.
Feedway does not enforce that relationship or a document schema.
Retries keep the first stored attachment, even when JSON changes; see the
[known deduplication limitation](api.md#known-deduplication-limitation).

## Read it in Miniflux

After deploying Feedway somewhere Miniflux can reach, add this subscription:

```text
https://feed.example.com/feed.json
```

Set `BASE_URL` to the public Feedway origin when Miniflux needs absolute entry
permalinks, for example `https://feed.example.com`. Each JSON Feed item then
links to `https://feed.example.com/entries/{id}`. Without `BASE_URL`, Feedway
keeps using a relative `/entries/{id}` permalink.

Entries with `data_json` advertise an `application/json` attachment pointing to
`/entries/{id}/data.json`, using `BASE_URL` when configured. A machine consumer
can discover the attachment in the feed and fetch the JSON directly. HTML-only
entries have no attachment. Consumers must handle missing or expired data
explicitly; the attachment expires with the Feedway entry even if a reader
retains its HTML.

The production Miniflux smoke test is intentionally left until the first real
deployment, where network routing and TLS can be verified together.

Attachment display also needs a controlled smoke test against the deployed
Miniflux version before releasing this feature. Local HTTP/PostgreSQL tests do
not prove the reader's attachment UI or production routing.

## Verify locally with curl

Read the public feed directly:

```bash
curl --fail http://localhost:8080/feed.json
```

Copy an item's `url` from that response to verify its public HTML page:

```bash
curl --fail http://localhost:8080/entries/sha256-v1:...
```

To verify conditional requests, fetch the `ETag` with `HEAD` and send it back in
`If-None-Match`. An unchanged feed returns `304 Not Modified` without a body.
The complete command pair is in [Read the feed](api.md#read-the-feed).
