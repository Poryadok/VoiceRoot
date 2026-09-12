-- BE-116: immutable Space audit ledger and transactional delivery outbox.

ALTER TABLE audit_log
    ADD CONSTRAINT audit_log_details_4k CHECK (octet_length(details::text) <= 4096),
    ADD CONSTRAINT audit_log_closed_registry CHECK (
        jsonb_typeof(details) = 'object'
        AND (
            (action = 'invite_revoked' AND target_type = 'invite' AND details = '{}'::jsonb)
            OR (action = 'member_kicked' AND target_type = 'profile' AND details = '{}'::jsonb)
            OR (
                action = 'member_banned' AND target_type = 'account'
                AND details - 'reason' = '{}'::jsonb
                AND (NOT details ? 'reason' OR jsonb_typeof(details -> 'reason') = 'string')
            )
            OR (action = 'member_unbanned' AND target_type = 'account' AND details = '{}'::jsonb)
            OR (
                action = 'member_timed_out' AND target_type = 'profile'
                AND details - ARRAY['duration_seconds','reason'] = '{}'::jsonb
                AND details ? 'duration_seconds'
                AND jsonb_typeof(details -> 'duration_seconds') = 'number'
                AND (NOT details ? 'reason' OR jsonb_typeof(details -> 'reason') = 'string')
            )
            OR (action = 'member_timeout_removed' AND target_type = 'profile' AND details = '{}'::jsonb)
            OR (action = 'ownership_transferred' AND target_type = 'profile' AND details = '{}'::jsonb)
        )
    );

