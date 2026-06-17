DROP INDEX IF EXISTS accounts_guest_last_online_idx;
ALTER TABLE accounts DROP COLUMN IF EXISTS last_online_at;
