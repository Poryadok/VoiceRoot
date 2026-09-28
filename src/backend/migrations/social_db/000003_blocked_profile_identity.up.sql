ALTER TABLE blocks
    ADD COLUMN blocked_profile_id UUID,
    ADD COLUMN blocked_display_name TEXT,
    ADD COLUMN blocked_username TEXT,
    ADD COLUMN blocked_discriminator TEXT;
