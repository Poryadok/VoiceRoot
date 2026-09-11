-- Durable coordinator state for the accepted P3 Space deletion lifecycle.
CREATE TABLE space_lifecycle_operations (
    operation_id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    actor_profile_id UUID NOT NULL,
    space_id UUID NOT NULL,
    session_epoch BIGINT NOT NULL CHECK (session_epoch > 0),
    method TEXT NOT NULL CHECK (method = 'DELETE'),
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    confirmation_name_sha256 BYTEA NOT NULL CHECK (octet_length(confirmation_name_sha256) = 32),
    proof_digest_sha256 BYTEA NOT NULL CHECK (octet_length(proof_digest_sha256) = 32),
    binding_sha256 BYTEA NOT NULL CHECK (octet_length(binding_sha256) = 32),
    state TEXT NOT NULL CHECK (state IN ('SCHEDULE_PENDING','COMPLETED')),
    auth_receipt_bytes BYTEA,
    auth_receipt_sha256 BYTEA,
    outcome_bytes BYTEA,
    outcome_sha256 BYTEA,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK ((auth_receipt_bytes IS NULL) = (auth_receipt_sha256 IS NULL)),
    CHECK (auth_receipt_sha256 IS NULL OR octet_length(auth_receipt_sha256) = 32),
    CHECK ((outcome_bytes IS NULL) = (outcome_sha256 IS NULL)),
    CHECK (outcome_sha256 IS NULL OR octet_length(outcome_sha256) = 32),
    CHECK ((outcome_bytes IS NULL) = (completed_at IS NULL)),
    CHECK (state <> 'COMPLETED' OR completed_at IS NOT NULL)
);

CREATE UNIQUE INDEX space_lifecycle_operations_actor_operation_idx
    ON space_lifecycle_operations(actor_profile_id, operation_id);
CREATE INDEX space_lifecycle_operations_space_idx ON space_lifecycle_operations(space_id);

CREATE TABLE space_lifecycle_aggregates (
    space_id UUID PRIMARY KEY REFERENCES spaces(id),
    deletion_operation_id UUID NOT NULL UNIQUE,
    phase TEXT NOT NULL CHECK (phase IN (
        'LIVE','SCHEDULE_PENDING','FREEZE_PENDING','SCHEDULED',
        'RESTORE_DECIDED','PURGE_DECIDED','PURGING','PURGED'
    )),
    generation BIGINT NOT NULL CHECK (generation > 0),
    manifest_id UUID,
    manifest_sha256 BYTEA,
    manifest_item_count BIGINT,
    scheduled_at TIMESTAMPTZ,
    purge_after TIMESTAMPTZ,
    purge_decided_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    local_purge_completed BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK ((manifest_id IS NULL) = (manifest_sha256 IS NULL)),
    CHECK ((manifest_id IS NULL) = (manifest_item_count IS NULL)),
    CHECK (manifest_sha256 IS NULL OR octet_length(manifest_sha256) = 32),
    CHECK (manifest_item_count IS NULL OR manifest_item_count >= 0),
    CHECK ((scheduled_at IS NULL) = (purge_after IS NULL)),
    CHECK (purge_decided_at IS NULL OR purge_after IS NOT NULL),
    CHECK (NOT local_purge_completed OR phase IN ('PURGING','PURGED')),
    CHECK (completed_at IS NULL OR phase IN ('LIVE','PURGED'))
);

CREATE TABLE space_lifecycle_participants (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    participant_id SMALLINT NOT NULL CHECK (participant_id BETWEEN 1 AND 10),
    request_kind TEXT NOT NULL CHECK (request_kind IN ('FENCE','PURGE','ROLE_RETIREMENT')),
    progress TEXT NOT NULL CHECK (progress IN (
        'NOT_STARTED','IN_FLIGHT','COMPLETE','RETRYABLE_FAILURE','CONTRACT_MISMATCH'
    )),
    request_bytes BYTEA NOT NULL,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    receipt_bytes BYTEA,
    receipt_sha256 BYTEA,
    manifest_id UUID NOT NULL,
    manifest_sha256 BYTEA NOT NULL CHECK (octet_length(manifest_sha256) = 32),
    manifest_item_count BIGINT NOT NULL CHECK (manifest_item_count >= 0),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (space_id, generation, participant_id, request_kind),
    FOREIGN KEY (space_id) REFERENCES space_lifecycle_aggregates(space_id) ON DELETE CASCADE,
    CHECK ((receipt_bytes IS NULL) = (receipt_sha256 IS NULL)),
    CHECK (receipt_sha256 IS NULL OR octet_length(receipt_sha256) = 32),
    CHECK ((receipt_bytes IS NULL) = (completed_at IS NULL)),
    CHECK (progress = 'COMPLETE' OR receipt_bytes IS NULL),
    CHECK (progress <> 'COMPLETE' OR receipt_bytes IS NOT NULL),
    CHECK (request_kind <> 'ROLE_RETIREMENT' OR participant_id = 1),
    CHECK (request_kind <> 'PURGE' OR participant_id <> 1)
);

CREATE INDEX space_lifecycle_participants_operation_idx
    ON space_lifecycle_participants(deletion_operation_id, generation, participant_id);

CREATE TABLE space_lifecycle_outbox (
    event_id UUID PRIMARY KEY,
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    event_type TEXT NOT NULL CHECK (event_type IN ('space.deletion_scheduled','space.restored','space.deleted')),
    state TEXT NOT NULL CHECK (state IN ('BLOCKED','READY','DELIVERED')),
    occurred_at TIMESTAMPTZ,
    event_bytes BYTEA,
    event_sha256 BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    delivered_at TIMESTAMPTZ,
    UNIQUE (space_id, deletion_operation_id, generation, event_type),
    FOREIGN KEY (space_id) REFERENCES space_lifecycle_aggregates(space_id) ON DELETE CASCADE,
    CHECK ((event_bytes IS NULL) = (event_sha256 IS NULL)),
    CHECK (event_sha256 IS NULL OR octet_length(event_sha256) = 32),
    CHECK (state = 'BLOCKED' OR (occurred_at IS NOT NULL AND event_bytes IS NOT NULL)),
    CHECK (state <> 'DELIVERED' OR delivered_at IS NOT NULL),
    CHECK (state = 'DELIVERED' OR delivered_at IS NULL)
);

CREATE INDEX space_lifecycle_outbox_ready_idx
    ON space_lifecycle_outbox(created_at, event_id) WHERE state = 'READY';

-- Deliberately no foreign key: this minimal record survives deletion of spaces.
CREATE TABLE space_deletion_tombstones (
    space_id UUID PRIMARY KEY,
    owner_account_hmac BYTEA NOT NULL CHECK (octet_length(owner_account_hmac) = 32),
    actor_account_hmac BYTEA NOT NULL CHECK (octet_length(actor_account_hmac) = 32),
    key_version TEXT NOT NULL CHECK (key_version <> ''),
    reason TEXT NOT NULL CHECK (reason = 'OWNER_REQUESTED'),
    scheduled_at TIMESTAMP(6) WITHOUT TIME ZONE NOT NULL,
    purge_after TIMESTAMP(6) WITHOUT TIME ZONE NOT NULL,
    purge_decided_at TIMESTAMP(6) WITHOUT TIME ZONE NOT NULL,
    purged_at TIMESTAMP(6) WITHOUT TIME ZONE NOT NULL,
    retain_until TIMESTAMP(6) WITHOUT TIME ZONE NOT NULL,
    CHECK (purge_decided_at >= purge_after),
    CHECK (purged_at >= purge_decided_at),
    CHECK (retain_until = purged_at + interval '365 days')
);
