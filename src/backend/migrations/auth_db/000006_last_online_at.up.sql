ALTER TABLE accounts
  ADD COLUMN IF NOT EXISTS last_online_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS accounts_guest_last_online_idx
  ON accounts (last_online_at)
  WHERE type = 'guest' AND status = 'active';
