BEGIN;

-- A deployment retains this creation identity independently. Reading it back
-- from an unknown/restored DB is not a replacement source qualification.
CREATE TABLE bot_space_membership_source (
 singleton boolean PRIMARY KEY CHECK(singleton),
 source_generation uuid NOT NULL CHECK(source_generation <> '00000000-0000-0000-0000-000000000000'),
 issued_pairs bigint NOT NULL DEFAULT 0 CHECK(issued_pairs >= 0),
 issued_operations bigint NOT NULL DEFAULT 0 CHECK(issued_operations >= 0),
 issued_calls bigint NOT NULL DEFAULT 0 CHECK(issued_calls >= 0)
);
INSERT INTO bot_space_membership_source(singleton,source_generation) VALUES(true,gen_random_uuid());

-- These records deliberately have no cascading FK to bots or visible installs.
CREATE TABLE bot_space_membership_operations (
 operation_id uuid PRIMARY KEY,
 kind text NOT NULL CHECK(kind IN ('ADD','CANCEL')),
 installation_id uuid NOT NULL,
 space_id uuid NOT NULL,
 bot_id uuid NOT NULL,
 actor_profile_id uuid NOT NULL,
 generation bigint NOT NULL CHECK(generation > 0),
 original_add_operation_id uuid NOT NULL,
 request_bytes bytea NOT NULL CHECK(octet_length(request_bytes)>0),
 request_hash text NOT NULL CHECK(request_hash ~ '^sha256:[0-9a-f]{64}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CHECK((kind='ADD' AND operation_id=original_add_operation_id) OR
       (kind='CANCEL' AND operation_id<>original_add_operation_id)),
 UNIQUE(space_id,bot_id,generation,kind)
);

CREATE TABLE bot_space_membership_heads (
 space_id uuid NOT NULL,
 bot_id uuid NOT NULL,
 installation_id uuid,
 actor_profile_id uuid,
 generation bigint CHECK(generation>0),
 add_operation_id uuid,
 cancel_operation_id uuid,
 desired_state text NOT NULL CHECK(desired_state IN ('INSTALL','CANCEL','TERMINAL')),
 phase text NOT NULL CHECK(phase IN ('PREPARED','ADD_ACK','ACTIVE','CANCEL_PENDING','REMOVE_ACK','CLEANUP_PENDING','CANCELED','LEGACY_UNKNOWN','PURGED')),
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 completion_revision bigint NOT NULL DEFAULT 0 CHECK(completion_revision>=0),
 lease_until timestamptz,
 cleanup_settled boolean NOT NULL DEFAULT false,
 PRIMARY KEY(space_id,bot_id),
 CHECK(phase IN ('LEGACY_UNKNOWN','PURGED') OR (installation_id IS NOT NULL AND actor_profile_id IS NOT NULL AND generation IS NOT NULL AND add_operation_id IS NOT NULL)),
 CHECK(NOT cleanup_settled OR phase IN ('CANCELED','PURGED'))
);

-- Completion inputs are immutable snapshots, separate from mutable worker CAS.
-- An active whitelist update retains the same membership identity/generation.
CREATE TABLE bot_space_membership_completions (
 add_operation_id uuid NOT NULL REFERENCES bot_space_membership_operations(operation_id),
 completion_revision bigint NOT NULL CHECK(completion_revision>0),
 administrator_account_id uuid NOT NULL,
 administrator_profile_id uuid NOT NULL,
 initiating_request_id text NOT NULL CHECK(length(initiating_request_id)>0),
 initiating_request_hash text NOT NULL CHECK(initiating_request_hash ~ '^sha256:[0-9a-f]{64}$'),
 inputs bytea NOT NULL CHECK(octet_length(inputs)>0),
 inputs_hash text NOT NULL CHECK(inputs_hash ~ '^sha256:[0-9a-f]{64}$'),
 completed boolean NOT NULL DEFAULT false,
 PRIMARY KEY(add_operation_id,completion_revision)
);

