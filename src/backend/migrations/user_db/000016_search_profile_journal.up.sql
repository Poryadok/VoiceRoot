-- User is the historical authority for Search's profile projection.
-- The revision is incremented in the same transaction as every projected
-- mutation, giving Search a per-profile monotonic ordering key.
ALTER TABLE profiles
    ADD COLUMN IF NOT EXISTS search_projection_revision BIGINT NOT NULL DEFAULT 0;
UPDATE profiles
    SET search_projection_revision = 1
    WHERE search_projection_revision = 0;

CREATE TABLE user_profile_search_journal (
    journal_offset BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id UUID NOT NULL UNIQUE,
    profile_id UUID NOT NULL,
    source_revision BIGINT NOT NULL CHECK (source_revision > 0),
    protocol_version INTEGER NOT NULL CHECK (protocol_version = 1),
    payload BYTEA NOT NULL,
    payload_sha256 BYTEA NOT NULL CHECK (octet_length(payload_sha256) = 32),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (profile_id, source_revision)
);

CREATE TABLE user_profile_search_outbox (
    event_id UUID PRIMARY KEY REFERENCES user_profile_search_journal(event_id),
    journal_offset BIGINT NOT NULL UNIQUE REFERENCES user_profile_search_journal(journal_offset),
    payload BYTEA NOT NULL,
    leased_until TIMESTAMPTZ NULL,
    lease_owner TEXT NULL,
    delivered_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((leased_until IS NULL) = (lease_owner IS NULL))
);
CREATE INDEX user_profile_search_outbox_ready_idx ON user_profile_search_outbox (journal_offset)
    WHERE delivered_at IS NULL;
