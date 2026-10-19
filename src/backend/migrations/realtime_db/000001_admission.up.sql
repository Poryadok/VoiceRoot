-- Metadata only. Pending admissions and terminal denies have no TTL.
CREATE TABLE realtime_incarnations (
 id uuid PRIMARY KEY,
 runtime_identity text NOT NULL CHECK (runtime_identity <> ''),
 state text NOT NULL CHECK (state IN ('OPEN','CLOSING','CLOSED')),
 opened_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 closed_at timestamptz
);
CREATE TABLE realtime_chat_gates (
 chat_id uuid PRIMARY KEY,
 space_id uuid,
 scope_known boolean NOT NULL DEFAULT false,
 generation bigint NOT NULL DEFAULT 0 CHECK (generation >= 0),
 state text NOT NULL DEFAULT 'UNKNOWN' CHECK (state IN ('UNKNOWN','LIVE','FROZEN','PURGE_DECIDED','PURGED')),
 operation_id uuid,
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE realtime_admissions (
 id uuid PRIMARY KEY,
 incarnation_id uuid NOT NULL REFERENCES realtime_incarnations(id),
 chat_id uuid NOT NULL REFERENCES realtime_chat_gates(chat_id),
 generation bigint NOT NULL CHECK (generation >= 0),
 state text NOT NULL CHECK (state IN ('PENDING','SETTLED')),
 admitted_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 settled_at timestamptz,
 CHECK ((state = 'SETTLED') = (settled_at IS NOT NULL))
);
CREATE INDEX realtime_pending_chat_idx ON realtime_admissions(chat_id) WHERE state='PENDING';
CREATE TABLE realtime_lifecycle_operations (
 space_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 generation bigint NOT NULL CHECK(generation > 0),
 kind text NOT NULL,
 request_bytes bytea,
 request_sha256 bytea NOT NULL CHECK(octet_length(request_sha256)=32),
 response_bytes bytea,
 completed_at timestamptz,
 PRIMARY KEY(space_id,operation_id,generation,kind),
 CHECK (response_bytes IS NULL OR completed_at IS NOT NULL),
 CHECK ((request_bytes IS NULL) = (response_bytes IS NULL))
);
CREATE TABLE realtime_manifest_heads (
 space_id uuid PRIMARY KEY,
 operation_id uuid NOT NULL,
 schedule_generation bigint NOT NULL CHECK(schedule_generation > 0),
 generation bigint NOT NULL CHECK(generation > 0),
 state text NOT NULL CHECK(state IN ('FROZEN','LIVE','PURGE_DECIDED','PURGED')),
 manifest_id uuid NOT NULL,
 manifest_sha256 bytea NOT NULL CHECK(octet_length(manifest_sha256)=32),
 item_count bigint NOT NULL CHECK(item_count >= 0),
 received_pages bigint NOT NULL DEFAULT 0 CHECK(received_pages >= 0),
 received_items bigint NOT NULL DEFAULT 0 CHECK(received_items >= 0),
 sealed boolean NOT NULL DEFAULT false,
 freeze_complete boolean NOT NULL DEFAULT false
);
CREATE TABLE realtime_manifest_items (
 space_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 chat_id uuid NOT NULL,
 PRIMARY KEY(space_id,operation_id,chat_id)
);
