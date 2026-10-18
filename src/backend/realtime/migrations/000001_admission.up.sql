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
 request_bytes bytea NOT NULL,
 response_bytes bytea,
 completed_at timestamptz,
 PRIMARY KEY(space_id,operation_id,generation,kind),
 CHECK ((response_bytes IS NULL) = (completed_at IS NULL))
);
