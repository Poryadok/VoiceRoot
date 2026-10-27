ALTER TABLE files ADD COLUMN catalog_asset BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE files ADD COLUMN catalog_source_size_unknown BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE files DROP CONSTRAINT files_size_bytes_check;
ALTER TABLE files ADD CONSTRAINT files_size_bytes_check CHECK (
    (size_bytes > 0 AND size_bytes <= 209715200 AND NOT catalog_source_size_unknown)
    OR (size_bytes = 0 AND catalog_asset AND catalog_source_size_unknown
        AND status = 'pending_upload' AND upload_intent = 'gif'
        AND NOT is_e2e AND sha256_hash IS NULL)
);
ALTER TABLE files ADD CONSTRAINT files_catalog_plaintext CHECK (
    NOT catalog_asset OR (NOT is_e2e AND upload_intent = 'gif' AND chat_id IS NULL AND story_id IS NULL)
);

-- Permanent actor gates contain purpose HMACs and opaque generations, not raw
-- account/profile identities. Keys remain private service-owned lifecycle data.
CREATE TABLE file_catalog_actor_gates (
    id UUID PRIMARY KEY,
    actor_kind TEXT NOT NULL CHECK (actor_kind IN ('account','profile')),
    key_version TEXT NOT NULL CHECK (octet_length(key_version) BETWEEN 1 AND 127),
    actor_hmac BYTEA NOT NULL CHECK (octet_length(actor_hmac) = 32),
    generation UUID NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('LIVE','CLOSED')),
    UNIQUE (actor_kind,key_version,actor_hmac)
);

-- Active source mappings are erased on actor/asset purge. The File marker
-- survives mapping erasure so an old catalog File can never become ordinary.
CREATE TABLE file_catalog_assets (
    file_id UUID PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
    source_id UUID NOT NULL,
    actor_account_id UUID NOT NULL,
    actor_profile_id UUID NOT NULL,
    provider TEXT NOT NULL CHECK (provider <> ''),
    provider_id TEXT NOT NULL CHECK (provider_id <> ''),
    account_gate_id UUID NOT NULL REFERENCES file_catalog_actor_gates(id),
    profile_gate_id UUID NOT NULL REFERENCES file_catalog_actor_gates(id),
    account_generation UUID NOT NULL,
    profile_generation UUID NOT NULL,
    chat_account_generation UUID NOT NULL,
    chat_profile_generation UUID NOT NULL,
    source_url_sha256 BYTEA NULL CHECK (source_url_sha256 IS NULL OR octet_length(source_url_sha256) = 32),
    source_sha256 BYTEA NULL CHECK (source_sha256 IS NULL OR octet_length(source_sha256) = 32),
    UNIQUE (actor_profile_id,source_id),
    CHECK (account_gate_id <> profile_gate_id)
);
CREATE INDEX file_catalog_assets_actor ON file_catalog_assets(actor_account_id,actor_profile_id);

-- The lookup key is purpose-HMAC(account/profile/operation); full request bytes
-- and source URLs are never stored. Completed details are compacted at P30D or
-- earlier privacy purge; terminal fences below prohibit missing-record replay.
CREATE TABLE file_catalog_import_operations (
    key_version TEXT NOT NULL,
    operation_hmac BYTEA NOT NULL CHECK (octet_length(operation_hmac) = 32),
    generation UUID NOT NULL,
    phase TEXT NOT NULL CHECK (phase IN ('PENDING','FETCHED','WRITE_STARTED','RECONCILIATION','PROCESSING','COMPLETE')),
    operation_id UUID NOT NULL,
    -- The retained receipt survives a no-bytes failed File's removal. This is
    -- an immutable response identity, never proof that a live File exists.
    file_id UUID NOT NULL,
    source_id UUID NOT NULL,
    provider TEXT NOT NULL CHECK (provider <> ''),
    provider_id TEXT NOT NULL CHECK (provider_id <> ''),
    chat_account_generation UUID NOT NULL,
    chat_profile_generation UUID NOT NULL,
    actor_account_id UUID NOT NULL,
    actor_profile_id UUID NOT NULL,
    destination_chat_id UUID NOT NULL,
    destination_chat_type TEXT NOT NULL CHECK (destination_chat_type IN ('dm','group','channel')),
    requested_chat_type INTEGER NULL CHECK (requested_chat_type BETWEEN 0 AND 3),
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    selection_sha256 BYTEA NOT NULL CHECK (octet_length(selection_sha256) = 32),
    max_fetch_bytes BIGINT NOT NULL CHECK (max_fetch_bytes > 0 AND max_fetch_bytes <= 52428800),
    completed_at TIMESTAMPTZ NULL,
    retain_until TIMESTAMPTZ NULL,
    outcome TEXT NULL CHECK (outcome IN ('ready','failed','deleted','expired')),
    PRIMARY KEY (key_version,operation_hmac),
    UNIQUE (actor_account_id,actor_profile_id,operation_id),
    CHECK ((completed_at IS NULL AND retain_until IS NULL AND outcome IS NULL AND phase <> 'COMPLETE')
        OR (completed_at IS NOT NULL AND retain_until = completed_at + interval '30 days' AND outcome IS NOT NULL AND phase = 'COMPLETE'))
);
CREATE INDEX file_catalog_import_operations_actor ON file_catalog_import_operations(actor_account_id,actor_profile_id);

CREATE TABLE file_catalog_terminal_fences (
    purpose TEXT NOT NULL CHECK (purpose IN ('operation','source','asset')),
    key_version TEXT NOT NULL,
    identity_hmac BYTEA NOT NULL CHECK (octet_length(identity_hmac) = 32),
    outcome TEXT NOT NULL CHECK (outcome IN ('ready','failed','deleted','expired','not_available')),
    fence_id UUID NOT NULL UNIQUE,
    PRIMARY KEY (purpose,key_version,identity_hmac)
);

