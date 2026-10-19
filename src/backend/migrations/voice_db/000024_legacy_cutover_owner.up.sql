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
REVOKE ALL ON FUNCTION voice_legacy_owner_immutable_fn() FROM PUBLIC;
COMMIT;
