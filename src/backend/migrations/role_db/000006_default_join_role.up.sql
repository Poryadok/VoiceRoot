-- Configurable default join role per space (docs/features/roles.md).
ALTER TABLE roles ADD COLUMN IF NOT EXISTS is_default_join BOOLEAN NOT NULL DEFAULT false;

CREATE UNIQUE INDEX roles_space_default_join_uidx
    ON roles (space_id)
    WHERE is_default_join = true;

-- Member is the default join role for existing spaces.
UPDATE roles SET is_default_join = true, updated_at = now()
WHERE is_system = true AND name = 'Member';
