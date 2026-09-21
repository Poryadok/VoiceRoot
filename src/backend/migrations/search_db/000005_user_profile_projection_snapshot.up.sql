-- Durable bootstrap state. A Search restart must resume the same immutable
-- User snapshot rather than reconstruct it from whichever profile rows exist.
ALTER TABLE search_user_profile_checkpoint
    ADD COLUMN IF NOT EXISTS snapshot_phase TEXT NOT NULL DEFAULT 'idle'
        CHECK (snapshot_phase IN ('idle', 'snapshot', 'replay')),
    ADD COLUMN IF NOT EXISTS snapshot_high_watermark BIGINT NOT NULL DEFAULT 0
        CHECK (snapshot_high_watermark >= 0),
    ADD COLUMN IF NOT EXISTS snapshot_cursor TEXT NOT NULL DEFAULT '';
