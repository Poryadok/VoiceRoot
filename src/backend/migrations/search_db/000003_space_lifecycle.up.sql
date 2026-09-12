-- R23 Search participant lifecycle evidence and deletion fence. Full request
-- and receipt evidence is retained for 30 days after application/completion.
CREATE TABLE search_space_lifecycle_fences (
    space_id UUID PRIMARY KEY,
    generation BIGINT NOT NULL CHECK (generation > 0),
    state TEXT NOT NULL CHECK (state IN ('LIVE', 'FROZEN', 'PURGE_DECIDED', 'PURGED')),
    deletion_operation_id UUID NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE search_space_lifecycle_operations (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    request_bytes BYTEA NOT NULL,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    state TEXT NOT NULL CHECK (state IN ('LIVE', 'FROZEN', 'PURGE_DECIDED')),
    created_at TIMESTAMPTZ NOT NULL,
    retain_until TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (space_id, deletion_operation_id, generation)
);

CREATE TABLE search_space_lifecycle_receipts (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    receipt_bytes BYTEA NOT NULL,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    applied_at TIMESTAMPTZ NOT NULL,
    retain_until TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (space_id, deletion_operation_id, generation)
);

CREATE TABLE search_space_purge_receipts (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    request_bytes BYTEA NOT NULL,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    receipt_bytes BYTEA NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL,
    retain_until TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (space_id, deletion_operation_id, generation)
);

CREATE TABLE search_space_chat_manifests (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    manifest_id TEXT NOT NULL CHECK (manifest_id <> ''),
    manifest_sha256 BYTEA NOT NULL CHECK (octet_length(manifest_sha256) = 32),
    item_count BIGINT NOT NULL CHECK (item_count > 0),
    page_count BIGINT NOT NULL CHECK (page_count > 0),
    sealed BOOLEAN NOT NULL CHECK (sealed),
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (space_id, deletion_operation_id),
    UNIQUE (manifest_id)
);

-- Exact deterministic Chat pages are retained durably until purge evidence
-- expires; the item relation drives cleanup and permanent anti-resurrection.
CREATE TABLE search_space_chat_manifest_pages (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    page_index BIGINT NOT NULL CHECK (page_index >= 0),
    page_bytes BYTEA NOT NULL,
    page_sha256 BYTEA NOT NULL CHECK (octet_length(page_sha256) = 32),
    item_count BIGINT NOT NULL CHECK (item_count BETWEEN 0 AND 1000),
    next_page_token TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (space_id, deletion_operation_id, generation, page_index)
);
CREATE TABLE search_space_chat_manifest_items (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    page_index BIGINT NOT NULL CHECK (page_index >= 0),
    item_index BIGINT NOT NULL CHECK (item_index >= 0),
    chat_id UUID NOT NULL,
    PRIMARY KEY (space_id, deletion_operation_id, generation, page_index, item_index),
    UNIQUE (space_id, deletion_operation_id, generation, chat_id)
);
CREATE INDEX search_space_chat_manifest_items_chat_idx ON search_space_chat_manifest_items (chat_id);

-- Compact terminal mappings survive projection cleanup and prevent a late event
-- from recreating a purged Chat index.
CREATE TABLE search_space_purged_chat_fences (
    space_id UUID NOT NULL,
    chat_id UUID NOT NULL,
    PRIMARY KEY (space_id, chat_id)
);

CREATE OR REPLACE FUNCTION search_reject_frozen_projection() RETURNS trigger AS $$
BEGIN
    IF TG_TABLE_NAME = 'space_search_documents' AND EXISTS (
        SELECT 1 FROM search_space_lifecycle_fences f WHERE f.space_id=(to_jsonb(NEW)->>'space_id')::uuid AND f.state IN ('FROZEN','PURGE_DECIDED','PURGED')
    ) THEN
        RAISE EXCEPTION 'search projection lifecycle fence is closed';
    END IF;
    IF TG_TABLE_NAME IN ('message_search_documents','chat_search_documents') AND EXISTS (
        SELECT 1 FROM search_space_chat_manifest_items p JOIN search_space_lifecycle_fences f ON f.space_id=p.space_id AND f.deletion_operation_id=p.deletion_operation_id
        WHERE p.chat_id=(to_jsonb(NEW)->>'chat_id')::uuid AND f.state IN ('FROZEN','PURGE_DECIDED','PURGED')
        UNION ALL SELECT 1 FROM search_space_purged_chat_fences q WHERE q.chat_id=(to_jsonb(NEW)->>'chat_id')::uuid
    ) THEN
        RAISE EXCEPTION 'search projection lifecycle fence is closed';
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;
CREATE TRIGGER search_message_lifecycle_gate BEFORE INSERT OR UPDATE ON message_search_documents FOR EACH ROW EXECUTE FUNCTION search_reject_frozen_projection();
CREATE TRIGGER search_chat_lifecycle_gate BEFORE INSERT OR UPDATE ON chat_search_documents FOR EACH ROW EXECUTE FUNCTION search_reject_frozen_projection();
CREATE TRIGGER search_space_lifecycle_gate BEFORE INSERT OR UPDATE ON space_search_documents FOR EACH ROW EXECUTE FUNCTION search_reject_frozen_projection();

-- The purge transaction deletes message_search_documents, chat_search_documents
-- and space_search_documents only after the durable PURGE_DECIDED fence.
