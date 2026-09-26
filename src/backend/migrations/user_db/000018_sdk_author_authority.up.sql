-- SDK target eligibility changes whenever a User-owned profile row changes.
-- A dedicated revision avoids overloading Search's projection-specific counter.
ALTER TABLE profiles
    ADD COLUMN sdk_eligibility_revision BIGINT NOT NULL DEFAULT 1
    CHECK (sdk_eligibility_revision > 0);

CREATE OR REPLACE FUNCTION bump_sdk_eligibility_revision()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.sdk_eligibility_revision := OLD.sdk_eligibility_revision + 1;
    RETURN NEW;
END
$$;

CREATE TRIGGER profiles_sdk_eligibility_revision
    BEFORE UPDATE ON profiles
    FOR EACH ROW EXECUTE FUNCTION bump_sdk_eligibility_revision();

-- An opaque Auth SDK actor may have no User profile. This table preserves the
-- historical actor alias without changing authors or granting target history.
CREATE TABLE sdk_author_tombstones (
    operation_id UUID PRIMARY KEY,
    receipt_id UUID NOT NULL UNIQUE,
    source_account_id UUID NOT NULL,
    source_actor_id UUID NOT NULL,
    target_account_id UUID NOT NULL,
    target_profile_id UUID NOT NULL,
    profile_revision BIGINT NOT NULL CHECK (profile_revision > 0),
    tombstone_revision BIGINT GENERATED ALWAYS AS IDENTITY UNIQUE,
    frozen_binding_id UUID NOT NULL,
    frozen_authority_epoch BIGINT NOT NULL CHECK (frozen_authority_epoch > 0),
    freeze_receipt_id UUID NOT NULL,
    request_hash CHAR(64) NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    committed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_account_id, source_actor_id)
);

CREATE OR REPLACE FUNCTION reject_sdk_author_tombstone_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'sdk author tombstone receipts are immutable'
        USING ERRCODE = '55000';
END
$$;

CREATE TRIGGER sdk_author_tombstones_no_update_delete
    BEFORE UPDATE OR DELETE ON sdk_author_tombstones
    FOR EACH ROW EXECUTE FUNCTION reject_sdk_author_tombstone_mutation();

CREATE TRIGGER sdk_author_tombstones_no_truncate
    BEFORE TRUNCATE ON sdk_author_tombstones
    FOR EACH STATEMENT EXECUTE FUNCTION reject_sdk_author_tombstone_mutation();
