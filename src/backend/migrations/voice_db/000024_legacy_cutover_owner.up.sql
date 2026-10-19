BEGIN;

CREATE TABLE voice_legacy_owner_works (
    work_id UUID PRIMARY KEY CHECK (work_id <> '00000000-0000-0000-0000-000000000000'),
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    work_bytes BYTEA NOT NULL CHECK (octet_length(work_bytes) > 0),
    work_sha256 BYTEA NOT NULL CHECK (octet_length(work_sha256) = 32),
    state TEXT NOT NULL DEFAULT 'PREPARED' CHECK (state IN ('PREPARED','ACCEPTED','COMPLETE')),
    receipt_bytes BYTEA,
    receipt_sha256 BYTEA,
    CHECK ((state = 'COMPLETE' AND octet_length(receipt_bytes) > 0 AND octet_length(receipt_sha256) = 32)
        OR (state <> 'COMPLETE' AND receipt_bytes IS NULL AND receipt_sha256 IS NULL))
);

CREATE TABLE voice_legacy_owner_acceptances (
    work_id UUID PRIMARY KEY REFERENCES voice_legacy_owner_works(work_id),
    acceptance_bytes BYTEA NOT NULL CHECK (octet_length(acceptance_bytes) > 0),
    acceptance_sha256 BYTEA NOT NULL CHECK (octet_length(acceptance_sha256) = 32)
);

CREATE TABLE voice_legacy_partitions (
    partition_id UUID PRIMARY KEY,
    work_id UUID NOT NULL UNIQUE REFERENCES voice_legacy_owner_works(work_id),
    canonical_bytes BYTEA NOT NULL CHECK (octet_length(canonical_bytes)>0),
    canonical_sha256 BYTEA NOT NULL CHECK (octet_length(canonical_sha256)=32)
);
CREATE TABLE voice_legacy_projection_steps (
    step_id UUID PRIMARY KEY,
    partition_id UUID NOT NULL REFERENCES voice_legacy_partitions(partition_id),
    resource_id TEXT NOT NULL CHECK (resource_id<>''),
    ordinal BIGINT NOT NULL CHECK (ordinal>0),
    linkage_bytes BYTEA NOT NULL,
    step_bytes BYTEA NOT NULL,
    step_sha256 BYTEA NOT NULL CHECK (octet_length(step_sha256)=32),
    receipt_bytes BYTEA,
    receipt_sha256 BYTEA,
    UNIQUE(partition_id,resource_id,ordinal),
    CHECK ((receipt_bytes IS NULL AND receipt_sha256 IS NULL)
      OR (octet_length(receipt_bytes)>0 AND octet_length(receipt_sha256)=32))
);

CREATE FUNCTION voice_legacy_projection_immutable_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' OR TG_TABLE_NAME='voice_legacy_partitions' THEN
        RAISE EXCEPTION 'legacy projection evidence is immutable';
    END IF;
    IF (NEW.step_id,NEW.partition_id,NEW.resource_id,NEW.ordinal,NEW.linkage_bytes,NEW.step_bytes,NEW.step_sha256)
       IS DISTINCT FROM (OLD.step_id,OLD.partition_id,OLD.resource_id,OLD.ordinal,OLD.linkage_bytes,OLD.step_bytes,OLD.step_sha256)
       OR OLD.receipt_bytes IS NOT NULL OR NEW.receipt_bytes IS NULL THEN
        RAISE EXCEPTION 'legacy projection intent/receipt is immutable';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER voice_legacy_partition_immutable BEFORE UPDATE OR DELETE ON voice_legacy_partitions
FOR EACH ROW EXECUTE FUNCTION voice_legacy_projection_immutable_fn();
CREATE TRIGGER voice_legacy_step_immutable BEFORE UPDATE OR DELETE ON voice_legacy_projection_steps
FOR EACH ROW EXECUTE FUNCTION voice_legacy_projection_immutable_fn();

CREATE FUNCTION voice_legacy_owner_immutable_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN RAISE EXCEPTION 'legacy owner evidence is retained'; END IF;
    IF TG_TABLE_NAME = 'voice_legacy_owner_acceptances' THEN RAISE EXCEPTION 'legacy acceptance is immutable'; END IF;
    IF (NEW.work_id, NEW.request_sha256, NEW.work_bytes, NEW.work_sha256)
        IS DISTINCT FROM (OLD.work_id, OLD.request_sha256, OLD.work_bytes, OLD.work_sha256)
        OR OLD.state = 'COMPLETE'
        OR NOT ((OLD.state = 'PREPARED' AND NEW.state = 'ACCEPTED') OR (OLD.state = 'ACCEPTED' AND NEW.state = 'COMPLETE'))
    THEN RAISE EXCEPTION 'legacy owner work is immutable'; END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER voice_legacy_owner_immutable BEFORE UPDATE OR DELETE ON voice_legacy_owner_works
FOR EACH ROW EXECUTE FUNCTION voice_legacy_owner_immutable_fn();
CREATE TRIGGER voice_legacy_acceptance_immutable BEFORE UPDATE OR DELETE ON voice_legacy_owner_acceptances
FOR EACH ROW EXECUTE FUNCTION voice_legacy_owner_immutable_fn();

REVOKE ALL ON voice_legacy_owner_works, voice_legacy_owner_acceptances FROM PUBLIC;
REVOKE ALL ON voice_legacy_partitions,voice_legacy_projection_steps FROM PUBLIC;
REVOKE ALL ON FUNCTION voice_legacy_owner_immutable_fn() FROM PUBLIC;
REVOKE ALL ON FUNCTION voice_legacy_projection_immutable_fn() FROM PUBLIC;
COMMIT;
