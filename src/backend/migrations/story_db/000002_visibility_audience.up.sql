-- Per-story audience JSON (privacy.md multiselect) and close_friends visibility.

ALTER TABLE stories
    ADD COLUMN IF NOT EXISTS visibility_audience JSONB;

ALTER TABLE stories DROP CONSTRAINT IF EXISTS stories_visibility_check;
ALTER TABLE stories ADD CONSTRAINT stories_visibility_check
    CHECK (visibility IN ('everyone', 'friends', 'close_friends', 'custom'));
