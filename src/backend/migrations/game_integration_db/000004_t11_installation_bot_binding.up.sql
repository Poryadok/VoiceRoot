-- Cross-service Bot IDs are logical references; GIS never queries Bot's DB.
ALTER TABLE installations ADD COLUMN bot_id UUID;

CREATE INDEX installations_active_bot_authority_idx
    ON installations (application_id, environment_id, bot_id)
    WHERE status = 'active' AND bot_id IS NOT NULL;
