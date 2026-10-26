CREATE TABLE IF NOT EXISTS entries (
    id           text PRIMARY KEY,
    title        text,
    content_html text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),

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

ALTER TABLE entries ADD COLUMN IF NOT EXISTS data_json json
    CHECK (octet_length(data_json::text) <= 262144);

-- The final ID constraint marks the atomic completion of this upgrade.
DO $migration$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'entries'::regclass AND conname = 'entries_public_id_valid'
    ) THEN
        ALTER TABLE entries ADD COLUMN IF NOT EXISTS feed text
            CHECK (octet_length(feed) BETWEEN 1 AND 64 AND feed ~ '^[a-z0-9]+(-[a-z0-9]+)*$');

        ALTER TABLE entries ADD COLUMN IF NOT EXISTS content_hash text
            CHECK (content_hash ~ '^sha256-v1:[0-9a-f]{64}$');

        -- Legacy IDs already contain the content hash. Never rewrite public IDs.
        UPDATE entries SET content_hash = id WHERE content_hash IS NULL;
        ALTER TABLE entries ALTER COLUMN content_hash SET NOT NULL;
        ALTER TABLE entries ALTER COLUMN id SET DEFAULT uuidv7()::text;

        -- The original schema used both names across earlier versions.
        ALTER TABLE entries DROP CONSTRAINT IF EXISTS entries_id_check;
        ALTER TABLE entries DROP CONSTRAINT IF EXISTS entries_id_valid;
        ALTER TABLE entries ADD CONSTRAINT entries_public_id_valid CHECK (
            id ~ '^sha256-v1:[0-9a-f]{64}$'
            OR id ~ '^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
        );

        CREATE UNIQUE INDEX IF NOT EXISTS entries_feed_content_hash_key
            ON entries(feed, content_hash) NULLS NOT DISTINCT;

        CREATE INDEX IF NOT EXISTS entries_feed_created_index
            ON entries(feed, created_at DESC, id DESC);
    END IF;
END;
$migration$;
