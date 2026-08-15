DROP INDEX IF EXISTS notification_settings_scoped_uq;
DROP INDEX IF EXISTS notification_settings_global_uq;

DELETE FROM notification_settings WHERE scope_id IS NULL;

ALTER TABLE notification_settings ALTER COLUMN scope_id SET NOT NULL;
ALTER TABLE notification_settings
  ADD CONSTRAINT notification_settings_pkey PRIMARY KEY (profile_id, scope_type, scope_id);