-- Only the verified initiating credential supplies this id/hash binding.
-- It is not an authority or a dedupe key derived from unsigned tracing headers.
CREATE TABLE bot_space_membership_calls (
 administrator_account_id uuid NOT NULL,
 request_id text NOT NULL CHECK(length(request_id)>0),
 administrator_profile_id uuid NOT NULL,
 method text NOT NULL CHECK(method IN ('INSTALL','CANCEL')),
 request_hash text NOT NULL CHECK(request_hash ~ '^sha256:[0-9a-f]{64}$'),
 add_operation_id uuid NOT NULL REFERENCES bot_space_membership_operations(operation_id),
 completion_revision bigint NOT NULL CHECK(completion_revision>=0),
 state text NOT NULL CHECK(state IN ('PENDING','SUCCEEDED')),
 result_installation_id uuid,
 PRIMARY KEY(administrator_account_id,request_id)
);

CREATE TABLE bot_space_membership_cleanup_chats (
 add_operation_id uuid NOT NULL REFERENCES bot_space_membership_operations(operation_id),
 chat_id uuid NOT NULL,
 PRIMARY KEY(add_operation_id,chat_id)
);

CREATE TABLE bot_space_membership_effects (
 add_operation_id uuid NOT NULL REFERENCES bot_space_membership_operations(operation_id),
 step_key text NOT NULL CHECK(length(step_key)>0 AND length(step_key)<=256),
 operation_id uuid NOT NULL UNIQUE,
 method text NOT NULL,
 request_bytes bytea NOT NULL,
 request_hash text NOT NULL CHECK(request_hash ~ '^sha256:[0-9a-f]{64}$'),
 acknowledged boolean NOT NULL DEFAULT false,
 response_bytes bytea,
 PRIMARY KEY(add_operation_id,step_key)
);

CREATE TABLE bot_space_membership_chat_sources (
 add_operation_id uuid NOT NULL REFERENCES bot_space_membership_operations(operation_id),
 completion_revision bigint NOT NULL,
 chat_id uuid NOT NULL,
 attachment_id uuid NOT NULL UNIQUE,
 request_bytes bytea NOT NULL,
 response_bytes bytea,
 PRIMARY KEY(add_operation_id,completion_revision,chat_id)
);
CREATE TABLE bot_space_membership_cleanup_decisions (
 add_operation_id uuid PRIMARY KEY REFERENCES bot_space_membership_operations(operation_id),
 decision_id uuid NOT NULL UNIQUE,
 decision_bytes bytea NOT NULL,
 scope_bytes bytea NOT NULL
);

-- Do not invent an original operation or generation for pre-journal installs.
INSERT INTO bot_space_membership_heads(space_id,bot_id,installation_id,actor_profile_id,desired_state,phase)
 SELECT i.space_id,i.bot_id,i.id,b.actor_profile_id,'TERMINAL','LEGACY_UNKNOWN'
 FROM bot_space_installations i JOIN bots b ON b.id=i.bot_id;
UPDATE bot_space_membership_source SET issued_pairs=(SELECT count(*) FROM bot_space_membership_heads) WHERE singleton;

CREATE FUNCTION bot_membership_evidence_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'Bot membership evidence cannot be removed or rewritten' USING ERRCODE='55000';
END;
$$;
CREATE TRIGGER bot_membership_operations_immutable BEFORE UPDATE OR DELETE ON bot_space_membership_operations
 FOR EACH ROW EXECUTE FUNCTION bot_membership_evidence_guard();
CREATE TRIGGER bot_membership_operations_no_truncate BEFORE TRUNCATE ON bot_space_membership_operations
 FOR EACH STATEMENT EXECUTE FUNCTION bot_membership_evidence_guard();
CREATE TRIGGER bot_membership_heads_no_delete BEFORE DELETE ON bot_space_membership_heads
 FOR EACH ROW EXECUTE FUNCTION bot_membership_evidence_guard();
CREATE TRIGGER bot_membership_heads_no_truncate BEFORE TRUNCATE ON bot_space_membership_heads
 FOR EACH STATEMENT EXECUTE FUNCTION bot_membership_evidence_guard();
