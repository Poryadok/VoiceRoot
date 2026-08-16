DROP INDEX IF EXISTS games_name_active_or_pending_uq;

DELETE FROM games WHERE status IN ('pending_moderation', 'rejected');

ALTER TABLE games DROP CONSTRAINT IF EXISTS games_status_check;
ALTER TABLE games
    ADD CONSTRAINT games_status_check
    CHECK (status IN ('active', 'archived'));

CREATE UNIQUE INDEX games_name_active_uq ON games (lower(name)) WHERE status = 'active';
