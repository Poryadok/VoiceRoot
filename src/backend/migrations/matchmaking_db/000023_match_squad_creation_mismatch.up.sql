BEGIN;
ALTER TABLE matchmaking_match_squad_operations
 ADD COLUMN creation_evidence_version SMALLINT NOT NULL DEFAULT 1 CHECK (creation_evidence_version=1),
 ADD COLUMN chat_creation_mismatch_bytes BYTEA,
 ADD COLUMN chat_creation_mismatch_sha256 BYTEA,
 ADD COLUMN chat_creation_mismatch_at TIMESTAMPTZ,
 ADD COLUMN voice_creation_mismatch_bytes BYTEA,
 ADD COLUMN voice_creation_mismatch_sha256 BYTEA,
 ADD COLUMN voice_creation_mismatch_at TIMESTAMPTZ,
 ADD CONSTRAINT chat_creation_mismatch_evidence CHECK (
  (chat_creation_mismatch_bytes IS NULL AND chat_creation_mismatch_sha256 IS NULL AND chat_creation_mismatch_at IS NULL) OR
  (chat_creation_mismatch_bytes IS NOT NULL AND octet_length(chat_creation_mismatch_bytes)>0 AND
   chat_creation_mismatch_sha256 IS NOT NULL AND chat_creation_mismatch_sha256=sha256(chat_creation_mismatch_bytes) AND chat_creation_mismatch_at IS NOT NULL)),
 ADD CONSTRAINT voice_creation_mismatch_evidence CHECK (
  (voice_creation_mismatch_bytes IS NULL AND voice_creation_mismatch_sha256 IS NULL AND voice_creation_mismatch_at IS NULL) OR
  (voice_creation_mismatch_bytes IS NOT NULL AND octet_length(voice_creation_mismatch_bytes)>0 AND
   voice_creation_mismatch_sha256 IS NOT NULL AND voice_creation_mismatch_sha256=sha256(voice_creation_mismatch_bytes) AND voice_creation_mismatch_at IS NOT NULL));

CREATE FUNCTION guard_match_squad_creation_mismatch() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.chat_creation_mismatch_bytes IS NOT NULL AND
  (NEW.chat_creation_mismatch_bytes,NEW.chat_creation_mismatch_sha256,NEW.chat_creation_mismatch_at)
  IS DISTINCT FROM (OLD.chat_creation_mismatch_bytes,OLD.chat_creation_mismatch_sha256,OLD.chat_creation_mismatch_at) THEN
  RAISE EXCEPTION 'cannot rewrite creation mismatch evidence';
 END IF;
 IF OLD.voice_creation_mismatch_bytes IS NOT NULL AND
  (NEW.voice_creation_mismatch_bytes,NEW.voice_creation_mismatch_sha256,NEW.voice_creation_mismatch_at)
  IS DISTINCT FROM (OLD.voice_creation_mismatch_bytes,OLD.voice_creation_mismatch_sha256,OLD.voice_creation_mismatch_at) THEN
  RAISE EXCEPTION 'cannot rewrite creation mismatch evidence';
 END IF;
 IF NEW.chat_creation_mismatch_bytes IS NOT NULL OR NEW.voice_creation_mismatch_bytes IS NOT NULL THEN
  IF (NEW.state IS DISTINCT FROM OLD.state AND NEW.state IN ('active','closed')) OR
   (OLD.chat_dispatch_state='NOT_ISSUED' AND NEW.chat_dispatch_state<>'NOT_ISSUED') OR
   (OLD.voice_dispatch_state='NOT_ISSUED' AND NEW.voice_dispatch_state<>'NOT_ISSUED') THEN
   RAISE EXCEPTION 'creation mismatch requires reconciliation';
  END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER match_squad_creation_mismatch_guard BEFORE UPDATE ON matchmaking_match_squad_operations
 FOR EACH ROW EXECUTE FUNCTION guard_match_squad_creation_mismatch();

CREATE FUNCTION guard_match_squad_mismatch_completion() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.state='complete' AND OLD.state IS DISTINCT FROM NEW.state AND EXISTS (
  SELECT 1 FROM matchmaking_match_squad_operations WHERE match_id=NEW.match_id AND
   (chat_creation_mismatch_bytes IS NOT NULL OR voice_creation_mismatch_bytes IS NOT NULL)) THEN
  RAISE EXCEPTION 'creation mismatch requires reconciliation';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER match_squad_mismatch_completion_guard BEFORE UPDATE ON matchmaking_match_squad_teardowns
 FOR EACH ROW EXECUTE FUNCTION guard_match_squad_mismatch_completion();
COMMIT;
