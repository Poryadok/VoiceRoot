BEGIN;
-- Ordinary Bot-level uncertainty only: no retired Space/chat/actor history,
-- receipt authority or retention clock is encoded by this bit.
ALTER TABLE bots ADD COLUMN retired_cleanup_unresolved boolean NOT NULL DEFAULT false;
COMMIT;
