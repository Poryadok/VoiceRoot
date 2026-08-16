ALTER TABLE linked_identities
    ADD COLUMN IF NOT EXISTS profile_id UUID;
