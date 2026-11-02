ALTER TABLE gis_sessions
    ADD COLUMN host_profile_id uuid,
    ADD COLUMN roster_hash bytea CHECK (roster_hash IS NULL OR octet_length(roster_hash)=32),
    ADD COLUMN lease_expires_at timestamptz,
    ADD COLUMN roster_receipt_id uuid,
    ADD COLUMN closed_at timestamptz;

ALTER TABLE gis_session_operations
    ADD COLUMN error_outcome text CHECK (error_outcome IN ('transient','permanent','unknown'));

-- The historical last-error migration also used version 14. Preserve the shipped
-- scoped-identity migration and restore its missing nullable outcome here.
UPDATE gis_session_operations SET error_outcome='unknown' WHERE error_code IS NOT NULL;

ALTER TABLE gis_session_operations
    DROP CONSTRAINT gis_session_operations_operation_kind_check,
    ADD CONSTRAINT gis_session_operations_operation_kind_check
        CHECK (operation_kind IN ('create','close','failure','roster'));

CREATE INDEX gis_sessions_roster_lease_due
    ON gis_sessions(lease_expires_at)
    WHERE session_status IN ('provisioning','active') AND lease_expires_at IS NOT NULL;

CREATE TABLE gis_session_roster_updates (
    operation_id uuid NOT NULL,
    session_id uuid NOT NULL,
    application_id uuid NOT NULL,
    environment_id uuid NOT NULL,
    request_hash bytea NOT NULL CHECK (octet_length(request_hash)=32),
    roster_revision bigint NOT NULL CHECK (roster_revision > 0),
    roster_hash bytea NOT NULL CHECK (octet_length(roster_hash)=32),
    members uuid[] NOT NULL,
    host_profile_id uuid,
    outcome text NOT NULL CHECK (outcome IN ('pending','stale','replayed','applied','failed')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (application_id,environment_id,operation_id),
    FOREIGN KEY (application_id,environment_id,operation_id)
        REFERENCES gis_session_operations(application_id,environment_id,operation_id),
    FOREIGN KEY (session_id,application_id,environment_id)
        REFERENCES gis_sessions(id,application_id,environment_id)
);

CREATE INDEX gis_session_roster_updates_pending
    ON gis_session_roster_updates(created_at,operation_id)
    WHERE outcome='pending';
