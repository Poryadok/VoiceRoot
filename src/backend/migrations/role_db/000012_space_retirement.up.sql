-- Permanent Role retirement evidence extends the lifecycle fence introduced by
-- 000010. Full protobuf bytes are retained for at least 30 days; the compact
-- receipt and the retired lifecycle fence never expire.
CREATE TABLE role_space_retirement_receipts (
    space_id UUID PRIMARY KEY,
    deletion_operation_id UUID NOT NULL UNIQUE,
    protocol_version INTEGER NOT NULL CHECK (protocol_version = 1),
    generation BIGINT NOT NULL CHECK (generation > 0),
    purge_decided_at TIMESTAMPTZ NOT NULL,
    manifest_id UUID NOT NULL,
    manifest_item_count NUMERIC(20, 0) NOT NULL CHECK (
        manifest_item_count >= 0 AND manifest_item_count <= 18446744073709551615
    ),
    receipt_id UUID NOT NULL UNIQUE,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    manifest_sha256 BYTEA NOT NULL CHECK (octet_length(manifest_sha256) = 32),
    request_bytes BYTEA NULL,
    receipt_bytes BYTEA NULL,
    retired_at TIMESTAMPTZ NOT NULL,
    full_bytes_retain_until TIMESTAMPTZ NOT NULL,
    CHECK ((request_bytes IS NULL) = (receipt_bytes IS NULL)),
    CHECK (request_bytes IS NULL OR octet_length(request_bytes) > 0),
    CHECK (receipt_bytes IS NULL OR octet_length(receipt_bytes) > 0)
);

CREATE FUNCTION role_space_retirement_receipt_insert() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.request_bytes IS NULL OR NEW.receipt_bytes IS NULL THEN
        RAISE EXCEPTION 'Role retirement full receipt bytes are required at completion'
            USING ERRCODE = '23514';
    END IF;
    NEW.full_bytes_retain_until := NEW.retired_at + interval '30 days';
    RETURN NEW;
END
$$;

CREATE FUNCTION role_space_retirement_receipt_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'Role retirement compact receipt is permanent'
            USING ERRCODE = '55000';
    END IF;
    IF NEW.space_id IS DISTINCT FROM OLD.space_id
       OR NEW.deletion_operation_id IS DISTINCT FROM OLD.deletion_operation_id
       OR NEW.protocol_version IS DISTINCT FROM OLD.protocol_version
       OR NEW.generation IS DISTINCT FROM OLD.generation
       OR NEW.purge_decided_at IS DISTINCT FROM OLD.purge_decided_at
       OR NEW.manifest_id IS DISTINCT FROM OLD.manifest_id
       OR NEW.manifest_item_count IS DISTINCT FROM OLD.manifest_item_count
       OR NEW.receipt_id IS DISTINCT FROM OLD.receipt_id
       OR NEW.request_sha256 IS DISTINCT FROM OLD.request_sha256
       OR NEW.manifest_sha256 IS DISTINCT FROM OLD.manifest_sha256
       OR NEW.retired_at IS DISTINCT FROM OLD.retired_at
       OR NEW.full_bytes_retain_until IS DISTINCT FROM OLD.full_bytes_retain_until THEN
        RAISE EXCEPTION 'Role retirement compact receipt is immutable'
            USING ERRCODE = '55000';
    END IF;
    IF NEW.request_bytes IS DISTINCT FROM OLD.request_bytes
       OR NEW.receipt_bytes IS DISTINCT FROM OLD.receipt_bytes THEN
        IF OLD.request_bytes IS NULL OR OLD.receipt_bytes IS NULL
           OR NEW.request_bytes IS NOT NULL OR NEW.receipt_bytes IS NOT NULL
           OR clock_timestamp() < OLD.full_bytes_retain_until THEN
            RAISE EXCEPTION 'Role retirement full receipt bytes remain protected'
                USING ERRCODE = '55000';
        END IF;
    END IF;
    RETURN NEW;
END
$$;

CREATE FUNCTION role_space_retirement_fence_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.retired_at IS NOT NULL
       AND (TG_OP = 'DELETE'
            OR NEW.space_id IS DISTINCT FROM OLD.space_id
            OR NEW.retired_at IS DISTINCT FROM OLD.retired_at) THEN
        RAISE EXCEPTION 'Role retirement fence is permanent'
            USING ERRCODE = '55000';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END
$$;

CREATE TRIGGER role_space_retirement_receipt_set_boundary
BEFORE INSERT ON role_space_retirement_receipts
FOR EACH ROW EXECUTE FUNCTION role_space_retirement_receipt_insert();

CREATE TRIGGER role_space_retirement_receipt_no_change
BEFORE UPDATE OR DELETE ON role_space_retirement_receipts
FOR EACH ROW EXECUTE FUNCTION role_space_retirement_receipt_immutable();

CREATE TRIGGER role_space_retirement_fence_no_change
BEFORE UPDATE OR DELETE ON role_space_lifecycle
FOR EACH ROW EXECUTE FUNCTION role_space_retirement_fence_immutable();

-- During retirement the lifecycle transition emits the one final Space-wide
-- invalidation. Cascading ordinary Role deletion must not emit duplicates.
CREATE OR REPLACE FUNCTION role_voice_policy_role_changed() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    changed_space_id UUID;
BEGIN
    changed_space_id := CASE WHEN TG_OP = 'DELETE' THEN OLD.space_id ELSE NEW.space_id END;
    IF current_setting('voice.role_retirement_space_id', true) = changed_space_id::text THEN
        IF TG_OP = 'DELETE' THEN
            RETURN OLD;
        END IF;
        RETURN NEW;
    END IF;
    PERFORM role_voice_policy_bump(changed_space_id, NULL, NULL);
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END
$$;

CREATE OR REPLACE FUNCTION role_voice_policy_member_changed() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    changed_space_id UUID;
    changed_profile_id UUID;
    changed_role_id UUID;
BEGIN
    IF TG_OP = 'DELETE' THEN
        changed_space_id := OLD.space_id;
        changed_profile_id := OLD.profile_id;
        changed_role_id := OLD.role_id;
    ELSE
        changed_space_id := NEW.space_id;
        changed_profile_id := NEW.profile_id;
        changed_role_id := NEW.role_id;
    END IF;

    IF current_setting('voice.role_retirement_space_id', true) = changed_space_id::text THEN
        IF TG_OP = 'DELETE' THEN
            RETURN OLD;
        END IF;
        RETURN NEW;
    END IF;
    IF TG_OP = 'DELETE' AND NOT EXISTS (SELECT 1 FROM roles WHERE id = changed_role_id) THEN
        RETURN OLD;
    END IF;

    PERFORM role_voice_policy_bump(changed_space_id, NULL, changed_profile_id);
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END
$$;
