-- P1 installs only the independently verified current Auth generation.
CREATE TABLE automod_lifecycle_authority (
  singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
  authority_generation uuid NOT NULL CHECK(authority_generation <> '00000000-0000-0000-0000-000000000000'::uuid)
);
-- Opaque complete Auth projection, including facts whose raw binding is absent.
CREATE TABLE automod_lifecycle_current (
  subject_id uuid PRIMARY KEY,
  authority_generation uuid NOT NULL,
  event_id uuid NOT NULL,
  lifecycle_revision bigint NOT NULL CHECK(lifecycle_revision>0),
  lifecycle_state text NOT NULL CHECK(lifecycle_state IN ('LIVE','DELETED')),
  deletion_cycle_id uuid,
  purge_at timestamptz,
  canonical_fact bytea NOT NULL CHECK(octet_length(canonical_fact) BETWEEN 1 AND 1024),
  fact_sha256 bytea NOT NULL CHECK(octet_length(fact_sha256)=32),
  CHECK((lifecycle_state='LIVE' AND deletion_cycle_id IS NULL AND purge_at IS NULL) OR
        (lifecycle_state='DELETED' AND deletion_cycle_id IS NOT NULL AND
         deletion_cycle_id <> '00000000-0000-0000-0000-000000000000'::uuid AND purge_at IS NOT NULL))
);
CREATE TRIGGER automod_projection_cycle_deadline BEFORE UPDATE ON automod_lifecycle_current
FOR EACH ROW EXECUTE FUNCTION automod_guard_cycle_deadline();
CREATE FUNCTION automod_lifecycle_binding_guard() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE current_fact automod_lifecycle_current;
BEGIN
  PERFORM pg_advisory_xact_lock(hashtextextended('moderation-subject-lifecycle:' || NEW.subject_id::text,0));
  SELECT * INTO current_fact FROM automod_lifecycle_current WHERE subject_id=NEW.subject_id;
  IF FOUND AND (NEW.lifecycle_revision < current_fact.lifecycle_revision OR
    (NEW.lifecycle_revision=current_fact.lifecycle_revision AND
      (NEW.lifecycle_state IS DISTINCT FROM current_fact.lifecycle_state OR
       NEW.deletion_cycle_id IS DISTINCT FROM current_fact.deletion_cycle_id OR
       NEW.purge_at IS DISTINCT FROM current_fact.purge_at))) THEN
    RAISE EXCEPTION USING ERRCODE='40001', MESSAGE='automod_lifecycle_projection_conflict';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER automod_lifecycle_binding_guard BEFORE INSERT OR UPDATE ON automod_subjects
FOR EACH ROW EXECUTE FUNCTION automod_lifecycle_binding_guard();
CREATE FUNCTION automod_lifecycle_terminal_cleanup() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  DELETE FROM automod_lifecycle_current WHERE subject_id=NEW.subject_id;
  RETURN NEW;
END $$;
CREATE TRIGGER automod_lifecycle_terminal_cleanup AFTER INSERT OR UPDATE ON automod_terminal_subjects
FOR EACH ROW EXECUTE FUNCTION automod_lifecycle_terminal_cleanup();
