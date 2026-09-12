CREATE TABLE file_blobs (
    blob_id UUID PRIMARY KEY,
    sha256 BYTEA NOT NULL,
    original_r2_key TEXT NOT NULL,
    converted_r2_key TEXT NULL,
    thumbnail_r2_key TEXT NULL,
    state TEXT NOT NULL CHECK (state IN ('LIVE', 'GC_PENDING', 'GC_COMPLETE')),
    gc_operation_id UUID NULL,
    gc_attempt INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NULL,
    deleted_at TIMESTAMPTZ NULL
);

ALTER TABLE files ADD COLUMN blob_id UUID NULL;
ALTER TABLE files DROP CONSTRAINT IF EXISTS files_r2_key_key;

INSERT INTO file_blobs (blob_id, sha256, original_r2_key, converted_r2_key, thumbnail_r2_key, state)
SELECT id,
       CASE WHEN sha256_hash ~ '^[0-9a-fA-F]{64}$' THEN decode(lower(sha256_hash), 'hex') ELSE decode(md5(id::text) || md5(id::text), 'hex') END,
       r2_key, converted_r2_key, thumbnail_r2_key, 'LIVE'
FROM files;

UPDATE files SET blob_id = id WHERE blob_id IS NULL;
ALTER TABLE files ALTER COLUMN blob_id SET NOT NULL;
ALTER TABLE files ADD CONSTRAINT files_blob_id_fk FOREIGN KEY (blob_id) REFERENCES file_blobs(blob_id);
CREATE INDEX files_blob_id_idx ON files(blob_id);

CREATE FUNCTION file_assign_blob_on_insert() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.blob_id IS NULL THEN
        NEW.blob_id := NEW.id;
        INSERT INTO file_blobs (blob_id, sha256, original_r2_key, converted_r2_key, thumbnail_r2_key, state)
        VALUES (
            NEW.id,
            CASE WHEN NEW.sha256_hash ~ '^[0-9a-fA-F]{64}$'
                 THEN decode(lower(NEW.sha256_hash), 'hex')
                 ELSE decode(repeat('00', 32), 'hex') END,
            NEW.r2_key,
            NEW.converted_r2_key,
            NEW.thumbnail_r2_key,
            'LIVE'
        );
    END IF;
    RETURN NEW;
END $$;

CREATE TRIGGER files_assign_blob_before_insert
BEFORE INSERT ON files
FOR EACH ROW EXECUTE FUNCTION file_assign_blob_on_insert();

CREATE FUNCTION file_sync_blob_after_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    UPDATE file_blobs
    SET sha256 = CASE WHEN NEW.sha256_hash ~ '^[0-9a-fA-F]{64}$'
                      THEN decode(lower(NEW.sha256_hash), 'hex')
                      ELSE sha256 END,
        original_r2_key = NEW.r2_key,
        converted_r2_key = NEW.converted_r2_key,
        thumbnail_r2_key = NEW.thumbnail_r2_key
    WHERE blob_id = NEW.blob_id;
    RETURN NEW;
END $$;

CREATE TRIGGER files_sync_blob_after_update
AFTER UPDATE OF sha256_hash, r2_key, converted_r2_key, thumbnail_r2_key ON files
FOR EACH ROW EXECUTE FUNCTION file_sync_blob_after_update();

CREATE TABLE file_references (
    file_id UUID NOT NULL REFERENCES files(id),
    owner_type INTEGER NOT NULL,
    owner_id UUID NOT NULL,
    subresource_id UUID NULL,
    scope_space_id UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    released_at TIMESTAMPTZ NULL,
    release_operation_id UUID NULL
);
CREATE UNIQUE INDEX file_references_live_key_uq
    ON file_references(file_id, owner_type, owner_id, COALESCE(subresource_id, '00000000-0000-0000-0000-000000000000'::uuid), COALESCE(scope_space_id, '00000000-0000-0000-0000-000000000000'::uuid))
    WHERE released_at IS NULL;
CREATE INDEX file_references_scope_live_idx ON file_references(scope_space_id) WHERE released_at IS NULL;

CREATE TABLE file_reference_operations (
    caller_service TEXT NOT NULL,
    operation_id UUID NOT NULL,
    request_bytes BYTEA NOT NULL,
    request_sha256 BYTEA NOT NULL,
    state TEXT NOT NULL,
    receipt_bytes BYTEA NULL,
    receipt_sha256 BYTEA NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    completed_at TIMESTAMPTZ NULL,
    PRIMARY KEY (caller_service, operation_id)
);

CREATE TABLE file_access_capabilities (
    capability_id UUID PRIMARY KEY,
    file_id UUID NOT NULL REFERENCES files(id),
    owner_type INTEGER NOT NULL,
    owner_id UUID NOT NULL,
    subresource_id UUID NULL,
    scope_space_id UUID NULL,
    subject_profile_id UUID NOT NULL,
    allowed_surfaces INTEGER[] NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    issue_operation_id UUID NOT NULL,
    request_bytes BYTEA NOT NULL,
    request_sha256 BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE UNIQUE INDEX file_access_capabilities_issue_uq ON file_access_capabilities(issue_operation_id);

CREATE TABLE file_space_lifecycle_fences (
    space_id UUID PRIMARY KEY,
    generation BIGINT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('LIVE', 'FROZEN', 'PURGE_DECIDED', 'PURGED')),
    deletion_operation_id UUID NULL,
    preliminary_chat_hash BYTEA NULL,
    final_manifest_hash BYTEA NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE file_space_deletion_manifests (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    schedule_generation BIGINT NOT NULL,
    producer_id INTEGER NOT NULL,
    expected_total_count BIGINT NOT NULL DEFAULT 0,
    expected_references_sha256 BYTEA NULL,
    received_chunk_count BIGINT NOT NULL DEFAULT 0,
    received_reference_count BIGINT NOT NULL DEFAULT 0,
    sealed_at TIMESTAMPTZ NULL,
    PRIMARY KEY (space_id, deletion_operation_id, schedule_generation, producer_id)
);

CREATE TABLE file_space_deletion_manifest_chunks (
    operation_id UUID PRIMARY KEY,
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    schedule_generation BIGINT NOT NULL,
    producer_id INTEGER NOT NULL,
    chunk_index BIGINT NOT NULL,
    request_bytes BYTEA NOT NULL,
    request_sha256 BYTEA NOT NULL,
    receipt_bytes BYTEA NOT NULL,
    references_bytes BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (space_id, deletion_operation_id, schedule_generation, producer_id, chunk_index)
);

CREATE TABLE file_space_deletion_producer_releases (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    purge_generation BIGINT NOT NULL,
    source_schedule_generation BIGINT NOT NULL,
    producer_id INTEGER NOT NULL,
    expected_references_sha256 BYTEA NOT NULL,
    request_bytes BYTEA NOT NULL,
    request_sha256 BYTEA NOT NULL,
    receipt_bytes BYTEA NOT NULL,
    released_count BIGINT NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (space_id, deletion_operation_id, purge_generation, producer_id)
);

CREATE TABLE file_space_purge_receipts (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL,
    request_bytes BYTEA NOT NULL,
    request_sha256 BYTEA NOT NULL,
    manifest_sha256 BYTEA NOT NULL,
    receipt_bytes BYTEA NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (space_id, deletion_operation_id, generation)
);
