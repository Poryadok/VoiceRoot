-- Even revision 1 may have been observed through the read-only resolver.
-- A populated File store therefore requires forward correction, not a reset.
DO $$
BEGIN
  LOCK TABLE files, file_preview_event_outbox IN ACCESS EXCLUSIVE MODE NOWAIT;
  IF current_setting('transaction_isolation') <> 'read committed' THEN
    RAISE EXCEPTION 'preview rollback requires fresh READ COMMITTED checks'
      USING ERRCODE = '55000';
  END IF;
  IF EXISTS (SELECT 1 FROM files)
     OR EXISTS (SELECT 1 FROM file_preview_event_outbox) THEN
    RAISE EXCEPTION 'preview rollback would erase File authority or event evidence'
      USING ERRCODE = '55000';
  END IF;
  DROP TABLE file_preview_event_outbox;
  ALTER TABLE files DROP COLUMN preview_metadata_revision;
END $$;
