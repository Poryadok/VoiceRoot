-- Expand-only durable inbox and revision fence for User-authoritative profiles.
ALTER TABLE profile_search_documents
    ADD COLUMN IF NOT EXISTS username_search_key TEXT NULL,
    ADD COLUMN IF NOT EXISTS display_name_search_key TEXT NULL,
    ADD COLUMN IF NOT EXISTS normalization_version INTEGER NULL,
    ADD COLUMN IF NOT EXISTS source_revision BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tombstoned_at TIMESTAMPTZ NULL;

CREATE TABLE search_user_profile_inbox (
    event_id UUID PRIMARY KEY,
    profile_id UUID NOT NULL,
    source_revision BIGINT NOT NULL CHECK (source_revision > 0),
    payload_sha256 BYTEA NOT NULL CHECK (octet_length(payload_sha256) = 32),
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    quarantined_at TIMESTAMPTZ NULL,
    quarantine_reason TEXT NULL
);
CREATE UNIQUE INDEX search_user_profile_inbox_profile_revision_idx
    ON search_user_profile_inbox (profile_id, source_revision);

CREATE TABLE search_user_profile_checkpoint (
    singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
    journal_offset BIGINT NOT NULL DEFAULT 0 CHECK (journal_offset >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO search_user_profile_checkpoint(singleton) VALUES (true) ON CONFLICT DO NOTHING;