CREATE TRIGGER bot_membership_source_no_delete BEFORE DELETE ON bot_space_membership_source
 FOR EACH ROW EXECUTE FUNCTION bot_membership_evidence_guard();
CREATE TRIGGER bot_membership_source_no_truncate BEFORE TRUNCATE ON bot_space_membership_source
 FOR EACH STATEMENT EXECUTE FUNCTION bot_membership_evidence_guard();

CREATE FUNCTION bot_membership_head_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.space_id<>OLD.space_id OR NEW.bot_id<>OLD.bot_id OR NEW.revision<=OLD.revision THEN
  RAISE EXCEPTION 'Bot membership head identity/revision conflict' USING ERRCODE='55000';
 END IF;
 IF OLD.phase='PURGED' OR (OLD.phase='LEGACY_UNKNOWN' AND NEW.phase<>'PURGED') THEN
  RAISE EXCEPTION 'Bot unknown/terminal source cannot be adopted' USING ERRCODE='55000';
 END IF;
 IF NEW.generation IS DISTINCT FROM OLD.generation THEN
  IF OLD.phase<>'CANCELED' OR NOT OLD.cleanup_settled OR OLD.generation IS NULL
    OR NEW.generation::numeric<>OLD.generation::numeric+1 OR NEW.phase<>'PREPARED'
    OR NEW.installation_id=OLD.installation_id OR NEW.add_operation_id=OLD.add_operation_id THEN
   RAISE EXCEPTION 'Bot generation requires settled exact predecessor' USING ERRCODE='55000';
  END IF;
 ELSIF NEW.installation_id IS DISTINCT FROM OLD.installation_id OR NEW.actor_profile_id IS DISTINCT FROM OLD.actor_profile_id
   OR NEW.add_operation_id IS DISTINCT FROM OLD.add_operation_id
   OR (OLD.cancel_operation_id IS NOT NULL AND NEW.cancel_operation_id IS DISTINCT FROM OLD.cancel_operation_id)
   OR NEW.completion_revision<OLD.completion_revision
   OR (OLD.desired_state='CANCEL' AND NEW.desired_state='INSTALL') THEN
  RAISE EXCEPTION 'Bot attempt cannot be rebound or rewound' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END;
$$;
CREATE TRIGGER bot_membership_head_changes BEFORE UPDATE ON bot_space_membership_heads
 FOR EACH ROW EXECUTE FUNCTION bot_membership_head_guard();

CREATE FUNCTION bot_membership_source_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.singleton IS DISTINCT FROM OLD.singleton OR NEW.source_generation IS DISTINCT FROM OLD.source_generation
   OR NEW.issued_pairs<OLD.issued_pairs OR NEW.issued_operations<OLD.issued_operations OR NEW.issued_calls<OLD.issued_calls THEN
  RAISE EXCEPTION 'Bot source identity/counters cannot rewind' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END;
$$;
CREATE TRIGGER bot_membership_source_changes BEFORE UPDATE ON bot_space_membership_source
 FOR EACH ROW EXECUTE FUNCTION bot_membership_source_guard();

