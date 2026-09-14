-- Guest admission is opt-in. Existing spaces are backfilled closed so a
-- deployment never silently broadens admission during this correction.
ALTER TABLE spaces
    ALTER COLUMN allow_guests SET DEFAULT false;

UPDATE spaces
SET allow_guests = false
WHERE allow_guests IS DISTINCT FROM false;
