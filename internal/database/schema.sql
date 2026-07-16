CREATE TABLE IF NOT EXISTS entries (
    id           text PRIMARY KEY,
    title        text,
    content_html text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT entries_id_valid CHECK (
        id ~ '^sha256-v1:[0-9a-f]{64}$'
    ),
    CONSTRAINT entries_title_length CHECK (
        title IS NULL OR char_length(title) <= 1000
    ),
    CONSTRAINT entries_content_html_valid CHECK (
        nullif(btrim(content_html), '') IS NOT NULL
        AND octet_length(content_html) <= 262144
    )
);

CREATE INDEX IF NOT EXISTS entries_created_index
    ON entries(created_at DESC, id DESC);
