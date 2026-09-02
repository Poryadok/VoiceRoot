-- Soft-delete DM for one profile (DeleteChat for self) — peer membership unchanged.
ALTER TABLE chat_members
    ADD COLUMN IF NOT EXISTS deleted_for_self BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS chat_members_profile_active_idx
    ON chat_members (profile_id, joined_at DESC)
    WHERE deleted_for_self = false;