-- Declared before any PUT. A STARTED/UNSETTLED write cannot be collected merely
-- because a timer/session expired: a runtime-owned drain or storage fence is
-- required. Unique attempt keys prevent an old write replacing a new source.
CREATE TABLE file_catalog_writes (
    object_key TEXT PRIMARY KEY,
    file_id UUID NOT NULL REFERENCES files(id),
    generation UUID NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('DECLARED','STARTED','SETTLED','UNSETTLED','GC_PENDING','DONE')),
    expected_sha256 BYTEA NOT NULL CHECK (octet_length(expected_sha256) = 32),
    expected_size_bytes BIGINT NOT NULL CHECK (expected_size_bytes > 0 AND expected_size_bytes <= 52428800)
);
CREATE INDEX file_catalog_writes_file_state ON file_catalog_writes(file_id,state);

-- Existing reference/expiry cleanup must not collect a source that can still
-- receive a late PUT. Keep reference release independent: the reference can be
-- released while physical cleanup remains blocked by actual unsettled work.
CREATE FUNCTION file_catalog_guard_blob_gc() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.state IN ('GC_PENDING','GC_COMPLETE') AND EXISTS (
        SELECT 1 FROM files f JOIN file_catalog_writes w ON w.file_id=f.id
        WHERE f.blob_id=NEW.blob_id AND w.state IN ('STARTED','UNSETTLED')
    ) THEN
        NEW.state := OLD.state;
        NEW.gc_operation_id := OLD.gc_operation_id;
        NEW.gc_attempt := OLD.gc_attempt;
        NEW.next_attempt_at := OLD.next_attempt_at;
        NEW.deleted_at := OLD.deleted_at;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER file_catalog_guard_blob_gc BEFORE UPDATE ON file_blobs
    FOR EACH ROW EXECUTE FUNCTION file_catalog_guard_blob_gc();

CREATE FUNCTION file_catalog_immutable_source() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.file_id,NEW.source_id,NEW.actor_account_id,NEW.actor_profile_id,NEW.provider,NEW.provider_id,
           NEW.account_gate_id,NEW.profile_gate_id,NEW.account_generation,NEW.profile_generation,
           NEW.chat_account_generation,NEW.chat_profile_generation)
       IS DISTINCT FROM
       ROW(OLD.file_id,OLD.source_id,OLD.actor_account_id,OLD.actor_profile_id,OLD.provider,OLD.provider_id,
           OLD.account_gate_id,OLD.profile_gate_id,OLD.account_generation,OLD.profile_generation,
           OLD.chat_account_generation,OLD.chat_profile_generation)
       OR (OLD.source_sha256 IS NOT NULL AND NEW.source_sha256 IS DISTINCT FROM OLD.source_sha256)
       OR (OLD.source_url_sha256 IS NOT NULL AND NEW.source_url_sha256 IS NOT NULL
           AND NEW.source_url_sha256 IS DISTINCT FROM OLD.source_url_sha256) THEN
        RAISE EXCEPTION 'catalog source identity is immutable';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER file_catalog_immutable_source BEFORE UPDATE ON file_catalog_assets
    FOR EACH ROW EXECUTE FUNCTION file_catalog_immutable_source();

CREATE FUNCTION file_catalog_guard_file() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'UPDATE' AND OLD.catalog_asset THEN
        IF NOT NEW.catalog_asset OR NEW.uploader_profile_id IS DISTINCT FROM OLD.uploader_profile_id
           OR (OLD.sha256_hash IS NOT NULL AND NEW.sha256_hash IS DISTINCT FROM OLD.sha256_hash) THEN
            RAISE EXCEPTION 'catalog File identity is immutable';
        END IF;
    END IF;
    IF NEW.catalog_asset AND NEW.status = 'ready' THEN
        PERFORM 1 FROM file_catalog_assets a
        JOIN file_catalog_actor_gates account_gate ON account_gate.id=a.account_gate_id
        JOIN file_catalog_actor_gates profile_gate ON profile_gate.id=a.profile_gate_id
        WHERE a.file_id=NEW.id AND account_gate.actor_kind='account' AND profile_gate.actor_kind='profile'
          AND account_gate.state='LIVE' AND profile_gate.state='LIVE'
          AND account_gate.generation=a.account_generation AND profile_gate.generation=a.profile_generation
          AND a.source_sha256=decode(NEW.sha256_hash,'hex') AND NEW.size_bytes > 0
          AND NOT NEW.catalog_source_size_unknown
        FOR SHARE OF account_gate,profile_gate;
        IF NOT FOUND THEN RAISE EXCEPTION 'catalog actor/source is not live'; END IF;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER file_catalog_guard_file BEFORE INSERT OR UPDATE ON files
    FOR EACH ROW EXECUTE FUNCTION file_catalog_guard_file();

-- Receipt retention starts at the actual File outcome, never a later poll or
-- retry. Later metadata/status updates cannot refresh the original deadline.
CREATE FUNCTION file_catalog_complete_operation() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE completed TIMESTAMPTZ;
BEGIN
    IF NEW.catalog_asset AND NEW.status IN ('ready','failed','deleted','expired')
       AND NEW.status IS DISTINCT FROM OLD.status THEN
        completed := clock_timestamp();
        UPDATE file_catalog_import_operations SET phase='COMPLETE',outcome=NEW.status,
            completed_at=completed,retain_until=completed+interval '30 days'
        WHERE file_id=NEW.id AND completed_at IS NULL;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER file_catalog_complete_operation AFTER UPDATE ON files
    FOR EACH ROW EXECUTE FUNCTION file_catalog_complete_operation();
