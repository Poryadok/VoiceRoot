-- Historical rows are not replayed as new sends. The application activates this
-- monotonic guard only after its current-boot terminal/source reconciliation.
CREATE TABLE automod_producer_state (
  singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
  source_epoch uuid NOT NULL DEFAULT gen_random_uuid(),
  format_version smallint NOT NULL DEFAULT 2 CHECK(format_version=2),
  active boolean NOT NULL DEFAULT false
);
INSERT INTO automod_producer_state DEFAULT VALUES;
CREATE FUNCTION automod_producer_state_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP='DELETE' OR NEW.source_epoch IS DISTINCT FROM OLD.source_epoch OR NEW.format_version IS DISTINCT FROM OLD.format_version OR (OLD.active AND NOT NEW.active) THEN
    RAISE EXCEPTION 'automod producer epoch and activation cannot be reset';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER automod_producer_state_guard BEFORE UPDATE OR DELETE ON automod_producer_state
FOR EACH ROW EXECUTE FUNCTION automod_producer_state_guard();

CREATE TABLE automod_terminal_subjects (
  subject_id uuid PRIMARY KEY,
  purged_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE automod_source_subjects (
  subject_id uuid PRIMARY KEY,
  account_id uuid NOT NULL UNIQUE,
  lifecycle_revision bigint NOT NULL CHECK(lifecycle_revision>0),
  through_sequence bigint NOT NULL DEFAULT 0 CHECK(through_sequence>=0),
  last_source_at timestamptz,
  CHECK((through_sequence=0)=(last_source_at IS NULL))
);
CREATE TABLE automod_hmac_keys (
  key_id text PRIMARY KEY CHECK(key_id ~ '^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$'),
  fingerprint bytea NOT NULL CHECK(octet_length(fingerprint)=32),
  retired_at timestamptz
);
CREATE FUNCTION automod_hmac_key_identity_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP='DELETE' OR NEW.key_id IS DISTINCT FROM OLD.key_id OR NEW.fingerprint IS DISTINCT FROM OLD.fingerprint OR
     (OLD.retired_at IS NOT NULL AND NEW.retired_at IS DISTINCT FROM OLD.retired_at) THEN
    RAISE EXCEPTION 'automod HMAC key identity and retirement cannot be reset';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER automod_hmac_key_identity_guard BEFORE UPDATE OR DELETE ON automod_hmac_keys
FOR EACH ROW EXECUTE FUNCTION automod_hmac_key_identity_guard();
CREATE TABLE automod_hmac_usage (
  subject_id uuid NOT NULL REFERENCES automod_source_subjects(subject_id) ON DELETE CASCADE,
  key_id text NOT NULL REFERENCES automod_hmac_keys(key_id),
  last_source_at timestamptz NOT NULL,
  PRIMARY KEY(subject_id,key_id)
);
CREATE TABLE automod_evidence_outbox (
  event_id uuid PRIMARY KEY,
  subject_id uuid NOT NULL REFERENCES automod_source_subjects(subject_id) ON DELETE CASCADE,
  sequence bigint NOT NULL CHECK(sequence>0),
  source_at timestamptz NOT NULL,
  core_sha256 bytea NOT NULL CHECK(octet_length(core_sha256)=32),
  payload bytea CHECK(payload IS NULL OR octet_length(payload) BETWEEN 1 AND 16384),
  lease_id uuid,
  lease_until timestamptz,
  UNIQUE(subject_id,sequence)
);
CREATE INDEX automod_evidence_outbox_delivery ON automod_evidence_outbox(source_at,event_id);
-- Erasable local coverage only. Core delivery and counting never join this
-- bridge. The independent chat selector also covers orphaned message rows.
CREATE TABLE automod_message_coverage (
  event_id uuid PRIMARY KEY REFERENCES automod_evidence_outbox(event_id) ON DELETE CASCADE,
  message_id uuid NOT NULL UNIQUE,
  companion_id uuid UNIQUE,
  chat_id uuid NOT NULL,
  CHECK(companion_id IS NULL OR companion_id<>message_id)
);
CREATE INDEX automod_message_coverage_chat ON automod_message_coverage(chat_id);
CREATE FUNCTION automod_coverage_identity_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'automod coverage links cannot be rewritten';
END $$;
CREATE TRIGGER automod_coverage_identity_guard BEFORE UPDATE ON automod_message_coverage
FOR EACH ROW EXECUTE FUNCTION automod_coverage_identity_guard();
CREATE FUNCTION automod_remove_deleted_message_coverage() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  DELETE FROM automod_message_coverage WHERE message_id=OLD.id OR companion_id=OLD.id;
  RETURN NULL;
END $$;
CREATE TRIGGER automod_remove_deleted_message_coverage AFTER DELETE ON messages
FOR EACH ROW EXECUTE FUNCTION automod_remove_deleted_message_coverage();
CREATE TABLE automod_expired_receipts (
  event_id uuid PRIMARY KEY REFERENCES automod_evidence_outbox(event_id) ON DELETE CASCADE,
  payload bytea NOT NULL CHECK(octet_length(payload) BETWEEN 1 AND 16384)
);

CREATE FUNCTION automod_evidence_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP='INSERT' THEN
    IF NEW.payload IS NULL THEN RAISE EXCEPTION 'new automod evidence payload is required'; END IF;
    RETURN NEW;
  END IF;
  IF (NEW.event_id,NEW.subject_id,NEW.sequence,NEW.source_at,NEW.core_sha256)
      IS DISTINCT FROM (OLD.event_id,OLD.subject_id,OLD.sequence,OLD.source_at,OLD.core_sha256) OR
     (NEW.payload IS DISTINCT FROM OLD.payload AND NOT (NEW.payload IS NULL AND
        EXISTS(SELECT 1 FROM automod_expired_receipts r WHERE r.event_id=OLD.event_id))) THEN
    RAISE EXCEPTION 'committed automod evidence is immutable';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER automod_evidence_immutable BEFORE INSERT OR UPDATE ON automod_evidence_outbox
FOR EACH ROW EXECUTE FUNCTION automod_evidence_immutable();

-- Direct SQL, old pods and forgotten enabled producers cannot bypass evidence
-- after activation. A forward's private companion is covered by its one primary
-- event; rows predating activation are never backfilled or re-counted.
CREATE FUNCTION automod_message_coverage() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF EXISTS(SELECT 1 FROM automod_producer_state WHERE singleton AND active) AND
     NOT EXISTS(SELECT 1 FROM automod_message_coverage e WHERE e.chat_id=NEW.chat_id AND (e.message_id=NEW.id OR e.companion_id=NEW.id)) THEN
    RAISE EXCEPTION 'committed message requires private automod evidence';
  END IF;
  RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER automod_message_coverage AFTER INSERT ON messages
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION automod_message_coverage();
