DROP INDEX IF EXISTS chat_members_profile_active_idx;
ALTER TABLE chat_members DROP COLUMN IF EXISTS deleted_for_self;
