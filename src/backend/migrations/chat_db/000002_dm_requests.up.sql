ALTER TABLE chat_members
    ADD COLUMN IF NOT EXISTS inbox_bucket VARCHAR(16) NOT NULL DEFAULT 'main'
    CHECK (inbox_bucket IN ('main', 'requests', 'declined'));

CREATE INDEX IF NOT EXISTS chat_members_profile_inbox_idx
    ON chat_members (profile_id, inbox_bucket);
