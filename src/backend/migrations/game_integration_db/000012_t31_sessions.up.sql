CREATE TABLE gis_sessions (
    id uuid PRIMARY KEY,
    application_id uuid NOT NULL,
    environment_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind IN ('party','match','fleet')),
    external_key text NOT NULL,
    parent_party_key text,
    display_name text,
    session_status text NOT NULL CHECK (session_status IN ('provisioning','active','closing','closed','failed')),
    stage text NOT NULL,
    roster_revision bigint NOT NULL,
    roster_complete boolean NOT NULL,
    members uuid[] NOT NULL,
    chat_id uuid,
    chat_owner_session_id uuid,
    chat_create_receipt_id uuid,
    chat_roster_receipt_id uuid,
    voice_room_id uuid,
    voice_provision_receipt_id uuid,
    role_grant_receipt_id uuid,
    terminalization_operation_id uuid,
    terminalization_kind text,
    terminalization_stage text,
    voice_close_receipt_id uuid,
    role_revoke_receipt_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (application_id, environment_id, kind, external_key),
    FOREIGN KEY (chat_owner_session_id) REFERENCES gis_sessions(id)
);

CREATE TABLE gis_session_operations (
    operation_id uuid PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES gis_sessions(id),
    application_id uuid NOT NULL,
    environment_id uuid NOT NULL,
    operation_kind text NOT NULL CHECK (operation_kind IN ('create','close','failure')),
    request_hash bytea NOT NULL CHECK (octet_length(request_hash)=32),
    status text NOT NULL CHECK (status IN ('pending','succeeded','failed')),
    stage text NOT NULL,
    error_code text,
    active_event_id uuid,
    voice_close_receipt_id uuid,
    role_revoke_receipt_id uuid,
    lease_owner uuid,
    lease_until timestamptz,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    attempts integer NOT NULL DEFAULT 0,
    stage_retry_count integer NOT NULL DEFAULT 0 CHECK (stage_retry_count >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX gis_session_operations_due ON gis_session_operations(next_attempt_at) WHERE status='pending';

CREATE TABLE gis_session_owner_receipts (
    operation_id uuid NOT NULL REFERENCES gis_session_operations(operation_id),
    stage text NOT NULL,
    owner_operation_id uuid NOT NULL,
    owner_request_hash bytea NOT NULL CHECK (octet_length(owner_request_hash)=32),
    resource_id uuid NOT NULL,
    receipt_id uuid NOT NULL,
    receipt_bytes bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(operation_id, stage),
    UNIQUE(owner_operation_id)
);

CREATE TABLE gis_session_outbox (
    event_id uuid PRIMARY KEY,
    application_id uuid NOT NULL,
    environment_id uuid NOT NULL,
    session_id uuid NOT NULL REFERENCES gis_sessions(id),
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    payload_bytes bytea NOT NULL,
    payload_sha256 bytea NOT NULL CHECK (octet_length(payload_sha256)=32),
    created_at timestamptz NOT NULL DEFAULT now(),
    delivered_at timestamptz,
    attempts integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    claim_lease_id uuid,
    claim_lease_until timestamptz,
    consumer_ack_lease_id uuid,
    consumer_ack_payload_sha256 bytea CHECK (consumer_ack_payload_sha256 IS NULL OR octet_length(consumer_ack_payload_sha256)=32),
    CHECK ((claim_lease_id IS NULL) = (claim_lease_until IS NULL)),
    CHECK ((consumer_ack_lease_id IS NULL) = (consumer_ack_payload_sha256 IS NULL)),
    CHECK (delivered_at IS NULL OR consumer_ack_lease_id IS NOT NULL)
);
CREATE UNIQUE INDEX gis_session_outbox_active_once ON gis_session_outbox(session_id,event_type);
CREATE INDEX gis_session_outbox_claimable ON gis_session_outbox(application_id,environment_id,created_at,event_id)
    WHERE delivered_at IS NULL;

CREATE TABLE gis_session_inbox (
    application_id uuid NOT NULL,
    environment_id uuid NOT NULL,
    event_id uuid NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(application_id,environment_id,event_id)
);
