-- BE-130: immutable global audit IDs survive ordinary payload retention.
BEGIN;
LOCK TABLE audit_log IN ACCESS EXCLUSIVE MODE;

-- Preserve the exact previously shipped local registry, including additive
-- local writers. Do not reinterpret historical JSONB as external wire payload.
DO $$
DECLARE local_expression TEXT;
BEGIN
    SELECT pg_get_expr(conbin, conrelid) INTO STRICT local_expression
    FROM pg_constraint
    WHERE conrelid = 'audit_log'::regclass AND conname = 'audit_log_closed_registry';
    EXECUTE format('CREATE FUNCTION space_audit_local_registry_v1(action text, target_type text, details jsonb)
        RETURNS boolean LANGUAGE sql IMMUTABLE AS %L', 'SELECT ' || local_expression);
END
$$;

CREATE FUNCTION space_audit_uint64_v1(value jsonb, positive boolean)
RETURNS boolean LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE number numeric;
BEGIN
    IF value IS NULL OR jsonb_typeof(value) <> 'number' THEN RETURN false; END IF;
    number := (value #>> '{}')::numeric;
    RETURN number = trunc(number) AND number >= CASE WHEN positive THEN 1 ELSE 0 END
        AND number <= 18446744073709551615;
END
$$;

CREATE FUNCTION space_audit_external_registry_v1(
    origin text, action text, target_type text, target_id uuid, space_id uuid, details jsonb)
RETURNS boolean LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE expected_target text; item jsonb; field text; previous_field text;
BEGIN
    IF details IS NULL OR jsonb_typeof(details) <> 'object' THEN RETURN false; END IF;
    IF origin = 'role' THEN
        expected_target := CASE
            WHEN action IN ('role_created','role_updated','role_deleted','default_join_role_set') THEN 'role'
            WHEN action = 'roles_reordered' THEN 'space'
            WHEN action IN ('role_assigned','role_revoked') THEN 'profile'
            WHEN action IN ('chat_override_set','chat_override_removed') THEN 'chat'
            WHEN action IN ('voice_room_override_set','voice_room_override_removed') THEN 'voice_room'
            ELSE NULL END;
    ELSIF origin = 'chat' AND action IN ('chat_created','chat_updated','chat_deleted') THEN
        expected_target := 'chat';
    END IF;
    IF expected_target IS NULL OR target_type IS DISTINCT FROM expected_target THEN RETURN false; END IF;
    IF action = 'roles_reordered' AND target_id IS DISTINCT FROM space_id THEN RETURN false; END IF;
    IF action IN ('role_created','role_deleted','default_join_role_set','chat_created','chat_deleted') THEN
        RETURN details = '{}'::jsonb;
    ELSIF action IN ('role_updated','chat_updated') THEN
        IF details - 'changed_fields' <> '{}'::jsonb OR NOT details ? 'changed_fields'
           OR jsonb_typeof(details -> 'changed_fields') <> 'array' THEN RETURN false; END IF;
        IF jsonb_array_length(details -> 'changed_fields') = 0 THEN RETURN false; END IF;
        FOR item IN SELECT value FROM jsonb_array_elements(details -> 'changed_fields') LOOP
            IF jsonb_typeof(item) <> 'string' THEN RETURN false; END IF;
            field := item #>> '{}';
            IF previous_field IS NOT NULL AND previous_field COLLATE "C" >= field COLLATE "C" THEN RETURN false; END IF;
            IF action = 'role_updated' AND field NOT IN ('name','permissions','position') THEN RETURN false; END IF;
            IF action = 'chat_updated' AND field NOT IN ('allow_guests','allow_user_main_feed','avatar_url','name','slow_mode_seconds','threads_enabled','topic') THEN RETURN false; END IF;
            previous_field := field;
        END LOOP;
        RETURN true;
    ELSIF action IN ('role_assigned','role_revoked') THEN
        RETURN details - 'role_id' = '{}'::jsonb AND details ? 'role_id'
            AND jsonb_typeof(details -> 'role_id') = 'string'
            AND details ->> 'role_id' ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';
    ELSIF action = 'roles_reordered' THEN
        RETURN details - 'count' = '{}'::jsonb AND space_audit_uint64_v1(details -> 'count',true);
    ELSE
        RETURN details - ARRAY['allow_mask','deny_mask','role_id'] = '{}'::jsonb
            AND details ?& ARRAY['allow_mask','deny_mask','role_id']
            AND jsonb_typeof(details -> 'role_id') = 'string'
            AND details ->> 'role_id' ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
            AND space_audit_uint64_v1(details -> 'allow_mask',false)
            AND space_audit_uint64_v1(details -> 'deny_mask',false);
    END IF;
END
$$;

ALTER TABLE audit_log
    ADD COLUMN ingress_origin text,
    ADD COLUMN ingress_fingerprint_version integer,
    ADD COLUMN ingress_fingerprint bytea,
    DROP CONSTRAINT audit_log_closed_registry,
    ADD CONSTRAINT audit_log_closed_registry CHECK (COALESCE(
        (ingress_origin IS NULL AND ingress_fingerprint_version IS NULL AND ingress_fingerprint IS NULL
            AND space_audit_local_registry_v1(action,target_type,details))
        OR (ingress_origin IN ('role','chat') AND ingress_fingerprint_version = 1
            AND octet_length(ingress_fingerprint) = 32
            AND space_audit_external_registry_v1(ingress_origin,action,target_type,target_id,space_id,details)), false));

CREATE TABLE audit_event_receipts (
    audit_event_id uuid PRIMARY KEY,
    space_id uuid NOT NULL,
    origin text NOT NULL CHECK (origin IN ('local','role','chat')),
    fingerprint_version integer,
    fingerprint bytea,
    CHECK (COALESCE(
        (origin = 'local' AND fingerprint_version IS NULL AND fingerprint IS NULL)
        OR (origin IN ('role','chat') AND fingerprint_version = 1 AND octet_length(fingerprint) = 32),false))
);
CREATE INDEX audit_event_receipts_space_idx ON audit_event_receipts(space_id);

INSERT INTO audit_event_receipts(audit_event_id,space_id,origin)
SELECT id,space_id,'local' FROM audit_log;

CREATE FUNCTION reserve_space_audit_id_v1()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO audit_event_receipts(audit_event_id,space_id,origin,fingerprint_version,fingerprint)
    VALUES(NEW.id,NEW.space_id,COALESCE(NEW.ingress_origin,'local'),NEW.ingress_fingerprint_version,NEW.ingress_fingerprint);
    RETURN NEW;
END
$$;
CREATE TRIGGER audit_log_reserve_global_id BEFORE INSERT ON audit_log
FOR EACH ROW EXECUTE FUNCTION reserve_space_audit_id_v1();

CREATE FUNCTION guard_space_audit_receipt_v1()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit ID receipts are immutable until authorized terminal minimization' USING ERRCODE = '55000';
END
$$;
CREATE TRIGGER audit_event_receipts_immutable BEFORE UPDATE OR DELETE ON audit_event_receipts
FOR EACH ROW EXECUTE FUNCTION guard_space_audit_receipt_v1();
COMMIT;
