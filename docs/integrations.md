# Integrations

Feedway accepts HTML from any HTTP client and publishes an aggregate feed or
filtered feeds for readers. These examples cover the two intended integration
points.

## Publish from n8n

Many independent workflows can end with the same **HTTP Request** node. They
can all publish into the aggregate feed, or use the same identifier to publish
into one selected Miniflux subscription.

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
  content_html: $json.content_html,
  feed: "daily-briefing"
} }}
```

Store the token in an n8n credential rather than directly in a workflow. The
upstream node must produce `content_html`; `title` is optional and an empty
title is allowed. `feed` is optional; use a lower-case slug such as
`daily-briefing` to classify the entry. The identifier needs no prior setup.
See the [HTTP API reference](api.md#feed-identifiers) for validation rules.

See the complete request contract and status codes in the [HTTP API reference](api.md#publish-an-entry).

To attach structured data, include a native JSON value in the same body:

```javascript
{{ {
  title: $json.title,
  content_html: $json.content_html,
  feed: "daily-briefing",
  data_json: $json.report
} }}
```

Do not apply `JSON.stringify` to `data_json`. Generate the readable HTML and
structured data from the same source when they describe the same report.
Feedway does not enforce that relationship or a document schema.
Retries keep the first stored attachment, even when JSON changes; see the
[known deduplication limitation](api.md#known-deduplication-limitation).

## Read it in Miniflux

After deploying Feedway somewhere Miniflux can reach, add the aggregate
subscription to read every entry:

```text
https://feed.example.com/feed.json
```

To keep a selected group separate, add its feed URL instead, for example:

```text
https://feed.example.com/feeds/daily-briefing/feed.json
```

Feedway does not require a feed to be registered first. A valid identifier
with no retained entries returns an empty feed. The aggregate feed is titled
`Feedway`; a filtered feed uses its identifier as the title.

Set `BASE_URL` to the public Feedway origin when Miniflux needs absolute entry
permalinks, for example `https://feed.example.com`. Each JSON Feed item then
links to `https://feed.example.com/entries/{id}`. Without `BASE_URL`, Feedway
uses an origin-root-relative `/entries/{id}` permalink, including from the
nested feed URL.

With `BASE_URL` set, each feed also advertises its own absolute `feed_url`.
The shared `home_page_url` points to the Feedway home page at `/`, where users
can browse available feeds. Without `BASE_URL`, both metadata fields are omitted.

Entries with `data_json` advertise an `application/json` attachment pointing to
`/entries/{id}/data.json`, using `BASE_URL` when configured. Without it, this
URL is also origin-root-relative. A machine consumer
can discover the attachment in the feed and fetch the JSON directly. HTML-only
entries have no attachment. Consumers must handle missing or expired data
explicitly; the attachment expires with the Feedway entry even if a reader
retains its HTML.

The feed also provides `_feedway.truncated` to signal that response limits omit
older entries still in storage. This metadata does not change how entries or
attachments are read. Feedway does not display a warning in Miniflux or modify
other consumers; see [feed completeness](api.md#feed-completeness).

The production Miniflux smoke test is intentionally left until the first real
deployment, where network routing and TLS can be verified together.

Attachment display also needs a controlled smoke test against the deployed
Miniflux version before releasing this feature. Local HTTP/PostgreSQL tests do
not prove the reader's attachment UI or production routing.

## Verify locally with curl

Read the public feed directly:

```bash
curl --fail http://localhost:8080/feed.json
curl --fail http://localhost:8080/feeds/daily-briefing/feed.json
```

Copy an item's `url` from that response to verify its public HTML page:

```bash
curl --fail http://localhost:8080/entries/0190abcd-1234-7abc-8def-0123456789ab
```

To verify conditional requests, fetch the `ETag` with `HEAD` and send it back in
`If-None-Match`. An unchanged feed returns `304 Not Modified` without a body.
The complete command pair is in [Read the feed](api.md#read-the-feed).
