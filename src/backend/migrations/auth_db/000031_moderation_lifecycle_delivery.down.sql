-- Never silently discard a lifecycle projection or outstanding terminal fact.
DO $$ BEGIN
  IF EXISTS(SELECT 1 FROM moderation_lifecycle_current) OR
     EXISTS(SELECT 1 FROM moderation_lifecycle_delivery) OR
     EXISTS(SELECT 1 FROM moderation_terminal_subjects) OR
     EXISTS(SELECT 1 FROM moderation_terminal_delivery) THEN
    RAISE EXCEPTION 'moderation lifecycle authority cannot be discarded';
  END IF;
END $$;
DROP TRIGGER auth_moderation_capture_terminal ON moderation_terminal_subjects;
DROP FUNCTION auth_moderation_capture_terminal();
DROP TRIGGER auth_moderation_capture_lifecycle ON accounts;
DROP FUNCTION auth_moderation_capture_lifecycle();
DROP TABLE moderation_terminal_delivery;
DROP INDEX moderation_terminal_event_id_idx;
ALTER TABLE moderation_terminal_subjects DROP COLUMN event_id;
DROP TABLE moderation_lifecycle_delivery;
DROP TABLE moderation_lifecycle_current;
