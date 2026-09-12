BEGIN;

CREATE TABLE subscription_provider_event_fences (
    provider TEXT NOT NULL,
    provider_event_hmac BYTEA NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    terminal_outcome_class TEXT NOT NULL,
    key_version TEXT NOT NULL,
    retain_until TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (provider, provider_event_hmac),
    CONSTRAINT subscription_provider_event_fences_provider_check
        CHECK (provider IN ('paddle', 'cloudpayments')),
    CONSTRAINT subscription_provider_event_fences_hmac_check
        CHECK (octet_length(provider_event_hmac) = 32),
    CONSTRAINT subscription_provider_event_fences_outcome_check
        CHECK (btrim(terminal_outcome_class) <> ''),
    CONSTRAINT subscription_provider_event_fences_key_version_check
        CHECK (btrim(key_version) <> ''),
    CONSTRAINT subscription_provider_event_fences_retention_check
        CHECK (retain_until = 'infinity'::timestamptz)
);

CREATE TABLE subscription_space_lifecycle_fences (
    space_id UUID PRIMARY KEY,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL,
    state TEXT NOT NULL,
    manifest_id TEXT NOT NULL,
    manifest_sha256 BYTEA NOT NULL,
    manifest_item_count BIGINT NOT NULL,
    request_sha256 BYTEA NOT NULL,
    receipt_id UUID NOT NULL UNIQUE,
    applied_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT subscription_space_lifecycle_fences_generation_check CHECK (generation > 0),
    CONSTRAINT subscription_space_lifecycle_fences_state_check
        CHECK (state IN ('FROZEN', 'LIVE', 'PURGE_DECIDED', 'PURGED')),
    CONSTRAINT subscription_space_lifecycle_fences_manifest_id_check CHECK (btrim(manifest_id) <> ''),
    CONSTRAINT subscription_space_lifecycle_fences_manifest_hash_check CHECK (octet_length(manifest_sha256) = 32),
    CONSTRAINT subscription_space_lifecycle_fences_manifest_count_check CHECK (manifest_item_count >= 0),
    CONSTRAINT subscription_space_lifecycle_fences_request_hash_check CHECK (octet_length(request_sha256) = 32)
);

CREATE TABLE subscription_space_lifecycle_operations (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL,
    operation_kind TEXT NOT NULL,
    request_sha256 BYTEA NOT NULL,
    request_bytes BYTEA,
    receipt_id UUID UNIQUE,
    receipt_bytes BYTEA,
    terminal_state TEXT NOT NULL,
    completed_at TIMESTAMPTZ,
    retain_until TIMESTAMPTZ,
    provider_cancel_state TEXT,
    provider_cancel_attempts INTEGER NOT NULL,
    provider_cancel_idempotency_key TEXT,
    provider_cancel_last_error TEXT,
    provider_cancel_next_attempt_at TIMESTAMPTZ,
    PRIMARY KEY (space_id, deletion_operation_id, generation, operation_kind),
    CONSTRAINT subscription_space_lifecycle_operations_generation_check CHECK (generation > 0),
    CONSTRAINT subscription_space_lifecycle_operations_kind_check CHECK (operation_kind IN ('FENCE', 'PURGE')),
    CONSTRAINT subscription_space_lifecycle_operations_request_hash_check CHECK (octet_length(request_sha256) = 32),
    CONSTRAINT subscription_space_lifecycle_operations_terminal_check CHECK (terminal_state IN ('PENDING', 'COMPLETED')),
    CONSTRAINT subscription_space_lifecycle_operations_attempts_check CHECK (provider_cancel_attempts >= 0),
    CONSTRAINT subscription_space_lifecycle_operations_cancel_state_check
        CHECK (provider_cancel_state IS NULL OR provider_cancel_state IN ('RETRYABLE', 'COMPLETED')),
    CONSTRAINT subscription_space_lifecycle_operations_cancel_key_check
        CHECK (provider_cancel_idempotency_key IS NULL OR btrim(provider_cancel_idempotency_key) <> ''),
    CONSTRAINT subscription_space_lifecycle_operations_coherence_check CHECK (
        (
            operation_kind = 'FENCE'
            AND terminal_state = 'COMPLETED'
            AND receipt_id IS NOT NULL
            AND completed_at IS NOT NULL
            AND retain_until = completed_at + interval '30 days'
            AND ((request_bytes IS NULL AND receipt_bytes IS NULL) OR (request_bytes IS NOT NULL AND receipt_bytes IS NOT NULL))
            AND provider_cancel_state IS NULL
            AND provider_cancel_attempts = 0
            AND provider_cancel_idempotency_key IS NULL
            AND provider_cancel_last_error IS NULL
            AND provider_cancel_next_attempt_at IS NULL
        ) OR (
            operation_kind = 'PURGE'
            AND provider_cancel_idempotency_key IS NOT NULL
            AND (
                (
                    terminal_state = 'PENDING'
                    AND request_bytes IS NOT NULL
                    AND receipt_id IS NULL
                    AND receipt_bytes IS NULL
                    AND completed_at IS NULL
                    AND retain_until IS NULL
                    AND provider_cancel_state = 'RETRYABLE'
                    AND provider_cancel_attempts > 0
                    AND provider_cancel_last_error IS NOT NULL
                    AND btrim(provider_cancel_last_error) <> ''
                    AND provider_cancel_next_attempt_at IS NOT NULL
                ) OR (
                    terminal_state = 'COMPLETED'
                    AND receipt_id IS NOT NULL
                    AND completed_at IS NOT NULL
                    AND retain_until = completed_at + interval '30 days'
                    AND ((request_bytes IS NULL AND receipt_bytes IS NULL) OR (request_bytes IS NOT NULL AND receipt_bytes IS NOT NULL))
                    AND provider_cancel_state = 'COMPLETED'
                    AND provider_cancel_attempts > 0
                    AND provider_cancel_last_error IS NULL
                    AND provider_cancel_next_attempt_at IS NULL
                )
            )
        )
    )
);

CREATE FUNCTION subscription_reject_evidence_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF current_setting('voice.subscription_evidence_write', true) IS DISTINCT FROM 'on' THEN
        RAISE EXCEPTION USING ERRCODE = '55000', MESSAGE = 'subscription durable evidence is immutable';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END
$$;

CREATE TRIGGER subscription_provider_event_fences_immutable
BEFORE UPDATE OR DELETE ON subscription_provider_event_fences
FOR EACH ROW EXECUTE FUNCTION subscription_reject_evidence_mutation();

CREATE TRIGGER subscription_space_lifecycle_fences_immutable
BEFORE UPDATE OR DELETE ON subscription_space_lifecycle_fences
FOR EACH ROW EXECUTE FUNCTION subscription_reject_evidence_mutation();

CREATE TRIGGER subscription_space_lifecycle_operations_immutable
BEFORE UPDATE OR DELETE ON subscription_space_lifecycle_operations
FOR EACH ROW EXECUTE FUNCTION subscription_reject_evidence_mutation();

COMMIT;
