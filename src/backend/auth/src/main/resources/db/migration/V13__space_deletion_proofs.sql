-- Separate hash-only Space deletion proofs and immutable consume receipts.
CREATE TABLE space_deletion_proofs (
    operation_id UUID PRIMARY KEY,
    account_id UUID NULL,
    profile_id UUID NULL,
    session_epoch BIGINT NOT NULL CHECK (session_epoch > 0),
    space_id UUID NOT NULL,
    confirmation_name_sha256 BYTEA NOT NULL
        CHECK (octet_length(confirmation_name_sha256) = 32),
    proof_digest_sha256 BYTEA NOT NULL UNIQUE
        CHECK (octet_length(proof_digest_sha256) = 32),
    security_revision BIGINT NOT NULL CHECK (security_revision > 0),
    verified_factors TEXT NOT NULL
        CHECK (verified_factors IN ('password', 'password,totp', 'password,backup_code')),
    expires_at TIMESTAMPTZ NOT NULL,
    receipt_id UUID NOT NULL UNIQUE,
    consumed_at TIMESTAMPTZ NULL,
    binding_bytes BYTEA NULL,
    binding_sha256 BYTEA NOT NULL CHECK (octet_length(binding_sha256) = 32),
    receipt_bytes BYTEA NULL,
    receipt_sha256 BYTEA NULL CHECK (
        receipt_sha256 IS NULL OR octet_length(receipt_sha256) = 32),
    acknowledged_at TIMESTAMPTZ NULL,
    receipt_lookup_hmac BYTEA NULL CHECK (
        receipt_lookup_hmac IS NULL OR octet_length(receipt_lookup_hmac) = 32),
    receipt_hmac_key_version INTEGER NULL CHECK (
        receipt_hmac_key_version IS NULL OR receipt_hmac_key_version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((consumed_at IS NULL AND receipt_bytes IS NULL AND receipt_sha256 IS NULL)
        OR (consumed_at IS NOT NULL AND receipt_bytes IS NOT NULL AND receipt_sha256 IS NOT NULL)),
    CHECK (acknowledged_at IS NULL OR consumed_at IS NOT NULL),
    CHECK ((account_id IS NOT NULL AND profile_id IS NOT NULL AND binding_bytes IS NOT NULL
            AND receipt_lookup_hmac IS NULL AND receipt_hmac_key_version IS NULL)
        OR (account_id IS NULL AND profile_id IS NULL AND binding_bytes IS NULL
            AND receipt_lookup_hmac IS NOT NULL AND receipt_hmac_key_version IS NOT NULL))
);

CREATE INDEX space_deletion_proofs_account_idx
    ON space_deletion_proofs(account_id) WHERE account_id IS NOT NULL;
CREATE INDEX space_deletion_proofs_retention_idx
    ON space_deletion_proofs(acknowledged_at, consumed_at)
    WHERE acknowledged_at IS NOT NULL;