CREATE FUNCTION bot_membership_snapshot_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP IN ('DELETE','TRUNCATE') THEN
  RAISE EXCEPTION 'Bot unresolved snapshot cannot be discarded' USING ERRCODE='55000';
 END IF;
 IF TG_TABLE_NAME='bot_space_membership_completions' THEN
  IF (to_jsonb(NEW)-'completed') IS DISTINCT FROM (to_jsonb(OLD)-'completed') OR (OLD.completed AND NOT NEW.completed) THEN
   RAISE EXCEPTION 'Bot completion input is immutable' USING ERRCODE='55000';
  END IF;
 ELSIF TG_TABLE_NAME='bot_space_membership_calls' THEN
  IF (to_jsonb(NEW)-'state'-'result_installation_id') IS DISTINCT FROM (to_jsonb(OLD)-'state'-'result_installation_id') OR (OLD.state='SUCCEEDED' AND NEW IS DISTINCT FROM OLD) OR (OLD.result_installation_id IS NOT NULL AND NEW.result_installation_id IS DISTINCT FROM OLD.result_installation_id) THEN
   RAISE EXCEPTION 'Bot request binding is immutable' USING ERRCODE='55000';
  END IF;
 ELSIF TG_TABLE_NAME='bot_space_membership_chat_sources' THEN
  IF (to_jsonb(NEW)-'response_bytes') IS DISTINCT FROM (to_jsonb(OLD)-'response_bytes') OR (OLD.response_bytes IS NOT NULL AND NEW.response_bytes IS DISTINCT FROM OLD.response_bytes) THEN
   RAISE EXCEPTION 'Bot Chat observation is immutable' USING ERRCODE='55000';
  END IF;
 ELSE
  IF (to_jsonb(NEW)-'acknowledged'-'response_bytes') IS DISTINCT FROM (to_jsonb(OLD)-'acknowledged'-'response_bytes') OR (OLD.acknowledged AND NOT NEW.acknowledged) OR (OLD.response_bytes IS NOT NULL AND NEW.response_bytes IS DISTINCT FROM OLD.response_bytes) THEN
   RAISE EXCEPTION 'Bot effect input is immutable' USING ERRCODE='55000';
  END IF;
 END IF;
 RETURN NEW;
END;
$$;
CREATE TRIGGER bot_membership_completion_changes BEFORE UPDATE OR DELETE ON bot_space_membership_completions
 FOR EACH ROW EXECUTE FUNCTION bot_membership_snapshot_guard();
CREATE TRIGGER bot_membership_completion_no_truncate BEFORE TRUNCATE ON bot_space_membership_completions
 FOR EACH STATEMENT EXECUTE FUNCTION bot_membership_snapshot_guard();
CREATE TRIGGER bot_membership_effect_changes BEFORE UPDATE OR DELETE ON bot_space_membership_effects
 FOR EACH ROW EXECUTE FUNCTION bot_membership_snapshot_guard();
CREATE TRIGGER bot_membership_effect_no_truncate BEFORE TRUNCATE ON bot_space_membership_effects
 FOR EACH STATEMENT EXECUTE FUNCTION bot_membership_snapshot_guard();
CREATE TRIGGER bot_membership_chat_source_changes BEFORE UPDATE OR DELETE ON bot_space_membership_chat_sources
 FOR EACH ROW EXECUTE FUNCTION bot_membership_snapshot_guard();
CREATE TRIGGER bot_membership_chat_source_no_truncate BEFORE TRUNCATE ON bot_space_membership_chat_sources
 FOR EACH STATEMENT EXECUTE FUNCTION bot_membership_snapshot_guard();
CREATE TRIGGER bot_membership_decision_immutable BEFORE UPDATE OR DELETE ON bot_space_membership_cleanup_decisions
 FOR EACH ROW EXECUTE FUNCTION bot_membership_evidence_guard();
CREATE TRIGGER bot_membership_decision_no_truncate BEFORE TRUNCATE ON bot_space_membership_cleanup_decisions
 FOR EACH STATEMENT EXECUTE FUNCTION bot_membership_evidence_guard();
CREATE TRIGGER bot_membership_call_changes BEFORE UPDATE OR DELETE ON bot_space_membership_calls
 FOR EACH ROW EXECUTE FUNCTION bot_membership_snapshot_guard();
CREATE TRIGGER bot_membership_call_no_truncate BEFORE TRUNCATE ON bot_space_membership_calls
 FOR EACH STATEMENT EXECUTE FUNCTION bot_membership_snapshot_guard();
CREATE TRIGGER bot_membership_cleanup_chat_immutable BEFORE UPDATE OR DELETE ON bot_space_membership_cleanup_chats
 FOR EACH ROW EXECUTE FUNCTION bot_membership_evidence_guard();
CREATE TRIGGER bot_membership_cleanup_chat_no_truncate BEFORE TRUNCATE ON bot_space_membership_cleanup_chats
 FOR EACH STATEMENT EXECUTE FUNCTION bot_membership_evidence_guard();
COMMIT;
