BEGIN;
ALTER TABLE story_media_reference_intents ADD COLUMN author_profile_id UUID NULL;
ALTER TABLE story_media_reference_intents ADD COLUMN media_type TEXT NULL;
CREATE TABLE story_reference_inventory_gate (
    singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
    owner_incarnation_id UUID NOT NULL DEFAULT gen_random_uuid(),
    revision BIGINT NOT NULL DEFAULT 0 CHECK (revision >= 0)
);
INSERT INTO story_reference_inventory_gate(singleton) VALUES(true);
CREATE TABLE story_reference_inventory_snapshots (
    snapshot_id UUID PRIMARY KEY,
    operation_id UUID NOT NULL UNIQUE,
    identity_bytes BYTEA NOT NULL,
    request_bytes BYTEA NOT NULL,
    descriptor_bytes BYTEA NOT NULL,
    boundary_revision BIGINT NOT NULL CHECK(boundary_revision >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE story_reference_inventory_pages (
    snapshot_id UUID NOT NULL REFERENCES story_reference_inventory_snapshots(snapshot_id),
    page_index INTEGER NOT NULL CHECK(page_index >= 0),
    page_bytes BYTEA NOT NULL,
    PRIMARY KEY(snapshot_id,page_index)
);
CREATE TABLE story_reference_inventory_journal (
    revision BIGINT PRIMARY KEY CHECK(revision > 0),
    revision_bytes BYTEA NOT NULL
);
CREATE TABLE story_reference_inventory_state (
    item_key TEXT PRIMARY KEY,
    owner_id UUID NULL,
    is_cover BOOLEAN NOT NULL,
    entry_bytes BYTEA NOT NULL
);
CREATE INDEX story_reference_inventory_state_owner ON story_reference_inventory_state(owner_id);
CREATE TABLE story_reference_inventory_retirements (
    story_id UUID NOT NULL,
    media_file_id UUID NOT NULL,
    operation_id UUID NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(story_id,media_file_id)
);
COMMIT;