CREATE TABLE audit_cursor_signing_key (
    singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
    key_bytes BYTEA NOT NULL CHECK (octet_length(key_bytes) = 32),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

INSERT INTO audit_cursor_signing_key(singleton,key_bytes)
VALUES(true,gen_random_bytes(32));

CREATE INDEX audit_log_space_actor_created_idx
    ON audit_log(space_id, actor_profile_id, created_at DESC, id DESC);

CREATE INDEX audit_log_space_action_created_idx
    ON audit_log(space_id, action, created_at DESC, id DESC);

CREATE TABLE audit_outbox (
    audit_event_id UUID PRIMARY KEY REFERENCES audit_log(id) ON DELETE CASCADE,
    space_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    lease_token UUID,
    lease_expires_at TIMESTAMPTZ,
    last_failure_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    CHECK ((lease_token IS NULL) = (lease_expires_at IS NULL)),
    CHECK ((attempt_count = 0) = (last_failure_at IS NULL)),
    CHECK (delivered_at IS NULL OR (lease_token IS NULL AND lease_expires_at IS NULL))
);

-- The disabled legacy ownership saga is the only BE-116 path which can undo an
-- audit insert. Persist only a hash of its one-use, row-bound capability.
CREATE TABLE audit_compensation_authorizations (
    audit_event_id UUID PRIMARY KEY REFERENCES audit_log(id) ON DELETE CASCADE,
    space_id UUID NOT NULL,
    previous_owner_profile_id UUID NOT NULL,
    new_owner_profile_id UUID NOT NULL,
    capability_sha256 BYTEA NOT NULL CHECK (octet_length(capability_sha256) = 32),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp() + interval '1 minute',
    CHECK (previous_owner_profile_id <> new_owner_profile_id)
);

CREATE INDEX audit_outbox_delivery_ready_idx
    ON audit_outbox(next_attempt_at, created_at, audit_event_id)
    WHERE delivered_at IS NULL;

CREATE OR REPLACE FUNCTION queue_space_audit_outbox()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO audit_outbox(audit_event_id, space_id, created_at)
    VALUES (NEW.id, NEW.space_id, NEW.created_at);
    RETURN NEW;
END
$$;

INSERT INTO audit_outbox(audit_event_id, space_id, created_at)
SELECT id, space_id, created_at
FROM audit_log
ON CONFLICT (audit_event_id) DO NOTHING;

CREATE TRIGGER audit_log_queue_outbox
AFTER INSERT ON audit_log
FOR EACH ROW EXECUTE FUNCTION queue_space_audit_outbox();

CREATE OR REPLACE FUNCTION guard_space_audit_immutability()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    delete_mode TEXT := current_setting('voice.audit_delete_mode', true);
    compensation_token TEXT := current_setting('voice.audit_compensation_token', true);
    compensation_outbox audit_outbox%ROWTYPE;
    compensation_authz audit_compensation_authorizations%ROWTYPE;
    compensation_space_owner UUID;
    compensation_outbox_found BOOLEAN := false;
    compensation_authz_found BOOLEAN := false;
    compensation_space_found BOOLEAN := false;
BEGIN
    IF TG_OP = 'UPDATE' THEN
        RAISE EXCEPTION 'audit_log rows are immutable' USING ERRCODE = '55000';
    END IF;
    IF delete_mode = 'retention'
       AND OLD.created_at < clock_timestamp() - interval '365 days'
       AND EXISTS (
           SELECT 1 FROM audit_outbox
           WHERE audit_event_id = OLD.id AND delivered_at IS NOT NULL
       ) THEN
        RETURN OLD;
    END IF;

    IF delete_mode = 'compensation' THEN
        -- ClaimAuditOutbox serializes on this exact row. Lock it before deciding:
        -- after a concurrent claim commits, FOR UPDATE returns the claimed version.
        SELECT outbox.* INTO compensation_outbox
        FROM audit_outbox AS outbox
        WHERE outbox.audit_event_id = OLD.id
        FOR UPDATE;
        compensation_outbox_found := FOUND;

        IF compensation_outbox_found THEN
            -- Keep the remaining exact capability/ownership tuple stable until
            -- the audit delete and its FK cascades have completed.
            SELECT authz.* INTO compensation_authz
            FROM audit_compensation_authorizations AS authz
            WHERE authz.audit_event_id = OLD.id
            FOR UPDATE;
            compensation_authz_found := FOUND;

            SELECT space.owner_profile_id INTO compensation_space_owner
            FROM spaces AS space
            WHERE space.id = OLD.space_id
            FOR UPDATE;
            compensation_space_found := FOUND;
        END IF;

        IF compensation_outbox_found
           AND compensation_authz_found
           AND compensation_space_found
           AND compensation_token ~ '^[0-9a-f]{64}$'
           AND OLD.action = 'ownership_transferred'
           AND OLD.target_type = 'profile'
           AND OLD.details = '{}'::jsonb
           AND compensation_authz.audit_event_id = OLD.id
           AND compensation_authz.space_id = OLD.space_id
           AND compensation_authz.previous_owner_profile_id = OLD.actor_profile_id
           AND compensation_authz.new_owner_profile_id = OLD.target_id
           AND compensation_authz.capability_sha256 = digest(decode(compensation_token, 'hex'), 'sha256')
           AND clock_timestamp() < compensation_authz.expires_at
           AND compensation_space_owner = compensation_authz.new_owner_profile_id
           AND compensation_outbox.audit_event_id = OLD.id
           AND compensation_outbox.space_id = OLD.space_id
           AND compensation_outbox.delivered_at IS NULL
           AND compensation_outbox.attempt_count = 0
           AND compensation_outbox.lease_token IS NULL
           AND compensation_outbox.lease_expires_at IS NULL
           AND compensation_outbox.last_failure_at IS NULL THEN
            RETURN OLD;
        END IF;
    END IF;
    RAISE EXCEPTION 'audit_log rows are immutable' USING ERRCODE = '55000';
END
$$;

CREATE TRIGGER audit_log_immutable
BEFORE UPDATE OR DELETE ON audit_log
FOR EACH ROW EXECUTE FUNCTION guard_space_audit_immutability();

CREATE FUNCTION guard_space_hard_delete_until_p3()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'direct Space deletion is disabled until the terminal P3 purge coordinator is active'
        USING ERRCODE = '55000';
END
$$;

CREATE TRIGGER spaces_hard_delete_disabled
BEFORE DELETE ON spaces
FOR EACH ROW EXECUTE FUNCTION guard_space_hard_delete_until_p3();
