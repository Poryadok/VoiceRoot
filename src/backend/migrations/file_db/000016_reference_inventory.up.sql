BEGIN;

-- No ACTIVE state or production activation provider is part of this migration.
CREATE TABLE file_reference_inventory_gate (
 singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
 blocked boolean NOT NULL DEFAULT false,
 generation bigint NOT NULL DEFAULT 1 CHECK(generation > 0)
);
INSERT INTO file_reference_inventory_gate(singleton) VALUES(true);

CREATE TABLE file_reference_inventory_cohorts (
 migration_id uuid PRIMARY KEY,
 installation_id uuid NOT NULL,
 restore_cohort_id uuid NOT NULL,
 file_incarnation_id uuid NOT NULL,
 state text NOT NULL CHECK(state IN ('MIGRATING','INVALIDATED')),
 registry_sha256 bytea NOT NULL CHECK(octet_length(registry_sha256)=32),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE file_reference_inventory_components (
 migration_id uuid NOT NULL REFERENCES file_reference_inventory_cohorts,
 component text NOT NULL,
 state text NOT NULL DEFAULT 'MISSING' CHECK(state IN ('MISSING','SNAPSHOT','CATCHING_UP','BLOCKED')),
 PRIMARY KEY(migration_id,component)
);
CREATE TABLE file_reference_inventory_imports (
 operation_id uuid PRIMARY KEY,
 migration_id uuid NOT NULL REFERENCES file_reference_inventory_cohorts,
 snapshot_id uuid NOT NULL UNIQUE,
 identity_bytes bytea NOT NULL,
 descriptor_bytes bytea,
 next_page bigint NOT NULL DEFAULT 0 CHECK(next_page>=0),
 cursor_revision bigint NOT NULL DEFAULT 0 CHECK(cursor_revision>=0),
 item_count bigint NOT NULL DEFAULT 0 CHECK(item_count>=0),
 source_unresolved_count bigint NOT NULL DEFAULT 0 CHECK(source_unresolved_count>=0),
 last_entry_bytes bytea,
 state text NOT NULL DEFAULT 'SNAPSHOT' CHECK(state IN ('SNAPSHOT','CATCHING_UP','BLOCKED')),
 blocked_reason text,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE file_reference_inventory_receipts (
 operation_id uuid NOT NULL REFERENCES file_reference_inventory_imports,
 kind text NOT NULL CHECK(kind IN ('PAGE','REVISION')),
 position bigint NOT NULL CHECK(position>=0),
 payload_bytes bytea NOT NULL,
 PRIMARY KEY(operation_id,kind,position)
);
CREATE TABLE file_reference_inventory_items (
 operation_id uuid NOT NULL REFERENCES file_reference_inventory_imports,
 item_key text NOT NULL,
 file_id uuid,
 owner_id uuid,
 owner_revision bigint NOT NULL CHECK(owner_revision>=0),
 retired_floor bigint NOT NULL DEFAULT 0 CHECK(retired_floor>=0),
 entry_bytes bytea NOT NULL,
 issue_reason text,
 PRIMARY KEY(operation_id,item_key)
);
CREATE INDEX file_reference_inventory_items_owner ON file_reference_inventory_items(operation_id,owner_id);
-- No FK to files: an issue may name an absent or terminal File. Holds survive
-- alias changes and never become readable file_references/access capabilities.
CREATE TABLE file_reference_inventory_holds (
 operation_id uuid NOT NULL REFERENCES file_reference_inventory_imports,
 item_key text NOT NULL,
 blob_id uuid NOT NULL REFERENCES file_blobs,
 PRIMARY KEY(operation_id,item_key,blob_id)
);
CREATE INDEX file_reference_inventory_holds_blob ON file_reference_inventory_holds(blob_id);

-- Per-key storage attempts survive local transaction/connection loss. Legacy
-- object keys are not claimed to be storage-side generations. No clock clears
-- STARTED/UNSETTLED, and a new cohort cannot claim their remote work drained.
CREATE TABLE file_reference_inventory_deletes (
 attempt_id uuid PRIMARY KEY,
 object_key text NOT NULL CHECK(object_key<>''),
 guard_generation bigint NOT NULL CHECK(guard_generation>0),
 state text NOT NULL CHECK(state IN ('STARTED','ACKED','UNSETTLED','NOT_SENT')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 acknowledged_at timestamptz
);
CREATE UNIQUE INDEX file_reference_inventory_unsettled_key
 ON file_reference_inventory_deletes(object_key) WHERE state IN ('STARTED','UNSETTLED');

COMMIT;
