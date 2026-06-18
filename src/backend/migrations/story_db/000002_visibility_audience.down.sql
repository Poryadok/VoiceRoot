ALTER TABLE stories DROP CONSTRAINT IF EXISTS stories_visibility_check;
ALTER TABLE stories ADD CONSTRAINT stories_visibility_check
    CHECK (visibility IN ('everyone', 'friends', 'custom'));

ALTER TABLE stories DROP COLUMN IF EXISTS visibility_audience;
