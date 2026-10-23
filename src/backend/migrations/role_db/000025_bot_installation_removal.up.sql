BEGIN;
CREATE TABLE role_bot_actor_bindings (
 bot_id UUID PRIMARY KEY, actor_profile_id UUID NOT NULL UNIQUE
);
CREATE TABLE role_bot_global_removals (
 bot_id UUID PRIMARY KEY REFERENCES role_bot_actor_bindings(bot_id),
 actor_profile_id UUID NOT NULL,
 decision_id UUID NOT NULL, decision_sha256 BYTEA NOT NULL CHECK(octet_length(decision_sha256)=32)
);
CREATE TABLE role_bot_installation_removals (
 bot_id UUID NOT NULL REFERENCES role_bot_actor_bindings(bot_id),
 space_id UUID NOT NULL, installation_id UUID NOT NULL UNIQUE,
 actor_profile_id UUID NOT NULL, evidence_kind TEXT NOT NULL CHECK(evidence_kind IN ('native','legacy_installation')),
 install_operation_id UUID, generation BIGINT,
 CHECK((evidence_kind='native')=(install_operation_id IS NOT NULL AND generation IS NOT NULL)),
 CHECK(generation IS NULL OR generation>0),
 PRIMARY KEY(bot_id,space_id,installation_id)
);
CREATE TABLE role_bot_removal_operations (
 operation_id UUID PRIMARY KEY,
 bot_id UUID NOT NULL REFERENCES role_bot_actor_bindings(bot_id),
 actor_profile_id UUID NOT NULL, space_id UUID NOT NULL,
 installation_id UUID,
 request_bytes BYTEA NOT NULL, request_sha256 BYTEA NOT NULL CHECK(octet_length(request_sha256)=32),
 response_bytes BYTEA,
 CHECK(octet_length(request_bytes)>0), CHECK(response_bytes IS NULL OR octet_length(response_bytes)>0)
);
CREATE INDEX role_bot_removal_scope ON role_bot_removal_operations(bot_id,space_id,installation_id);
CREATE FUNCTION role_bot_removal_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_TABLE_NAME='role_bot_removal_operations' AND TG_OP='UPDATE'
 AND OLD.response_bytes IS NULL AND NEW.response_bytes IS NOT NULL
 AND (to_jsonb(NEW)-'response_bytes')=(to_jsonb(OLD)-'response_bytes') THEN RETURN NEW; END IF;
 RAISE EXCEPTION 'Bot removal evidence is immutable' USING ERRCODE='55000';
END $$;
CREATE TRIGGER role_bot_actor_bindings_guard BEFORE UPDATE OR DELETE OR TRUNCATE ON role_bot_actor_bindings FOR EACH STATEMENT EXECUTE FUNCTION role_bot_removal_immutable();
CREATE TRIGGER role_bot_global_removals_guard BEFORE UPDATE OR DELETE OR TRUNCATE ON role_bot_global_removals FOR EACH STATEMENT EXECUTE FUNCTION role_bot_removal_immutable();
CREATE TRIGGER role_bot_installation_removals_guard BEFORE UPDATE OR DELETE OR TRUNCATE ON role_bot_installation_removals FOR EACH STATEMENT EXECUTE FUNCTION role_bot_removal_immutable();
CREATE TRIGGER role_bot_removal_operations_change BEFORE UPDATE OR DELETE ON role_bot_removal_operations FOR EACH ROW EXECUTE FUNCTION role_bot_removal_immutable();
CREATE TRIGGER role_bot_removal_operations_truncate BEFORE TRUNCATE ON role_bot_removal_operations FOR EACH STATEMENT EXECUTE FUNCTION role_bot_removal_immutable();
COMMIT;
