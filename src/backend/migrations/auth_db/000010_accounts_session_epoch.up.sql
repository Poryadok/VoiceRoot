ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS session_epoch BIGINT NOT NULL DEFAULT 1 CHECK (session_epoch > 0);
