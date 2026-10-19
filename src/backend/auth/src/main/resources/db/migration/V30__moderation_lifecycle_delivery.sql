-- Private Auth-owned lifecycle projection and independent recipient intents.
-- No raw account/profile IDs, public event payload or consumer ACK dependency.
CREATE TABLE moderation_lifecycle_current (
  subject_id uuid PRIMARY KEY,
  protocol_version smallint NOT NULL DEFAULT 1 CHECK(protocol_version=1),
  event_id uuid NOT NULL UNIQUE DEFAULT gen_random_uuid(),
  authority_generation uuid NOT NULL,
  lifecycle_revision bigint NOT NULL CHECK(lifecycle_revision>0),
  state text NOT NULL CHECK(state IN ('LIVE','DELETED')),
  deletion_cycle_id uuid,
  purge_at timestamptz,
  captured_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  CHECK((state='LIVE' AND deletion_cycle_id IS NULL AND purge_at IS NULL) OR
        (state='DELETED' AND deletion_cycle_id IS NOT NULL AND purge_at IS NOT NULL))
);
CREATE TABLE moderation_lifecycle_delivery (
  subject_id uuid NOT NULL REFERENCES moderation_lifecycle_current(subject_id) ON DELETE CASCADE,
  recipient text NOT NULL CHECK(recipient IN ('messaging','moderation')),
  event_id uuid NOT NULL,
  requested_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY(subject_id,recipient)
);
ALTER TABLE moderation_terminal_subjects ADD COLUMN event_id uuid NOT NULL DEFAULT gen_random_uuid();
CREATE UNIQUE INDEX moderation_terminal_event_id_idx ON moderation_terminal_subjects(event_id);
CREATE TABLE moderation_terminal_delivery (
  subject_id uuid NOT NULL REFERENCES moderation_terminal_subjects(subject_id),
  recipient text NOT NULL CHECK(recipient IN ('messaging','moderation')),
  event_id uuid NOT NULL,
  authority_generation uuid NOT NULL,
  requested_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY(subject_id,recipient)
);

CREATE FUNCTION auth_moderation_capture_lifecycle() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE generation uuid; captured_event uuid;
BEGIN
  IF TG_OP='DELETE' THEN
    DELETE FROM moderation_lifecycle_current WHERE subject_id=OLD.moderation_subject_id;
    RETURN OLD;
  END IF;
  IF TG_OP='UPDATE' AND OLD.deleted_at IS NOT NULL AND NEW.deleted_at IS NOT NULL AND
     OLD.deleted_at IS DISTINCT FROM NEW.deleted_at THEN
    -- A repeated deletion cannot restart/extend its existing canonical cycle.
    RAISE EXCEPTION 'active moderation deletion cycle is immutable';
  END IF;
  IF NEW.moderation_subject_id IS NULL THEN
    IF TG_OP='UPDATE' THEN
      DELETE FROM moderation_lifecycle_current WHERE subject_id=OLD.moderation_subject_id;
    END IF;
    RETURN NEW;
  END IF;
  IF EXISTS(SELECT 1 FROM moderation_terminal_subjects WHERE subject_id=NEW.moderation_subject_id) THEN
    DELETE FROM moderation_lifecycle_current WHERE subject_id=NEW.moderation_subject_id;
    RETURN NEW;
  END IF;
  IF TG_OP='UPDATE' AND NEW.moderation_lifecycle_revision=OLD.moderation_lifecycle_revision AND
     NEW.moderation_subject_id IS NOT DISTINCT FROM OLD.moderation_subject_id THEN
    RETURN NEW;
  END IF;
  SELECT g.generation INTO STRICT generation FROM moderation_authority_generation g WHERE singleton;
  captured_event := gen_random_uuid();
  INSERT INTO moderation_lifecycle_current(subject_id,event_id,authority_generation,
      lifecycle_revision,state,deletion_cycle_id,purge_at)
  VALUES(NEW.moderation_subject_id,captured_event,generation,NEW.moderation_lifecycle_revision,
      CASE WHEN NEW.deleted_at IS NULL THEN 'LIVE' ELSE 'DELETED' END,
      NEW.moderation_deletion_cycle_id,NEW.moderation_purge_at)
  ON CONFLICT(subject_id) DO UPDATE SET event_id=EXCLUDED.event_id,
      authority_generation=EXCLUDED.authority_generation,lifecycle_revision=EXCLUDED.lifecycle_revision,
      state=EXCLUDED.state,deletion_cycle_id=EXCLUDED.deletion_cycle_id,
      purge_at=EXCLUDED.purge_at,captured_at=clock_timestamp()
  WHERE moderation_lifecycle_current.lifecycle_revision<EXCLUDED.lifecycle_revision;
  IF NOT FOUND THEN RAISE EXCEPTION 'moderation lifecycle projection diverged'; END IF;
  INSERT INTO moderation_lifecycle_delivery(subject_id,recipient,event_id)
  VALUES(NEW.moderation_subject_id,'messaging',captured_event),(NEW.moderation_subject_id,'moderation',captured_event)
  ON CONFLICT(subject_id,recipient) DO UPDATE SET event_id=EXCLUDED.event_id,requested_at=clock_timestamp();
  RETURN NEW;
END $$;
CREATE TRIGGER auth_moderation_capture_lifecycle AFTER INSERT OR UPDATE OR DELETE ON accounts
FOR EACH ROW EXECUTE FUNCTION auth_moderation_capture_lifecycle();

CREATE FUNCTION auth_moderation_capture_terminal() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE generation uuid;
BEGIN
  SELECT g.generation INTO STRICT generation FROM moderation_authority_generation g WHERE singleton;
  DELETE FROM moderation_lifecycle_current WHERE subject_id=NEW.subject_id;
  INSERT INTO moderation_terminal_delivery(subject_id,recipient,event_id,authority_generation)
  VALUES(NEW.subject_id,'messaging',NEW.event_id,generation),(NEW.subject_id,'moderation',NEW.event_id,generation);
  RETURN NEW;
END $$;
CREATE TRIGGER auth_moderation_capture_terminal AFTER INSERT ON moderation_terminal_subjects
FOR EACH ROW EXECUTE FUNCTION auth_moderation_capture_terminal();

-- Existing owner state is a finite migration seed, never invented history or a
-- substitute for the independently current restore cut. Terminal always wins.
INSERT INTO moderation_lifecycle_current(subject_id,authority_generation,lifecycle_revision,state,deletion_cycle_id,purge_at)
SELECT a.moderation_subject_id,g.generation,a.moderation_lifecycle_revision,
       CASE WHEN a.deleted_at IS NULL THEN 'LIVE' ELSE 'DELETED' END,
       a.moderation_deletion_cycle_id,a.moderation_purge_at
FROM accounts a CROSS JOIN moderation_authority_generation g
WHERE g.singleton AND a.moderation_subject_id IS NOT NULL AND
      NOT EXISTS(SELECT 1 FROM moderation_terminal_subjects t WHERE t.subject_id=a.moderation_subject_id);
INSERT INTO moderation_lifecycle_delivery(subject_id,recipient,event_id)
SELECT c.subject_id,r.recipient,c.event_id FROM moderation_lifecycle_current c
CROSS JOIN (VALUES('messaging'),('moderation')) r(recipient);
INSERT INTO moderation_terminal_delivery(subject_id,recipient,event_id,authority_generation)
SELECT t.subject_id,r.recipient,t.event_id,g.generation FROM moderation_terminal_subjects t
CROSS JOIN moderation_authority_generation g CROSS JOIN (VALUES('messaging'),('moderation')) r(recipient)
WHERE g.singleton;
