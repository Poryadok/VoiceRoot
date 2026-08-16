-- Profile that received the verification badge for this linked identity (verification.md cron refresh).
ALTER TABLE linked_identities
    ADD COLUMN IF NOT EXISTS profile_id UUID;
