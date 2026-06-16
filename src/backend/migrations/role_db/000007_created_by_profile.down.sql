DROP INDEX IF EXISTS roles_space_created_by_profile_idx;
ALTER TABLE roles DROP COLUMN IF EXISTS created_by_profile_id;
