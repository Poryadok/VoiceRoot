ALTER TABLE blocks
    DROP COLUMN blocked_discriminator,
    DROP COLUMN blocked_username,
    DROP COLUMN blocked_display_name,
    DROP COLUMN blocked_profile_id;
