ALTER TABLE chat_create_requests
    ADD COLUMN execution_kind TEXT NOT NULL DEFAULT 'legacy',
    ADD COLUMN operation_id UUID,
    ADD CONSTRAINT chat_create_execution_kind CHECK (
        (execution_kind='legacy' AND operation_id IS NULL)
        OR (execution_kind='space_composite_v1' AND operation_id IS NOT NULL)
    );

-- The prepared Chat is stored here, outside the ordinary/public chats relation.
-- No domain FK may delete this permanent operation/resource fence.
CREATE TABLE chat_space_composite_operations (
    operation_id UUID PRIMARY KEY,
    space_id UUID NOT NULL,
    chat_id UUID UNIQUE NOT NULL,
    node_id UUID UNIQUE NOT NULL,
    audit_event_id UUID UNIQUE NOT NULL,
    chat_type TEXT NOT NULL CHECK (chat_type IN ('group','channel')),
    state TEXT NOT NULL CHECK (state IN ('PREPARED','COMPLETE','ABORTED')),
    failure_code INTEGER NOT NULL DEFAULT 0 CHECK (failure_code>=0 AND failure_code<=16),
    creator_profile_id UUID,
    client_request_id UUID,
    create_request_sha256 BYTEA,
    binding_bytes BYTEA,
    response_bytes BYTEA,
    outcome_bytes BYTEA,
    created_at TIMESTAMPTZ NOT NULL,
    lease_token UUID,
    lease_expires_at TIMESTAMPTZ,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    attempt_count BIGINT NOT NULL DEFAULT 0 CHECK (attempt_count>=0),
    last_error_class TEXT,
    deletion_operation_id UUID,
    purge_generation BIGINT CHECK (purge_generation>0),
    local_purge_completed_at TIMESTAMPTZ,
    full_evidence_expires_at TIMESTAMPTZ,
    CHECK (operation_id<>'00000000-0000-0000-0000-000000000000'::uuid
        AND space_id<>'00000000-0000-0000-0000-000000000000'::uuid
        AND chat_id<>'00000000-0000-0000-0000-000000000000'::uuid
        AND node_id<>'00000000-0000-0000-0000-000000000000'::uuid
        AND audit_event_id<>'00000000-0000-0000-0000-000000000000'::uuid),
    CHECK (isfinite(created_at)),
    CHECK ((local_purge_completed_at IS NULL AND full_evidence_expires_at IS NULL)
        OR (state<>'PREPARED' AND deletion_operation_id IS NOT NULL AND purge_generation IS NOT NULL
            AND local_purge_completed_at IS NOT NULL AND full_evidence_expires_at IS NOT NULL
            AND isfinite(local_purge_completed_at) AND isfinite(full_evidence_expires_at)
            AND full_evidence_expires_at=local_purge_completed_at+interval '30 days')),
    CHECK ((binding_bytes IS NOT NULL AND creator_profile_id IS NOT NULL
            AND create_request_sha256 IS NOT NULL AND octet_length(create_request_sha256)=32
            AND response_bytes IS NOT NULL)
        OR (state<>'PREPARED' AND full_evidence_expires_at IS NOT NULL
            AND binding_bytes IS NULL AND creator_profile_id IS NULL AND client_request_id IS NULL
            AND create_request_sha256 IS NULL AND response_bytes IS NULL AND outcome_bytes IS NULL))
);
CREATE UNIQUE INDEX chat_space_composite_creator_key
    ON chat_space_composite_operations(creator_profile_id,client_request_id)
    WHERE client_request_id IS NOT NULL;

CREATE TABLE chat_space_composite_outbox (
    operation_id UUID NOT NULL REFERENCES chat_space_composite_operations(operation_id),
    effect_kind TEXT NOT NULL CHECK (effect_kind IN ('AUDIT','CHAT_CREATED')),
    event_id UUID UNIQUE NOT NULL,
    eligibility TEXT NOT NULL CHECK (eligibility IN ('HELD','READY','CANCELLED')),
    payload BYTEA,
    delivered_at TIMESTAMPTZ,
    lease_token UUID,
    lease_expires_at TIMESTAMPTZ,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    attempt_count BIGINT NOT NULL DEFAULT 0 CHECK (attempt_count>=0),
    last_error_class TEXT,
    PRIMARY KEY(operation_id,effect_kind),
    CHECK (payload IS NOT NULL OR eligibility='CANCELLED' OR delivered_at IS NOT NULL)
);

ALTER TABLE chat_space_lifecycle_fences ADD COLUMN draining_composites BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE chat_space_lifecycle_fences DROP CONSTRAINT chat_space_lifecycle_fences_check;
ALTER TABLE chat_space_lifecycle_fences ADD CONSTRAINT chat_space_lifecycle_fences_check CHECK (
    (generation=0 AND NOT draining_composites AND state='LIVE' AND deletion_operation_id IS NULL AND schedule_generation IS NULL
        AND manifest_id IS NULL AND manifest_sha256 IS NULL AND manifest_item_count IS NULL)
    OR (generation>0 AND deletion_operation_id IS NOT NULL AND schedule_generation IS NOT NULL AND schedule_generation>0
        AND ((NOT draining_composites AND manifest_id IS NOT NULL AND manifest_sha256 IS NOT NULL
            AND octet_length(manifest_sha256)=32 AND manifest_item_count IS NOT NULL AND manifest_item_count>=0)
          OR (draining_composites AND state='FROZEN' AND schedule_generation=generation
            AND manifest_id IS NULL AND manifest_sha256 IS NULL AND manifest_item_count IS NULL)))
);
ALTER TABLE chat_space_lifecycle_fences DROP CONSTRAINT chat_space_lifecycle_source_manifest_check;
ALTER TABLE chat_space_lifecycle_fences ADD CONSTRAINT chat_space_lifecycle_source_manifest_check CHECK (
    (generation=0 AND source_manifest_id IS NULL AND source_manifest_sha256 IS NULL AND source_manifest_item_count IS NULL)
    OR (generation>0 AND NOT draining_composites AND source_manifest_id IS NOT NULL
        AND source_manifest_sha256 IS NOT NULL AND octet_length(source_manifest_sha256)=32
        AND source_manifest_item_count IS NOT NULL AND source_manifest_item_count>=0)
    OR (generation>0 AND draining_composites AND source_manifest_id IS NULL
        AND source_manifest_sha256 IS NULL AND source_manifest_item_count IS NULL)
);
CREATE TABLE chat_space_composite_drains (
    space_id UUID PRIMARY KEY,
    deletion_operation_id UUID NOT NULL,
    schedule_generation BIGINT NOT NULL CHECK (schedule_generation>0),
    request_bytes BYTEA NOT NULL,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256)=32)
);
