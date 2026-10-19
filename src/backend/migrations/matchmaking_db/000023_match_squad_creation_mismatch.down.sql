BEGIN;
LOCK TABLE matchmaking_match_squad_operations IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM matchmaking_match_squad_operations WHERE
  chat_creation_mismatch_bytes IS NOT NULL OR voice_creation_mismatch_bytes IS NOT NULL) THEN
  RAISE EXCEPTION 'cannot remove creation mismatch evidence';
 END IF;
END $$;
DROP TRIGGER match_squad_mismatch_completion_guard ON matchmaking_match_squad_teardowns;
DROP FUNCTION guard_match_squad_mismatch_completion();
DROP TRIGGER match_squad_creation_mismatch_guard ON matchmaking_match_squad_operations;
DROP FUNCTION guard_match_squad_creation_mismatch();
ALTER TABLE matchmaking_match_squad_operations
 DROP COLUMN creation_evidence_version,
 DROP COLUMN chat_creation_mismatch_bytes,
 DROP COLUMN chat_creation_mismatch_sha256,
 DROP COLUMN chat_creation_mismatch_at,
 DROP COLUMN voice_creation_mismatch_bytes,
 DROP COLUMN voice_creation_mismatch_sha256,
 DROP COLUMN voice_creation_mismatch_at;
COMMIT;
