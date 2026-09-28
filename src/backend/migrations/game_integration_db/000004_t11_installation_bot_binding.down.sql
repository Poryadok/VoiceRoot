DROP INDEX IF EXISTS installations_active_bot_authority_idx;
ALTER TABLE installations DROP COLUMN IF EXISTS bot_id;
