-- Immutable audit requests are committed inside the ordinary Role transaction.
-- No cross-service foreign keys: Space acknowledges through its protected API.
CREATE TABLE role_space_audit_outbox (
    audit_event_id UUID PRIMARY KEY,
    space_id UUID NOT NULL,
    request_bytes BYTEA NOT NULL CHECK (octet_length(request_bytes) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    lease_token UUID,
    lease_expires_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    CHECK ((lease_token IS NULL) = (lease_expires_at IS NULL)),
    CHECK (delivered_at IS NULL OR lease_token IS NULL)
);

CREATE INDEX role_space_audit_outbox_due
    ON role_space_audit_outbox(next_attempt_at, created_at, audit_event_id)
    WHERE delivered_at IS NULL;

CREATE FUNCTION role_space_audit_outbox_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF (NEW.audit_event_id, NEW.space_id, NEW.request_bytes, NEW.created_at)
       IS DISTINCT FROM
       (OLD.audit_event_id, OLD.space_id, OLD.request_bytes, OLD.created_at) THEN
        RAISE EXCEPTION 'role audit request is immutable';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER role_space_audit_outbox_immutable
BEFORE UPDATE ON role_space_audit_outbox
FOR EACH ROW EXECUTE FUNCTION role_space_audit_outbox_immutable();
