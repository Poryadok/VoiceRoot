CREATE TABLE search_user_profile_fence (
    profile_id UUID PRIMARY KEY,
    source_revision BIGINT NOT NULL CHECK (source_revision >= 0),
    payload_sha256 BYTEA NOT NULL CHECK (octet_length(payload_sha256) = 32),
    tombstoned_at TIMESTAMPTZ NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
