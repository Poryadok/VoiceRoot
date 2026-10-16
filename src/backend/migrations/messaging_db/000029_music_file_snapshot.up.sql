-- Preserve the latest File-owned plaintext Music metadata even before the
-- first message insert, so a delayed send cannot outlive a processed tombstone.
CREATE TABLE messaging_music_file_snapshots (
    file_id UUID PRIMARY KEY,
    source_chat_id UUID NOT NULL,
    metadata_revision BIGINT NOT NULL CHECK (metadata_revision > 0),
    status TEXT NOT NULL CHECK (status IN ('ready', 'deleted', 'expired')),
    size_bytes BIGINT NOT NULL,
    scan_result TEXT NOT NULL,
    duration_seconds BIGINT NOT NULL,
    title TEXT NOT NULL,
    artist TEXT NOT NULL,
    album TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK (
        (status = 'ready' AND size_bytes BETWEEN 1 AND 10485760
         AND scan_result IN ('clean', 'skipped') AND duration_seconds > 0)
        OR
        (status IN ('deleted', 'expired') AND size_bytes = 0
         AND scan_result = '' AND duration_seconds = 0
         AND title = '' AND artist = '' AND album = '')
    )
);
