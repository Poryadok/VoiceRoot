BEGIN;
-- A reservation is technical replay/selection data, never admission authority.
CREATE TABLE space_join_operations (
 actor_profile_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 actor_account_id uuid NOT NULL,
 space_id uuid NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
 request_method text NOT NULL CHECK(request_method IN (
  '/voice.space.v1.SpaceService/StartJoinAttempt',
  '/voice.space.v1.SpaceService/SubmitJoinAttempt',
  '/voice.space.v1.SpaceService/ApproveJoinRequest')),
 request_sha256 bytea NOT NULL CHECK(octet_length(request_sha256)=32),
 selected_attempt_id uuid REFERENCES space_join_attempts(id) ON DELETE CASCADE,
 state text NOT NULL DEFAULT 'RESERVED' CHECK(state IN ('RESERVED','DECIDED','COMPLETED')),
 decision_basis jsonb,
 decision_basis_sha256 bytea UNIQUE CHECK(octet_length(decision_basis_sha256)=32),
 observed_basis_sha256 bytea CHECK(octet_length(observed_basis_sha256)=32),
 publication_operation_id uuid UNIQUE,
 publication_state text CHECK(publication_state IN ('PREPARING','PUBLISHED')),
 outcome_code integer,
 outcome_proto bytea,
 CHECK((decision_basis IS NULL)=(decision_basis_sha256 IS NULL)),
 CHECK(state<>'RESERVED' OR (decision_basis IS NULL AND observed_basis_sha256 IS NULL)),
 CHECK(state<>'DECIDED' OR decision_basis IS NOT NULL OR observed_basis_sha256 IS NOT NULL),
 CHECK(decision_basis IS NULL OR observed_basis_sha256 IS NULL),
 CHECK((publication_operation_id IS NULL)=(publication_state IS NULL)),
 CHECK(publication_operation_id IS NULL OR decision_basis IS NOT NULL),
 CHECK((state='COMPLETED')=(outcome_code IS NOT NULL AND outcome_proto IS NOT NULL)),
 CHECK((outcome_code IS NULL)=(outcome_proto IS NULL)),
 UNIQUE(space_id,decision_basis_sha256),
 FOREIGN KEY(space_id,observed_basis_sha256) REFERENCES space_join_operations(space_id,decision_basis_sha256) ON DELETE CASCADE,
 PRIMARY KEY(actor_profile_id,operation_id)
);
CREATE INDEX space_join_operation_space ON space_join_operations(space_id);
CREATE UNIQUE INDEX space_join_original_submit_basis ON space_join_operations(selected_attempt_id)
WHERE decision_basis->'context'->>'kind' IN ('1','2');
CREATE UNIQUE INDEX space_join_original_approval_basis ON space_join_operations(selected_attempt_id)
WHERE decision_basis->'context'->>'kind'='3';
CREATE FUNCTION purge_space_join_operations_terminal() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF current_setting('voice.audit_delete_mode',true) IS DISTINCT FROM 'space_purge'
    OR NOT space_terminal_purge_ready(OLD.id) THEN
  RAISE EXCEPTION 'join operation erasure requires terminal Space P3' USING ERRCODE='55000';
 END IF;
 DELETE FROM space_join_operations WHERE space_id=OLD.id;
 RETURN OLD;
END $$;
CREATE TRIGGER space_join_operations_terminal AFTER DELETE ON spaces
FOR EACH ROW EXECUTE FUNCTION purge_space_join_operations_terminal();
COMMIT;
