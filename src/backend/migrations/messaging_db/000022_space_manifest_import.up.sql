CREATE TABLE messaging_space_chat_manifests (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    schedule_generation BIGINT NOT NULL CHECK (schedule_generation > 0),
    manifest_id TEXT NOT NULL CHECK (manifest_id <> ''),
    manifest_sha256 BYTEA NOT NULL CHECK (octet_length(manifest_sha256) = 32),
    item_count BIGINT NOT NULL CHECK (item_count >= 0),
    page_count BIGINT NOT NULL CHECK (page_count > 0),
    sealed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    sealed_at TIMESTAMPTZ NULL,
    PRIMARY KEY (space_id, deletion_operation_id),
    UNIQUE (manifest_id),
    CHECK (page_count = GREATEST(1, (item_count + 999) / 1000)),
    CHECK ((sealed = FALSE AND sealed_at IS NULL) OR (sealed = TRUE AND sealed_at IS NOT NULL))
);

CREATE TABLE messaging_space_chat_manifest_pages (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    schedule_generation BIGINT NOT NULL CHECK (schedule_generation > 0),
    page_index BIGINT NOT NULL CHECK (page_index >= 0),
    page_bytes BYTEA NOT NULL,
    page_sha256 BYTEA NOT NULL CHECK (octet_length(page_sha256) = 32),
    item_count BIGINT NOT NULL CHECK (item_count BETWEEN 0 AND 1000),
    next_page_token TEXT NOT NULL,
    request_bytes BYTEA NOT NULL,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    receipt_bytes BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (space_id, deletion_operation_id, schedule_generation, page_index),
    FOREIGN KEY (space_id, deletion_operation_id)
        REFERENCES messaging_space_chat_manifests(space_id, deletion_operation_id)
);

CREATE TABLE messaging_space_chat_manifest_items (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    schedule_generation BIGINT NOT NULL CHECK (schedule_generation > 0),
    page_index BIGINT NOT NULL CHECK (page_index >= 0),
    item_index INTEGER NOT NULL CHECK (item_index BETWEEN 0 AND 999),
    chat_id UUID NOT NULL,
    PRIMARY KEY (space_id, deletion_operation_id, schedule_generation, page_index, item_index),
    UNIQUE (space_id, deletion_operation_id, schedule_generation, chat_id),
    FOREIGN KEY (space_id, deletion_operation_id, schedule_generation, page_index)
        REFERENCES messaging_space_chat_manifest_pages(space_id, deletion_operation_id, schedule_generation, page_index)
);

CREATE INDEX messaging_space_chat_manifest_items_chat_idx
    ON messaging_space_chat_manifest_items(chat_id);
