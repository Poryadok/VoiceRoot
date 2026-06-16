-- BOT-B: track which profile created a custom role (bot uninstall cleanup).
ALTER TABLE roles ADD COLUMN IF NOT EXISTS created_by_profile_id UUID NULL;

CREATE INDEX IF NOT EXISTS roles_space_created_by_profile_idx
    ON roles (space_id, created_by_profile_id)
    WHERE created_by_profile_id IS NOT NULL;
