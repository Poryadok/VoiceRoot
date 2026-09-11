BEGIN;

CREATE TABLE IF NOT EXISTS voice_lifecycle_redis_divergences (
    divergence_id UUID PRIMARY KEY,
    actor_profile_id UUID NOT NULL,
    operation_id UUID NOT NULL,
    classification TEXT NOT NULL,
    redis_class TEXT NOT NULL,
    redis_schema_version SMALLINT NULL,
    redis_state TEXT NULL,
    observation_digest BYTEA NOT NULL,
    subject_profile_id UUID NULL,
    classified_pg_state TEXT NULL,
    classified_lease_fence BIGINT NULL,
    first_observed_at TIMESTAMPTZ NOT NULL,
    last_observed_at TIMESTAMPTZ NOT NULL,
    resolution_id UUID NULL,
    resolution_kind TEXT NULL,
    resolution_evidence_digest BYTEA NULL,
    resolved_by_account_id UUID NULL,
    resolved_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT voice_lifecycle_redis_divergences_divergence_id_not_nil CHECK (divergence_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_redis_divergences_actor_profile_id_not_nil CHECK (actor_profile_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_redis_divergences_operation_id_not_nil CHECK (operation_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_redis_divergences_subject_profile_id_not_nil CHECK (subject_profile_id IS NULL OR subject_profile_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_redis_divergences_resolution_id_not_nil CHECK (resolution_id IS NULL OR resolution_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_redis_divergences_resolved_by_not_nil CHECK (resolved_by_account_id IS NULL OR resolved_by_account_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_redis_divergences_classification_check CHECK (classification IN ('orphan','decided','completed','quarantined') AND btrim(classification) = classification),
    CONSTRAINT voice_lifecycle_redis_divergences_redis_class_check CHECK (redis_class IN ('legacy_schema','malformed','binding_mismatch','owner_mismatch','state_order_mismatch','receipt_mismatch','deadline_mismatch','ttl_invalid') AND btrim(redis_class) = redis_class),
    CONSTRAINT voice_lifecycle_redis_divergences_redis_schema_version_check CHECK (redis_schema_version IS NULL OR redis_schema_version > 0),
    CONSTRAINT voice_lifecycle_redis_divergences_redis_state_check CHECK (redis_state IS NULL OR redis_state IN ('pending','completed')),
    CONSTRAINT voice_lifecycle_redis_divergences_observation_digest_check CHECK (octet_length(observation_digest) = 32),
    CONSTRAINT voice_lifecycle_redis_divergences_resolution_digest_check CHECK (resolution_evidence_digest IS NULL OR octet_length(resolution_evidence_digest) = 32),
    CONSTRAINT voice_lifecycle_redis_divergences_classified_fence_check CHECK (classified_lease_fence IS NULL OR classified_lease_fence >= 0),
    CONSTRAINT voice_lifecycle_redis_divergences_classification_shape_check CHECK (
        (classification = 'orphan' AND subject_profile_id IS NULL AND classified_pg_state IS NULL AND classified_lease_fence IS NULL)
        OR
        (classification <> 'orphan' AND subject_profile_id IS NOT NULL AND classified_pg_state IS NOT NULL AND classified_pg_state = classification AND classified_lease_fence IS NOT NULL)
    ),
    CONSTRAINT voice_lifecycle_redis_divergences_resolution_kind_check CHECK (resolution_kind IS NULL OR resolution_kind IN ('orphan_removed','mirror_restored_exact','mirror_reset_for_rebuild','expired_operation_retired')),
    CONSTRAINT voice_lifecycle_redis_divergences_resolution_shape_check CHECK (
        (resolution_id IS NULL AND resolution_kind IS NULL AND resolution_evidence_digest IS NULL AND resolved_by_account_id IS NULL AND resolved_at IS NULL)
        OR
        (resolution_id IS NOT NULL AND resolution_kind IS NOT NULL AND resolution_evidence_digest IS NOT NULL AND resolved_by_account_id IS NOT NULL AND resolved_at IS NOT NULL)
    ),
    CONSTRAINT voice_lifecycle_redis_divergences_time_order_check CHECK (last_observed_at >= first_observed_at AND updated_at >= created_at AND (resolved_at IS NULL OR resolved_at >= last_observed_at))
);

CREATE UNIQUE INDEX IF NOT EXISTS voice_lifecycle_redis_divergences_one_open_operation ON voice_lifecycle_redis_divergences(actor_profile_id, operation_id) WHERE resolved_at IS NULL;
CREATE INDEX IF NOT EXISTS voice_lifecycle_redis_divergences_open_queue ON voice_lifecycle_redis_divergences(first_observed_at, divergence_id) WHERE resolved_at IS NULL;
CREATE INDEX IF NOT EXISTS voice_lifecycle_redis_divergences_open_subject ON voice_lifecycle_redis_divergences(subject_profile_id, first_observed_at) WHERE resolved_at IS NULL AND subject_profile_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS voice_lifecycle_redis_divergences_retained_history ON voice_lifecycle_redis_divergences(resolved_at, divergence_id) WHERE resolved_at IS NOT NULL;

CREATE OR REPLACE FUNCTION voice_lifecycle_redis_divergence_update_guard_fn()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(OLD.divergence_id,OLD.actor_profile_id,OLD.operation_id,OLD.classification,OLD.redis_class,
           OLD.redis_schema_version,OLD.redis_state,OLD.observation_digest,OLD.subject_profile_id,
           OLD.classified_pg_state,OLD.classified_lease_fence,OLD.first_observed_at,OLD.created_at)
       IS DISTINCT FROM
       ROW(NEW.divergence_id,NEW.actor_profile_id,NEW.operation_id,NEW.classification,NEW.redis_class,
           NEW.redis_schema_version,NEW.redis_state,NEW.observation_digest,NEW.subject_profile_id,
           NEW.classified_pg_state,NEW.classified_lease_fence,NEW.first_observed_at,NEW.created_at) THEN
        RAISE EXCEPTION 'voice lifecycle redis divergence evidence is immutable';
    END IF;
    IF OLD.resolved_at IS NOT NULL THEN
        RAISE EXCEPTION 'resolved voice lifecycle redis divergence is immutable';
    END IF;
    IF NEW.last_observed_at < OLD.last_observed_at OR NEW.updated_at < OLD.updated_at THEN
        RAISE EXCEPTION 'voice lifecycle redis divergence timestamps cannot decrease';
    END IF;
    IF NEW.resolved_at IS NOT NULL AND NEW.last_observed_at IS DISTINCT FROM OLD.last_observed_at THEN
        RAISE EXCEPTION 'voice lifecycle redis divergence resolution cannot change observation time';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS voice_lifecycle_redis_divergence_update_guard ON voice_lifecycle_redis_divergences;
CREATE TRIGGER voice_lifecycle_redis_divergence_update_guard BEFORE UPDATE ON voice_lifecycle_redis_divergences
FOR EACH ROW EXECUTE FUNCTION voice_lifecycle_redis_divergence_update_guard_fn();

COMMIT;
