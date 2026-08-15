ALTER TABLE accounts
  ADD COLUMN IF NOT EXISTS guest_reminder_last_shown_at TIMESTAMPTZ NULL;
