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

## Read it in Miniflux

After deploying Feedway somewhere Miniflux can reach, add this subscription:

```text
https://feed.example.com/feed.json
```

Each JSON Feed item contains a relative `/entries/{id}` permalink. Miniflux
resolves it against the feed origin, so its external link opens the retained
entry in Feedway. No `BASE_URL`, `home_page_url`, or `feed_url` is required.

The production Miniflux smoke test is intentionally left until the first real
deployment, where network routing and TLS can be verified together.

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
