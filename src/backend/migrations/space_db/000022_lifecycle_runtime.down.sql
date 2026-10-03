-- Refuse rollback once Space rows have been irreversibly removed.
DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM space_lifecycle_aggregates a WHERE NOT EXISTS(SELECT 1 FROM spaces s WHERE s.id=a.space_id)) THEN
        RAISE EXCEPTION 'cannot roll back lifecycle runtime after terminal local purge';
    END IF;
END $$;
DROP TRIGGER space_lifecycle_receipt_progress ON space_lifecycle_participants;
DROP TRIGGER space_lifecycle_phase_progress ON space_lifecycle_aggregates;
DROP FUNCTION space_lifecycle_mark_progress();
DROP INDEX space_lifecycle_recovery_due_idx;
ALTER TABLE space_lifecycle_aggregates DROP COLUMN retry_attempt,DROP COLUMN next_attempt_at,DROP COLUMN progress_at,DROP COLUMN last_failure_at;
ALTER TABLE space_lifecycle_aggregates ADD CONSTRAINT space_lifecycle_aggregates_space_id_fkey FOREIGN KEY(space_id) REFERENCES spaces(id);
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

CREATE OR REPLACE FUNCTION guard_space_hard_delete_until_p3()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'direct Space deletion is disabled until the terminal P3 purge coordinator is active'
        USING ERRCODE = '55000';
END
$$;

DROP FUNCTION space_terminal_purge_ready(UUID);
