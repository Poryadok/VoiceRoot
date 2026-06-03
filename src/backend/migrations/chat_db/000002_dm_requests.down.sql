DROP INDEX IF EXISTS chat_members_profile_inbox_idx;

ALTER TABLE chat_members
    DROP COLUMN IF EXISTS inbox_bucket;
