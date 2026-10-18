-- Frozen bits 43/44 are independent of settings, invite and send authority.
-- Seed deltas apply only to built-in roles; custom same-name roles and all
-- explicit overrides retain their original allow/deny masks.
UPDATE roles
SET permissions = permissions | (1::bigint << 43) | (1::bigint << 44),
    updated_at = clock_timestamp()
WHERE is_system AND name IN ('Owner', 'Admin');

UPDATE roles
SET permissions = permissions | (1::bigint << 44),
    updated_at = clock_timestamp()
WHERE is_system AND name IN ('Member', 'Moderator');

-- A scope incarnation and eligibility binding are both part of identity.
-- These rows assert no current membership; every decision requalifies them.
CREATE TABLE role_space_chat_grants (
    space_id UUID NOT NULL,
    chat_id UUID NOT NULL,
    chat_scope_incarnation UUID NOT NULL,
    target_profile_id UUID NOT NULL,
    membership_binding JSONB NOT NULL CHECK (jsonb_typeof(membership_binding) = 'object'),
    allowed BOOLEAN NOT NULL,
    operation_id UUID NOT NULL,
    actor_profile_id UUID NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (space_id, chat_id, chat_scope_incarnation, target_profile_id, membership_binding)
);

CREATE TABLE role_space_chat_policies (
    space_id UUID NOT NULL,
    chat_id UUID NOT NULL,
    chat_scope_incarnation UUID NOT NULL,
    member_additions_enabled BOOLEAN NOT NULL,
    operation_id UUID NOT NULL,
    actor_profile_id UUID NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (space_id, chat_id, chat_scope_incarnation)
);

-- Append-only operation records are also the local audit trail. Replaying an
-- old allow returns its historical result without reapplying the grant.
CREATE TABLE role_space_chat_operations (
    operation_id UUID PRIMARY KEY,
    space_id UUID NOT NULL,
    request_bytes BYTEA NOT NULL CHECK (octet_length(request_bytes) > 0),
    resolved_binding JSONB,
    allowed BOOLEAN NOT NULL,
    role_revision BIGINT NOT NULL CHECK (role_revision > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE role_space_chat_outbox (
    operation_id UUID PRIMARY KEY REFERENCES role_space_chat_operations(operation_id),
    space_id UUID NOT NULL,
    role_revision BIGINT NOT NULL CHECK (role_revision > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    delivered_at TIMESTAMPTZ
);

CREATE FUNCTION role_space_chat_operation_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'space chat operation receipt is immutable' USING ERRCODE='55000';
END;
$$;
CREATE TRIGGER role_space_chat_operations_immutable
BEFORE UPDATE OR DELETE ON role_space_chat_operations
FOR EACH ROW EXECUTE FUNCTION role_space_chat_operation_immutable();
CREATE TRIGGER role_space_chat_operations_no_truncate
BEFORE TRUNCATE ON role_space_chat_operations
FOR EACH STATEMENT EXECUTE FUNCTION role_space_chat_operation_immutable();

-- Migration15 established complete coverage for existing Role inputs. Extend
-- that SAME monotonic clock, retaining its permanent floor and Space fence.
CREATE TRIGGER role_authority_revision_space_chat_grants
AFTER INSERT OR UPDATE OR DELETE ON role_space_chat_grants
FOR EACH ROW EXECUTE FUNCTION role_authority_revision_changed('space_id');
CREATE TRIGGER role_authority_revision_space_chat_grants_no_truncate
BEFORE TRUNCATE ON role_space_chat_grants
FOR EACH STATEMENT EXECUTE FUNCTION role_authority_revision_floor_guard();
CREATE TRIGGER role_authority_revision_space_chat_policies
AFTER INSERT OR UPDATE OR DELETE ON role_space_chat_policies
FOR EACH ROW EXECUTE FUNCTION role_authority_revision_changed('space_id');
CREATE TRIGGER role_authority_revision_space_chat_policies_no_truncate
BEFORE TRUNCATE ON role_space_chat_policies
FOR EACH STATEMENT EXECUTE FUNCTION role_authority_revision_floor_guard();
