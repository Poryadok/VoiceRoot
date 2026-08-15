-- Fix global settings: PK made scope_id NOT NULL; allow NULL + partial unique indexes.
ALTER TABLE notification_settings DROP CONSTRAINT IF EXISTS notification_settings_pkey;
ALTER TABLE notification_settings ALTER COLUMN scope_id DROP NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS notification_settings_global_uq
  ON notification_settings (profile_id, scope_type)
  WHERE scope_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS notification_settings_scoped_uq
  ON notification_settings (profile_id, scope_type, scope_id)
  WHERE scope_id IS NOT NULL;
