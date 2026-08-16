-- User game catalog requests: pending_moderation / rejected (docs/features/game-catalog.md П.4).

ALTER TABLE games ALTER COLUMN status TYPE VARCHAR(32);

ALTER TABLE games DROP CONSTRAINT IF EXISTS games_status_check;
ALTER TABLE games
    ADD CONSTRAINT games_status_check
    CHECK (status IN ('active', 'archived', 'pending_moderation', 'rejected'));

DROP INDEX IF EXISTS games_name_active_uq;
CREATE UNIQUE INDEX games_name_active_or_pending_uq
    ON games (lower(name))
    WHERE status IN ('active', 'pending_moderation');
