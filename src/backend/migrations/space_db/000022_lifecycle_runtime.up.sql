-- Lifecycle evidence survives the local Space row for bounded recovery.
ALTER TABLE space_lifecycle_aggregates DROP CONSTRAINT space_lifecycle_aggregates_space_id_fkey;

CREATE FUNCTION space_terminal_purge_ready(target UUID) RETURNS boolean
LANGUAGE sql STABLE AS $$
SELECT EXISTS (
    SELECT 1 FROM space_lifecycle_aggregates a
    JOIN space_deletion_tombstones t ON t.space_id=a.space_id
    WHERE a.space_id=target AND a.phase='PURGED' AND a.local_purge_completed
      AND clock_timestamp() >= a.purge_after
      AND (SELECT count(*) FROM space_lifecycle_participants p
           WHERE p.space_id=a.space_id AND p.deletion_operation_id=a.deletion_operation_id
             AND p.generation=a.generation AND p.progress='COMPLETE'
             AND ((p.request_kind='ROLE_RETIREMENT' AND p.participant_id=1)
               OR (p.request_kind='PURGE' AND p.participant_id BETWEEN 2 AND 10)))=10
)
$$;

CREATE OR REPLACE FUNCTION guard_space_hard_delete_until_p3() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF current_setting('voice.audit_delete_mode',true)='space_purge'
       AND space_terminal_purge_ready(OLD.id) THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION 'direct Space deletion requires terminal P3 completion' USING ERRCODE='55000';
END
$$;

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
    IF TG_OP = 'DELETE' AND delete_mode = 'space_purge' AND space_terminal_purge_ready(OLD.space_id) THEN
        RETURN OLD;
    END IF;
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

ALTER TABLE space_lifecycle_aggregates
    ADD COLUMN retry_attempt INTEGER NOT NULL DEFAULT 0 CHECK (retry_attempt >= 0),
    ADD COLUMN next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    ADD COLUMN progress_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    ADD COLUMN last_failure_at TIMESTAMPTZ;
CREATE INDEX space_lifecycle_recovery_due_idx ON space_lifecycle_aggregates(next_attempt_at,space_id)
    WHERE phase NOT IN ('LIVE','PURGED');

-- Only new state/evidence resets the stalled timer, never an exact replay.
CREATE FUNCTION space_lifecycle_mark_progress() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_TABLE_NAME='space_lifecycle_aggregates' THEN
        IF OLD.phase IS DISTINCT FROM NEW.phase OR OLD.generation IS DISTINCT FROM NEW.generation THEN
            NEW.progress_at=clock_timestamp(); NEW.retry_attempt=0; NEW.next_attempt_at=clock_timestamp();
        END IF;
        RETURN NEW;
    END IF;
    IF NEW.progress='COMPLETE' AND (TG_OP='INSERT' OR OLD.progress IS DISTINCT FROM NEW.progress) THEN
        UPDATE space_lifecycle_aggregates SET progress_at=clock_timestamp(),retry_attempt=0,next_attempt_at=clock_timestamp()
        WHERE space_id=NEW.space_id AND generation=NEW.generation;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER space_lifecycle_phase_progress BEFORE UPDATE ON space_lifecycle_aggregates
    FOR EACH ROW EXECUTE FUNCTION space_lifecycle_mark_progress();
CREATE TRIGGER space_lifecycle_receipt_progress AFTER INSERT OR UPDATE ON space_lifecycle_participants
    FOR EACH ROW EXECUTE FUNCTION space_lifecycle_mark_progress();
