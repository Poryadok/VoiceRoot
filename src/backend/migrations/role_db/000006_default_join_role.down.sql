DROP INDEX IF EXISTS roles_space_default_join_uidx;
ALTER TABLE roles DROP COLUMN IF EXISTS is_default_join;
