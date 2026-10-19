BEGIN;
-- Q is grouping metadata only. The actual invite-bound A and public operation
-- ledgers are installed later by35/36; no forward FK or duplicate ledger.
CREATE TABLE space_join_request_groups (
 id uuid PRIMARY KEY,
 space_id uuid NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
 account_id uuid NOT NULL,
 profile_id uuid NOT NULL,
 policy_version bigint NOT NULL CHECK(policy_version>0),
 policy_sha256 bytea NOT NULL CHECK(octet_length(policy_sha256)=32),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(space_id,account_id,profile_id,policy_version,policy_sha256),
 UNIQUE(id,space_id)
);
CREATE TABLE space_join_request_attempts (
 attempt_id uuid PRIMARY KEY,
 request_id uuid NOT NULL,
 space_id uuid NOT NULL,
 FOREIGN KEY(request_id,space_id) REFERENCES space_join_request_groups(id,space_id) ON DELETE CASCADE,
 UNIQUE(request_id,attempt_id)
);
CREATE TABLE space_join_request_selections (
 actor_profile_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 actor_account_id uuid NOT NULL,
 space_id uuid NOT NULL,
 request_id uuid NOT NULL,
 selected_attempt_id uuid NOT NULL,
 explicit_attempt_id uuid,
 request_sha256 bytea NOT NULL CHECK(octet_length(request_sha256)=32),
 CHECK(explicit_attempt_id IS NULL OR explicit_attempt_id=selected_attempt_id),
 FOREIGN KEY(request_id,selected_attempt_id) REFERENCES space_join_request_attempts(request_id,attempt_id) ON DELETE CASCADE,
 PRIMARY KEY(actor_profile_id,operation_id)
);
-- Runtime references are intentionally checked only when a child/selection is
-- inserted: migration32 precedes the actual35/36 ledgers.
CREATE FUNCTION validate_space_join_request_attempt() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NOT EXISTS(
  SELECT 1 FROM space_join_request_groups g JOIN space_join_attempts a ON a.id=NEW.attempt_id
  WHERE g.id=NEW.request_id AND g.space_id=NEW.space_id
   AND a.space_id=g.space_id AND a.account_id=g.account_id AND a.profile_id=g.profile_id
   AND a.policy_version=g.policy_version AND a.policy_sha256=g.policy_sha256
   AND a.state IN ('PENDING','READY') AND a.policy_snapshot->'require_manual_approval'='true'::jsonb
 ) THEN RAISE EXCEPTION 'exact pending request cohort required' USING ERRCODE='55000'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER validate_space_join_request_attempt BEFORE INSERT ON space_join_request_attempts
 FOR EACH ROW EXECUTE FUNCTION validate_space_join_request_attempt();
CREATE FUNCTION validate_space_join_request_selection() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NOT EXISTS(
  SELECT 1 FROM space_join_operations o JOIN space_join_request_groups g ON g.id=NEW.request_id
  WHERE o.actor_profile_id=NEW.actor_profile_id AND o.operation_id=NEW.operation_id
   AND o.actor_account_id=NEW.actor_account_id AND o.space_id=NEW.space_id AND g.space_id=NEW.space_id
   AND o.request_method='/voice.space.v1.SpaceService/ApproveJoinRequest'
   AND o.request_sha256=NEW.request_sha256 AND o.selected_attempt_id=NEW.selected_attempt_id
 ) THEN RAISE EXCEPTION 'exact public request selection required' USING ERRCODE='55000'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER validate_space_join_request_selection BEFORE INSERT ON space_join_request_selections
 FOR EACH ROW EXECUTE FUNCTION validate_space_join_request_selection();
CREATE FUNCTION immutable_space_join_request_mapping() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' AND current_setting('voice.audit_delete_mode',true)='space_purge'
  AND space_terminal_purge_ready(OLD.space_id) THEN RETURN OLD; END IF;
 RAISE EXCEPTION 'join request mapping is immutable until terminal Space purge' USING ERRCODE='55000';
END $$;
CREATE TRIGGER immutable_space_join_request_group BEFORE UPDATE OR DELETE ON space_join_request_groups FOR EACH ROW EXECUTE FUNCTION immutable_space_join_request_mapping();
CREATE TRIGGER immutable_space_join_request_attempt BEFORE UPDATE OR DELETE ON space_join_request_attempts FOR EACH ROW EXECUTE FUNCTION immutable_space_join_request_mapping();
CREATE TRIGGER immutable_space_join_request_selection BEFORE UPDATE OR DELETE ON space_join_request_selections FOR EACH ROW EXECUTE FUNCTION immutable_space_join_request_mapping();
CREATE TRIGGER no_truncate_space_join_request_group BEFORE TRUNCATE ON space_join_request_groups FOR EACH STATEMENT EXECUTE FUNCTION immutable_space_join_request_mapping();
CREATE TRIGGER no_truncate_space_join_request_attempt BEFORE TRUNCATE ON space_join_request_attempts FOR EACH STATEMENT EXECUTE FUNCTION immutable_space_join_request_mapping();
CREATE TRIGGER no_truncate_space_join_request_selection BEFORE TRUNCATE ON space_join_request_selections FOR EACH STATEMENT EXECUTE FUNCTION immutable_space_join_request_mapping();
COMMIT;
