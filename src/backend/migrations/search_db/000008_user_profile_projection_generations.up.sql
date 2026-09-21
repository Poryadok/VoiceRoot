-- Additive shadow storage keeps every #428 table, primary key and conflict
-- target intact for old binaries during a rolling deployment.
CREATE TABLE search_user_profile_generations (
    generation BIGINT PRIMARY KEY CHECK (generation > 0), state TEXT NOT NULL CHECK (state IN ('building', 'ready')),
    high_watermark BIGINT NOT NULL DEFAULT 0 CHECK (high_watermark >= 0), journal_cutoff BIGINT NOT NULL DEFAULT 0 CHECK (journal_cutoff >= 0),
    evidence_sha256 BYTEA NULL CHECK (evidence_sha256 IS NULL OR octet_length(evidence_sha256) = 32), created_at TIMESTAMPTZ NOT NULL DEFAULT now(), ready_at TIMESTAMPTZ NULL
);
INSERT INTO search_user_profile_generations(generation,state,ready_at) VALUES (1,'ready',now()) ON CONFLICT DO NOTHING;
CREATE TABLE search_user_profile_generation_route (
    singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton), active_generation BIGINT NOT NULL REFERENCES search_user_profile_generations(generation),
    rollback_generation BIGINT NULL REFERENCES search_user_profile_generations(generation), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO search_user_profile_generation_route(singleton,active_generation) VALUES (true,1) ON CONFLICT DO NOTHING;
CREATE TABLE search_user_profile_generation_documents (
    generation BIGINT NOT NULL REFERENCES search_user_profile_generations(generation), profile_id UUID NOT NULL, account_id UUID NOT NULL, username TEXT NOT NULL,
    discriminator CHAR(4) NOT NULL, display_name TEXT NOT NULL, username_lower TEXT NOT NULL, verification_type TEXT NOT NULL DEFAULT 'none',
    username_search_key TEXT NULL, display_name_search_key TEXT NULL, normalization_version INTEGER NULL, source_revision BIGINT NOT NULL DEFAULT 0,
    tombstoned_at TIMESTAMPTZ NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY (generation,profile_id)
);
CREATE TABLE search_user_profile_generation_inbox (
    generation BIGINT NOT NULL REFERENCES search_user_profile_generations(generation), event_id UUID NOT NULL, profile_id UUID NOT NULL,
    source_revision BIGINT NOT NULL CHECK (source_revision > 0), payload_sha256 BYTEA NOT NULL CHECK (octet_length(payload_sha256) = 32),
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(), quarantined_at TIMESTAMPTZ NULL, quarantine_reason TEXT NULL, PRIMARY KEY (generation,event_id)
);
CREATE UNIQUE INDEX search_user_profile_generation_inbox_active_revision_idx ON search_user_profile_generation_inbox(generation,profile_id,source_revision) WHERE quarantined_at IS NULL;
CREATE TABLE search_user_profile_generation_fence (
    generation BIGINT NOT NULL REFERENCES search_user_profile_generations(generation), profile_id UUID NOT NULL, source_revision BIGINT NOT NULL CHECK (source_revision >= 0),
    payload_sha256 BYTEA NOT NULL CHECK (octet_length(payload_sha256)=32), tombstoned_at TIMESTAMPTZ NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY(generation,profile_id)
);
CREATE TABLE search_user_profile_generation_checkpoint (
    generation BIGINT PRIMARY KEY REFERENCES search_user_profile_generations(generation), journal_offset BIGINT NOT NULL DEFAULT 0 CHECK(journal_offset>=0),
    snapshot_phase TEXT NOT NULL DEFAULT 'idle' CHECK(snapshot_phase IN ('idle','snapshot','replay')), snapshot_high_watermark BIGINT NOT NULL DEFAULT 0 CHECK(snapshot_high_watermark>=0),
    snapshot_cursor TEXT NOT NULL DEFAULT '', evidence_count BIGINT NOT NULL DEFAULT 0 CHECK(evidence_count>=0), evidence_first_offset BIGINT NOT NULL DEFAULT 0,
    evidence_last_offset BIGINT NOT NULL DEFAULT 0, evidence_digest BYTEA NULL CHECK(evidence_digest IS NULL OR octet_length(evidence_digest)=32), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO search_user_profile_generation_documents SELECT 1,profile_id,account_id,username,discriminator,display_name,username_lower,verification_type,username_search_key,display_name_search_key,normalization_version,source_revision,tombstoned_at,updated_at FROM profile_search_documents;
INSERT INTO search_user_profile_generation_inbox SELECT 1,event_id,profile_id,source_revision,payload_sha256,received_at,quarantined_at,quarantine_reason FROM search_user_profile_inbox;
INSERT INTO search_user_profile_generation_fence SELECT 1,profile_id,source_revision,payload_sha256,tombstoned_at,updated_at FROM search_user_profile_fence;
INSERT INTO search_user_profile_generation_checkpoint SELECT 1,journal_offset,snapshot_phase,snapshot_high_watermark,snapshot_cursor,0,0,0,NULL,updated_at FROM search_user_profile_checkpoint;
