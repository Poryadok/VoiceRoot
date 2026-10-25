-- Rollback requires an empty ledger; never silently destroy result fences.
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM space_chat_creation_fences) OR EXISTS(SELECT 1 FROM space_permanent_purge_fences) THEN
  RAISE EXCEPTION 'cannot remove nonempty composite/purge fences';
 END IF;
END $$;
DROP TABLE space_chat_creation_evidence;
DROP TABLE space_chat_creation_fences;
DROP TABLE space_permanent_purge_fences;
