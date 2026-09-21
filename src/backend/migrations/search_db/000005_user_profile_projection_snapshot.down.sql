ALTER TABLE search_user_profile_checkpoint
    DROP COLUMN IF EXISTS snapshot_cursor,
    DROP COLUMN IF EXISTS snapshot_high_watermark,
    DROP COLUMN IF EXISTS snapshot_phase;
