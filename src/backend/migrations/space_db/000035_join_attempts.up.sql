BEGIN;
CREATE TABLE space_join_attempts (
 id uuid PRIMARY KEY,
 space_id uuid NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
 account_id uuid NOT NULL,
 profile_id uuid NOT NULL,
 invite_id uuid,
 guest boolean NOT NULL,
 binding_sha256 bytea NOT NULL CHECK(octet_length(binding_sha256)=32),
 policy_version bigint NOT NULL CHECK(policy_version>0),
 policy_sha256 bytea NOT NULL CHECK(octet_length(policy_sha256)=32),
 policy_snapshot jsonb NOT NULL,
 state text NOT NULL CHECK(state IN ('COLLECTING','PENDING','READY','CLAIMED','PUBLISHED','FAILED','EXPIRED','INVALIDATED')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 collection_deadline timestamptz NOT NULL,
 submission_sha256 bytea CHECK(octet_length(submission_sha256)=32),
 submitted_answers jsonb,
 evidence_sha256 bytea CHECK(octet_length(evidence_sha256)=32),
 approved_account_id uuid,
 approved_profile_id uuid,
 approval_operation_id uuid,
 publication_operation_id uuid UNIQUE,
 CHECK(collection_deadline>created_at),
 CHECK((submission_sha256 IS NULL)=(submitted_answers IS NULL)),
 CHECK((approved_account_id IS NULL)=(approved_profile_id IS NULL)),
 CHECK((approved_profile_id IS NULL)=(approval_operation_id IS NULL)),
 CHECK(state NOT IN ('READY','CLAIMED','PUBLISHED') OR (evidence_sha256 IS NOT NULL AND submission_sha256 IS NOT NULL)),
 CHECK(state NOT IN ('CLAIMED','PUBLISHED') OR publication_operation_id IS NOT NULL)
);
CREATE UNIQUE INDEX space_join_attempt_active_binding ON space_join_attempts(binding_sha256)
WHERE state IN ('COLLECTING','PENDING','READY');
CREATE INDEX space_join_attempt_review ON space_join_attempts(space_id,created_at,id) WHERE state='PENDING';
CREATE INDEX space_join_attempt_account_pending ON space_join_attempts(space_id,account_id)
WHERE state IN ('COLLECTING','PENDING','READY','CLAIMED');
CREATE TABLE space_join_approval_operations (
 actor_profile_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 attempt_id uuid NOT NULL REFERENCES space_join_attempts(id) ON DELETE CASCADE,
 request_sha256 bytea NOT NULL CHECK(octet_length(request_sha256)=32),
 state text NOT NULL CHECK(state IN ('RESERVED','COMPLETED')),
 evidence_sha256 bytea CHECK(octet_length(evidence_sha256)=32),
 outcome jsonb,
 CHECK((state='COMPLETED')=(evidence_sha256 IS NOT NULL AND outcome IS NOT NULL)),
 CHECK((evidence_sha256 IS NULL)=(outcome IS NULL)),
 PRIMARY KEY(actor_profile_id,operation_id)
);
-- Reuse the current canonical P3 predicate, never a frozen participant count.
CREATE FUNCTION purge_space_join_attempts_terminal() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF current_setting('voice.audit_delete_mode',true) IS DISTINCT FROM 'space_purge'
    OR NOT space_terminal_purge_ready(OLD.id) THEN
  RAISE EXCEPTION 'join attempt erasure requires terminal Space P3' USING ERRCODE='55000';
 END IF;
 DELETE FROM space_join_attempts WHERE space_id=OLD.id;
 RETURN OLD;
END $$;
CREATE TRIGGER space_join_attempts_terminal AFTER DELETE ON spaces
FOR EACH ROW EXECUTE FUNCTION purge_space_join_attempts_terminal();
COMMIT;
